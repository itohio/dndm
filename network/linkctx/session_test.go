package linkctx

import (
	"testing"

	linktypes "github.com/itohio/dndm/types/link"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// Session rules (review R-DND-022 L1-L3, L6).

// pumpN delivers queued frames for at most maxRounds rounds and reports whether both
// queues drained (a pump that never drains is a Hello storm).
func (p *pair) pumpN(maxRounds int) (rounds int, settled bool) {
	p.t.Helper()
	for rounds = 0; rounds < maxRounds; rounds++ {
		if len(p.toA) == 0 && len(p.toB) == 0 {
			return rounds, true
		}
		toA, toB := p.toA, p.toB
		p.toA, p.toB = nil, nil
		for _, f := range toB {
			require.NoError(p.t, p.b.Receive(f))
		}
		for _, f := range toA {
			require.NoError(p.t, p.a.Receive(f))
		}
	}
	return rounds, len(p.toA) == 0 && len(p.toB) == 0
}

func (p *pair) hellos() int { return len(p.fa.hellos) + len(p.fb.hellos) }

// bindAndDeliver binds a route on A and checks that DATA reaches B.
func (p *pair) bindAndDeliver(path, payload string) {
	p.t.Helper()
	_, err := p.a.Bind(path, "types.T")
	require.NoError(p.t, err)
	_, settled := p.pumpN(20)
	require.True(p.t, settled)
	require.NoError(p.t, p.a.Send(path, "types.T", []byte(payload)))
	_, settled = p.pumpN(20)
	require.True(p.t, settled)
	require.Contains(p.t, p.gotB, delivered{path, "types.T", payload})
}

func onboard() Config {
	return Config{Profile: linktypes.Profile_PROFILE_ONBOARD, MaxRoutes: 64}
}

// L1 (a): Start on an open link (a Hello retry) must not start a Hello storm, and must
// not reset either side's routes.
func TestHelloRetryOnOpenLinkSettles(t *testing.T) {
	p := newPair(t, onboard())
	p.open()
	p.bindAndDeliver("route.a", "before")

	before := p.hellos()
	require.NoError(t, p.a.Start())
	rounds, settled := p.pumpN(100)
	assert.True(t, settled, "Hellos still flowing after %d rounds", rounds)
	assert.LessOrEqual(t, p.hellos()-before, 2, "a Hello retry costs at most one Hello each way")
	require.True(t, p.a.Open() && p.b.Open())

	require.NoError(t, p.a.Send("route.a", "types.T", []byte("after")), "the retry must not drop A's bindings")
	p.pumpN(20)
	assert.Contains(t, p.gotB, delivered{"route.a", "types.T", "after"})
}

// L1 (b): after a one-sided reconnect has settled, a late duplicate of the peer's
// previous-session Hello (a reordering face) must be ignored: no storm, no reset.
func TestLateHelloFromRetiredSessionIgnored(t *testing.T) {
	p := newPair(t, onboard())
	p.open()
	oldHello := p.fa.hellos[0] // A's first-session Hello

	p.a.Restart() // A reconnects on its own; B is still open
	require.NoError(t, p.a.Start())
	rounds, settled := p.pumpN(100)
	require.True(t, settled, "the one-sided reconnect itself must settle (%d rounds)", rounds)
	require.True(t, p.a.Open() && p.b.Open())
	p.bindAndDeliver("route.new", "fresh")

	before := p.hellos()
	require.NoError(t, p.b.Receive(oldHello)) // the late duplicate arrives
	rounds, settled = p.pumpN(100)
	assert.True(t, settled, "Hellos still flowing after %d rounds", rounds)
	assert.Equal(t, 0, p.hellos()-before, "a retired session's Hello gets no answer")
	assert.Equal(t, uint64(1), p.b.Stats().DroppedStale)

	require.NoError(t, p.a.Send("route.new", "types.T", []byte("still bound")), "B must not have reset A's routes")
	p.pumpN(20)
	assert.Contains(t, p.gotB, delivered{"route.new", "types.T", "still bound"})
}

// L1 (c): one spoofed Hello with an unknown session (possible on unsecured profiles) is
// bounded: B takes it for a reconnect and answers once, A answers that once, and B drops
// A's answer because A's session is now retired. It cannot set two honest nodes
// ping-ponging. (It still resets B's tables: nothing is authenticated on ONBOARD; secured
// profiles need DND-009.)
func TestSpoofedHelloBounded(t *testing.T) {
	p := newPair(t, onboard())
	p.open()
	spoof, err := proto.Marshal(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Hello{Hello: &linktypes.Hello{
		Version: LinkVersion, Profile: linktypes.Profile_PROFILE_ONBOARD, MaxRoutes: 64, Session: 0xBAD5E55,
	}}})
	require.NoError(t, err)
	before := p.hellos()
	require.NoError(t, p.b.Receive(spoof))
	rounds, settled := p.pumpN(100)
	assert.True(t, settled, "Hellos still flowing after %d rounds", rounds)
	assert.LessOrEqual(t, p.hellos()-before, 2, "one answer each way at most")
	assert.Equal(t, uint64(1), p.b.Stats().DroppedStale, "B drops A's answer (A's session is retired)")
	assert.True(t, p.a.Open() && p.b.Open())
}

