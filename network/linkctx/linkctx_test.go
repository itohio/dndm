package linkctx

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/itohio/dndm/codec"
	linktypes "github.com/itohio/dndm/types/link"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// Tests for the link-context core (DND-022, docs/design/link-context-wire.md §4 and §8).

// fakeFace delivers frames to a peer queue. hold, when set, keeps DATA frames back so a
// test can deliver them late (a reordering face such as UDP).
type fakeFace struct {
	queue *[][]byte
	sizes []int // sizes of DATA frames sent through this face
	hold  bool
	held  [][]byte
}

func (f *fakeFace) Send(frame []byte) error {
	b := append([]byte(nil), frame...)
	if d, err := codec.DecodeLinkData(b); err == nil && d.RouteID != 0 {
		f.sizes = append(f.sizes, len(b))
		if f.hold {
			f.held = append(f.held, b)
			return nil
		}
	}
	*f.queue = append(*f.queue, b)
	return nil
}

type delivered struct {
	path, typ string
	payload   string
}

type pair struct {
	t          *testing.T
	a, b       *Link
	fa, fb     *fakeFace
	toA, toB   [][]byte
	gotA, gotB []delivered
	now        time.Time
}

func newPair(t *testing.T, cfg Config) *pair {
	p := &pair{t: t, now: time.Unix(1_000_000, 0)}
	cfg.Now = func() time.Time { return p.now }
	p.fa = &fakeFace{queue: &p.toB}
	p.fb = &fakeFace{queue: &p.toA}
	p.a = New(p.fa, cfg, func(path, typ string, payload []byte) {
		p.gotA = append(p.gotA, delivered{path, typ, string(payload)})
	})
	p.b = New(p.fb, cfg, func(path, typ string, payload []byte) {
		p.gotB = append(p.gotB, delivered{path, typ, string(payload)})
	})
	return p
}

// pump delivers queued frames in both directions until both queues are empty.
func (p *pair) pump() {
	p.t.Helper()
	for len(p.toA) > 0 || len(p.toB) > 0 {
		toA, toB := p.toA, p.toB
		p.toA, p.toB = nil, nil
		for _, f := range toB {
			require.NoError(p.t, p.b.Receive(f))
		}
		for _, f := range toA {
			require.NoError(p.t, p.a.Receive(f))
		}
	}
}

func (p *pair) open() {
	p.t.Helper()
	require.NoError(p.t, p.a.Start())
	require.NoError(p.t, p.b.Start())
	p.pump()
	require.True(p.t, p.a.Open() && p.b.Open(), "both sides must be open after the Hello exchange")
}

func routeName(i int) (string, string) {
	return fmt.Sprintf("easyrobot.route.%d", i), "types.control.ManipulatorState"
}

// Verify DND-R7 F1: with ids carried across sessions they pass 127 after about 13
// reconnects of 10 routes and every DATA frame grows to 5 B of overhead. Ids belong to a
// session and are allocated lowest-free, so they stay at 1..10.
func TestRouteIdsStaySmallAcrossReconnects(t *testing.T) {
	p := newPair(t, Config{Profile: linktypes.Profile_PROFILE_ONBOARD, MaxRoutes: 64})
	payload := bytes.Repeat([]byte{0x42}, 62)
	maxID := uint32(0)
	for session := range 50 {
		p.open()
		for i := 1; i <= 10; i++ {
			path, typ := routeName(i)
			id, err := p.a.Bind(path, typ)
			require.NoError(t, err)
			maxID = max(maxID, id)
		}
		p.pump()
		// Unbind and rebind a few routes in between.
		for i := 1; i <= 3; i++ {
			path, typ := routeName(i)
			require.NoError(t, p.a.Unbind(path, typ))
		}
		p.pump()
		for i := 1; i <= 3; i++ {
			path, typ := routeName(i)
			id, err := p.a.Bind(path, typ)
			require.NoError(t, err)
			maxID = max(maxID, id)
		}
		p.pump()
		for i := 1; i <= 10; i++ {
			path, typ := routeName(i)
			require.NoError(t, p.a.Send(path, typ, payload))
		}
		p.pump()
		require.Len(t, p.gotB, 10, "session %d: every route delivers", session)
		p.gotB = nil
		p.a.Restart() // a reconnect: a new session on both sides
		p.b.Restart()
	}
	assert.LessOrEqual(t, maxID, uint32(10), "route ids must stay within the number of bound routes")
	for i, n := range p.fa.sizes {
		require.Equal(t, 4, n-len(payload), "DATA frame %d: overhead", i)
	}
}