// L2: a Close must name a current session (the sender's own, or before it has one, the
// receiver's). A Close for any other session is dropped and counted.
func TestStaleCloseIgnored(t *testing.T) {
	p := newPair(t, onboard())
	p.open()
	stale, err := proto.Marshal(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Close{Close: &linktypes.Close{
		Reason: linktypes.CloseReason_CLOSE_REASON_NORMAL, Session: 12345,
	}}})
	require.NoError(t, err)
	require.NoError(t, p.b.Receive(stale))
	assert.True(t, p.b.Open(), "a Close for an unknown session must not close the link")
	assert.Equal(t, uint64(1), p.b.Stats().DroppedStale)

	require.NoError(t, p.a.Close(linktypes.CloseReason_CLOSE_REASON_NORMAL, "bye"))
	p.pumpN(20)
	assert.False(t, p.b.Open(), "a Close for the current session closes the link")
	assert.ErrorIs(t, p.b.Start(), ErrClosed)
}

// L3: a peer Hello without max_routes (proto3 zero: an older or minimal peer) means "not
// specified", not "no routes".
func TestHelloWithoutMaxRoutesUsesDefault(t *testing.T) {
	p := newPair(t, onboard())
	require.NoError(t, p.a.Start())
	hello, err := proto.Marshal(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Hello{Hello: &linktypes.Hello{
		Version: LinkVersion, Profile: linktypes.Profile_PROFILE_ONBOARD, Session: 77,
	}}})
	require.NoError(t, err)
	require.NoError(t, p.a.Receive(hello))
	require.True(t, p.a.Open())
	_, err = p.a.Bind("route.x", "types.T")
	assert.NoError(t, err, "max_routes 0 must not leave a link that cannot bind")
}

// L6: a Hello with another profile closes the link with PROFILE_MISMATCH.
func TestProfileMismatchCloses(t *testing.T) {
	p := newPair(t, onboard())
	bad, err := proto.Marshal(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Hello{Hello: &linktypes.Hello{
		Version: LinkVersion, Profile: linktypes.Profile_PROFILE_ZONE_PSK, MaxRoutes: 8, Session: 5,
	}}})
	require.NoError(t, err)
	require.NoError(t, p.b.Receive(bad))
	require.Len(t, p.toA, 1)
	var f linktypes.LinkFrame
	require.NoError(t, proto.Unmarshal(p.toA[0], &f))
	assert.Equal(t, linktypes.CloseReason_CLOSE_REASON_PROFILE_MISMATCH, f.GetClose().GetReason())
	assert.ErrorIs(t, p.b.Start(), ErrClosed)
}

// L6: Unbind before the BindAck arrives: the late ack must not bind the route, and the id
// is reusable once UnbindAck has come back.
func TestUnbindBeforeBindAck(t *testing.T) {
	p := newPair(t, onboard())
	p.open()
	id, err := p.a.Bind("route.x", "types.T")
	require.NoError(t, err)
	require.NoError(t, p.a.Unbind("route.x", "types.T")) // Bind and Unbind are both still queued
	p.pumpN(20)
	assert.ErrorIs(t, p.a.Send("route.x", "types.T", []byte("x")), ErrNotBound)
	assert.Zero(t, p.b.Routes(), "B unbound it again")
	id2, err := p.a.Bind("route.y", "types.T")
	require.NoError(t, err)
	assert.Equal(t, id, id2, "after UnbindAck (no quarantine on this face) the id is free again")
}

// The answer rule must keep a lossy face live: if B's Hello is lost, A's retry (Start
// again) gets B's Hello and A opens.
func TestHelloRetryAfterLostHello(t *testing.T) {
	p := newPair(t, onboard())
	require.NoError(t, p.a.Start())
	require.NoError(t, p.b.Start())
	p.toA = nil // B's Hello is lost
	_, settled := p.pumpN(20)
	require.True(t, settled)
	require.True(t, p.b.Open())
	require.False(t, p.a.Open())

	require.NoError(t, p.a.Start()) // retry
	rounds, settled := p.pumpN(20)
	assert.True(t, settled, "Hellos still flowing after %d rounds", rounds)
	assert.True(t, p.a.Open(), "the retry gets B's Hello")
	p.bindAndDeliver("route.r", "ok")
}