// A freed id is not reused before UnbindAck plus the quarantine, and a late frame for the
// old route is dropped, not delivered on the new one (design §4, verify A3).
func TestRouteIdReuseQuarantine(t *testing.T) {
	const q = 2 * time.Second
	p := newPair(t, Config{Profile: linktypes.Profile_PROFILE_ONBOARD, MaxRoutes: 64, Quarantine: q})
	p.open()

	idX, err := p.a.Bind("route.x", "types.X")
	require.NoError(t, err)
	p.pump()
	p.fa.hold = true // the next DATA frame is delayed by the face
	require.NoError(t, p.a.Send("route.x", "types.X", []byte("old")))
	p.fa.hold = false

	require.NoError(t, p.a.Unbind("route.x", "types.X"))
	p.pump() // Unbind and UnbindAck
	idY, err := p.a.Bind("route.y", "types.Y")
	require.NoError(t, err)
	assert.NotEqual(t, idX, idY, "an id in quarantine must not be reused")
	p.pump()

	for _, f := range p.fa.held { // the late frame for the old route arrives now
		require.NoError(t, p.b.Receive(f))
	}
	assert.Empty(t, p.gotB, "a late frame for an unbound id must not be delivered")
	assert.Equal(t, uint64(1), p.b.Stats().DroppedUnbound)

	p.now = p.now.Add(q + time.Millisecond) // the quarantine has passed
	idZ, err := p.a.Bind("route.z", "types.Z")
	require.NoError(t, err)
	assert.Equal(t, idX, idZ, "after the quarantine the lowest free id is reused")
	p.pump()
	require.NoError(t, p.a.Send("route.z", "types.Z", []byte("new")))
	p.pump()
	assert.Equal(t, []delivered{{"route.z", "types.Z", "new"}}, p.gotB)
}

// Counter low bits (design §4, verify DND-R7 F2), on the classic CAN profile (7 bits).
func TestCounterLowBits(t *testing.T) {
	const bits = 7
	mac := []byte{0xA1, 0xA2, 0xA3} // 3 B MAC
	data := []byte{0x01}            // 1 B of command data
	for _, ctr := range []uint64{127, 128, 255, 256} {
		low := LowBits(ctr, bits)
		assert.Less(t, low, uint32(128), "ctr %d", ctr)
		// On CAN the route id travels in the CAN identifier, so the frame is the LinkFrame
		// without route_id: subtract route_id's 2 B (id 1) from the encoded size.
		size := codec.LinkDataSize(codec.LinkData{RouteID: 1, Ctr: low, Sealed: append(append([]byte(nil), data...), mac...)}) - 2
		assert.LessOrEqual(t, size, 8, "ctr %d: a classic CAN command frame fits 8 B with 1 B of data", ctr)
	}

	// The receiver rebuilds the full counter from the highest accepted one, across the
	// 127 -> 128 wrap, ahead (lost frames) and behind (late frames within the window).
	for _, c := range []struct{ highest, sent uint64 }{
		{127, 128}, {120, 130}, {200, 263}, {200, 137}, {255, 256}, {256, 255}, {1, 2},
	} {
		assert.Equal(t, c.sent, RebuildCounter(c.highest, LowBits(c.sent, bits), bits), "highest %d, sent %d", c.highest, c.sent)
	}
	// A gap of exactly 64 is a tie at 7 bits (264 and 136 share their low bits); ties
	// resolve ahead. Beyond that the rebuilt counter is wrong, so the tag check fails and
	// the link resyncs with a new handshake (tag check: DND-009).
	// Both code paths of the tie: the ahead candidate in the next window (200 -> 264) and
	// in the same window (130 -> 194, not 66; verify DND-022's mutant T).
	assert.Equal(t, uint64(264), RebuildCounter(200, LowBits(264, bits), bits), "a tie resolves ahead (next window)")
	assert.Equal(t, uint64(194), RebuildCounter(130, LowBits(194, bits), bits), "a tie resolves ahead (same window)")
	assert.NotEqual(t, uint64(265), RebuildCounter(200, LowBits(265, bits), bits), "65 frames lost: ambiguous")
	assert.NotEqual(t, uint64(135), RebuildCounter(200, LowBits(135, bits), bits), "65 frames late: ambiguous")
	// 14 bits: unambiguous up to 8191 either way.
	assert.Equal(t, uint64(10_000+8191), RebuildCounter(10_000, LowBits(10_000+8191, 14), 14))
	assert.Equal(t, uint64(10_000-8191), RebuildCounter(10_000, LowBits(10_000-8191, 14), 14))
}

// A1: every frame except Hello/Handshake is dropped, and counted, until the handshake has
// completed.
func TestLinkDropsBeforeHandshake(t *testing.T) {
	p := newPair(t, Config{Profile: linktypes.Profile_PROFILE_ONBOARD, MaxRoutes: 64})
	data, err := codec.AppendLinkData(nil, codec.LinkData{RouteID: 1, Payload: []byte("early")})
	require.NoError(t, err)
	bind, err := proto.Marshal(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Bind{Bind: &linktypes.Bind{RouteId: 1, Path: "p", Type: "t"}}})
	require.NoError(t, err)

	require.NoError(t, p.b.Receive(data))
	require.NoError(t, p.b.Receive(bind))
	assert.Empty(t, p.gotB)
	assert.Empty(t, p.toA, "no BindAck before the handshake")
	assert.Equal(t, uint64(2), p.b.Stats().DroppedBeforeOpen)

	_, err = p.a.Bind("p", "t")
	assert.ErrorIs(t, err, ErrNotOpen, "a link that is not open cannot bind")
}

// A3: a Bind that would retarget a live id is refused; the same Bind again is idempotent.
func TestBindRetargetRefused(t *testing.T) {
	p := newPair(t, Config{Profile: linktypes.Profile_PROFILE_ONBOARD, MaxRoutes: 64})
	p.open()
	_, err := p.a.Bind("route.x", "types.X")
	require.NoError(t, err)
	p.pump()

	acks := func(b *linktypes.Bind) linktypes.BindStatus {
		t.Helper()
		f, err := proto.Marshal(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Bind{Bind: b}})
		require.NoError(t, err)
		require.NoError(t, p.b.Receive(f))
		require.Len(t, p.toA, 1)
		var ack linktypes.LinkFrame
		require.NoError(t, proto.Unmarshal(p.toA[0], &ack))
		p.toA = nil
		return ack.GetBindAck().GetStatus()
	}
	assert.Equal(t, linktypes.BindStatus_BIND_STATUS_OK, acks(&linktypes.Bind{RouteId: 1, Path: "route.x", Type: "types.X"}), "a duplicate Bind is acknowledged again")
	assert.Equal(t, linktypes.BindStatus_BIND_STATUS_DENIED, acks(&linktypes.Bind{RouteId: 1, Path: "route.evil", Type: "types.X"}), "a live id must not be retargeted")

	require.NoError(t, p.a.Send("route.x", "types.X", []byte("still x")))
	p.pump()
	assert.Equal(t, []delivered{{"route.x", "types.X", "still x"}}, p.gotB)
}

// A8: the receiver's route table is capped (Hello.max_routes); further Binds get
// BIND_STATUS_TABLE_FULL, and the sender frees the refused id.
func TestBindTableFull(t *testing.T) {
	p := newPair(t, Config{Profile: linktypes.Profile_PROFILE_ONBOARD, MaxRoutes: 4})
	p.open()
	for i := 1; i <= 6; i++ {
		path, typ := routeName(i)
		_, err := p.a.Bind(path, typ)
		if i <= 4 {
			require.NoError(t, err)
		} else {
			assert.ErrorIs(t, err, ErrTableFull, "the sender's own table is capped too")
		}
	}
	p.pump()
	assert.Equal(t, 4, p.b.Routes(), "the receiver holds at most max_routes")

	// A peer that ignores its cap: the receiver answers TABLE_FULL.
	f, err := proto.Marshal(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Bind{Bind: &linktypes.Bind{RouteId: 9, Path: "flood", Type: "t"}}})
	require.NoError(t, err)
	require.NoError(t, p.b.Receive(f))
	require.Len(t, p.toA, 1)
	var ack linktypes.LinkFrame
	require.NoError(t, proto.Unmarshal(p.toA[0], &ack))
	assert.Equal(t, linktypes.BindStatus_BIND_STATUS_TABLE_FULL, ack.GetBindAck().GetStatus())
	assert.Equal(t, uint64(1), p.b.Stats().BindsRefused)
}
