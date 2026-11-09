package codec

import (
	"bytes"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/itohio/dndm/errors"
	types "github.com/itohio/dndm/types/core"
	linktypes "github.com/itohio/dndm/types/link"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// Byte-count tests for link-context DATA frames (DND-022, link-context-wire.md §3 and §8).

// fooOfSize returns a message whose encoding is exactly n bytes (2 <= n <= 129).
func fooOfSize(t *testing.T, n int) *testtypes.Foo {
	t.Helper()
	m := &testtypes.Foo{Text: strings.Repeat("x", n-2)}
	require.Equal(t, n, proto.Size(m))
	return m
}

func TestLinkFrameOverhead(t *testing.T) {
	cases := []struct {
		payload  int
		overhead int // UDP / COBS serial: the face delimits the frame
		prefix   int // TCP: varint length prefix on top
	}{
		{26, 4, 1},    // EasyRobot ManipulatorTarget
		{62, 4, 1},    // EasyRobot ManipulatorState
		{127, 4, 2},   // frame 131 B: the length prefix needs 2 B
		{128, 5, 2},   // payload length needs 2 B
		{16383, 5, 3}, // frame 16 388 B: the length prefix needs 3 B
		{16384, 6, 3}, // payload length needs 3 B
	}
	for _, c := range cases {
		p := bytes.Repeat([]byte{0xA5}, c.payload)
		f, err := AppendLinkData(nil, LinkData{RouteID: 1, Payload: p})
		require.NoError(t, err)
		assert.Equal(t, c.overhead, len(f)-c.payload, "payload %d B: DATA overhead", c.payload)
		s := AppendLinkLength(nil, len(f))
		assert.Equal(t, c.prefix, len(s), "payload %d B: TCP length prefix", c.payload)
	}

	// The same EasyRobot routes as v1 frames (preamble + Header with the route as text),
	// so the saving is measured here, not quoted (design §1: 86 and 94 B).
	routes := []struct {
		route    string
		payload  int
		overhead int
	}{
		{"types.control.ManipulatorState@easyrobot.manipulator.raw", 62, 86},
		{"types.control.ManipulatorTarget@easyrobot.manipulator.raw.target", 26, 94},
	}
	for _, r := range routes {
		msg := fooOfSize(t, r.payload)
		v1, err := AppendMessageTo(nil, &types.Header{
			Timestamp: uint64(time.Now().UnixNano()),
			Type:      types.Type_MESSAGE,
			Route:     r.route,
		}, msg)
		require.NoError(t, err)
		v1Len := len(v1)
		Release(v1)
		assert.Equal(t, r.overhead, v1Len-r.payload, "%s: v1 overhead", r.route)

		v2, err := AppendLinkMessage(nil, 1, 0, msg)
		require.NoError(t, err)
		assert.Equal(t, 4, len(v2)-r.payload, "%s: link-context overhead", r.route)
		t.Logf("%s: v1 %d B, link-context %d B (%d B less)", r.route, v1Len, len(v2), v1Len-len(v2))
	}
}

func TestLinkFrameRouteIdBoundary(t *testing.T) {
	p := bytes.Repeat([]byte{0x5A}, 62)
	for _, c := range []struct {
		id       uint32
		overhead int
	}{{1, 4}, {127, 4}, {128, 5}, {16383, 5}, {16384, 6}} {
		f, err := AppendLinkData(nil, LinkData{RouteID: c.id, Payload: p})
		require.NoError(t, err)
		assert.Equal(t, c.overhead, len(f)-len(p), "route_id %d", c.id)
	}
}

// The hand-written protowire path must be the generated schema's wire, in both
// directions; otherwise it becomes a second contract.
func TestLinkFrameRoundTrip(t *testing.T) {
	p := bytes.Repeat([]byte{0x11}, 62)
	sealed := bytes.Repeat([]byte{0x22}, 62+8)
	for name, d := range map[string]LinkData{
		"data":         {RouteID: 1, Payload: p},
		"data+seq":     {RouteID: 5, Seq: 200, Payload: p},
		"sealed":       {RouteID: 300, Ctr: 200, Sealed: sealed},
		"sealed ctr 0": {RouteID: 3, Sealed: sealed},
		"fragment":     {RouteID: 2, Seq: 1, Frag: 3<<1 | 1, Payload: p},
		"empty":        {RouteID: 9},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := AppendLinkData(nil, d)
			require.NoError(t, err)

			got, err := DecodeLinkData(f)
			require.NoError(t, err)
			assertLinkData(t, d, got)

			var lf linktypes.LinkFrame
			require.NoError(t, proto.Unmarshal(f, &lf))
			assertLinkData(t, d, LinkData{RouteID: lf.RouteId, Seq: lf.Seq, Frag: lf.Frag, Ctr: lf.Ctr, Payload: lf.Payload, Sealed: lf.Sealed})

			gen, err := proto.MarshalOptions{Deterministic: true}.Marshal(&lf)
			require.NoError(t, err)
			assert.Equal(t, gen, f, "generated code and the protowire path must produce the same bytes")
		})
	}

	t.Run("message", func(t *testing.T) {
		msg := fooOfSize(t, 62)
		f, err := AppendLinkMessage(nil, 7, 3, msg)
		require.NoError(t, err)
		want, err := proto.Marshal(msg)
		require.NoError(t, err)
		got, err := DecodeLinkData(f)
		require.NoError(t, err)
		assertLinkData(t, LinkData{RouteID: 7, Seq: 3, Payload: want}, got)
	})

	controls := map[string]*linktypes.LinkFrame{
		"hello":      {Control: &linktypes.LinkFrame_Hello{Hello: &linktypes.Hello{Version: 2, Profile: linktypes.Profile_PROFILE_ZONE_PSK, Nonce: []byte{1, 2, 3}, MaxFrame: 1500, MaxRoutes: 64, Session: 1 << 40}}},
		"handshake":  {Control: &linktypes.LinkFrame_Handshake{Handshake: &linktypes.Handshake{Step: 1, Data: bytes.Repeat([]byte{7}, 32)}}},
		"announce":   {Control: &linktypes.LinkFrame_Announce{Announce: &linktypes.Announce{Role: linktypes.Role_ROLE_INTEREST, Path: "easyrobot.manipulator.raw", Type: "types.control.ManipulatorState", Register: true}}},
		"bind":       {Control: &linktypes.LinkFrame_Bind{Bind: &linktypes.Bind{RouteId: 1, Path: "easyrobot.manipulator.raw", Type: "types.control.ManipulatorState", Stages: 1, Policy: []byte{4}}}},
		"bind_ack":   {Control: &linktypes.LinkFrame_BindAck{BindAck: &linktypes.BindAck{RouteId: 1, Status: linktypes.BindStatus_BIND_STATUS_TABLE_FULL}}},
		"unbind":     {Control: &linktypes.LinkFrame_Unbind{Unbind: &linktypes.Unbind{RouteId: 1}}},
		"unbind_ack": {Control: &linktypes.LinkFrame_UnbindAck{UnbindAck: &linktypes.UnbindAck{RouteId: 1}}},
		"close":      {Control: &linktypes.LinkFrame_Close{Close: &linktypes.Close{Reason: linktypes.CloseReason_CLOSE_REASON_DUPLICATE, Text: "dup", Session: 9}}},
	}
	for name, lf := range controls {
		t.Run("control "+name, func(t *testing.T) {
			f, err := proto.Marshal(lf)
			require.NoError(t, err)
			d, err := DecodeLinkData(f)
			require.NoError(t, err)
			assert.Zero(t, d.RouteID, "a control frame decodes as route 0 (decode it with proto.Unmarshal)")

			var back linktypes.LinkFrame
			require.NoError(t, proto.Unmarshal(f, &back))
			assert.True(t, proto.Equal(lf, &back))
		})
	}

	t.Run("data and control in one frame", func(t *testing.T) {
		f, err := proto.Marshal(&linktypes.LinkFrame{RouteId: 1, Payload: p, Control: &linktypes.LinkFrame_Bind{Bind: &linktypes.Bind{RouteId: 1}}})
		require.NoError(t, err)
		_, err = DecodeLinkData(f)
		assert.True(t, stderrors.Is(err, errors.ErrBadFrame), "got %v", err)
	})
	// Review R-DND-R9 L2: the oneof allows one control message in the struct, but the wire
	// can still carry two (generated decoding keeps the last). That is a protocol error.
	t.Run("two control messages in one frame", func(t *testing.T) {
		hello, err := proto.Marshal(controls["hello"])
		require.NoError(t, err)
		closing, err := proto.Marshal(controls["close"])
		require.NoError(t, err)
		_, err = DecodeLinkData(append(hello, closing...))
		assert.True(t, stderrors.Is(err, errors.ErrBadFrame), "got %v", err)
	})
	// Review R-DND-R9 L1: the encoder refuses payload with sealed, so the decoder must too;
	// otherwise unauthenticated bytes could ride next to a valid sealed blob.
	t.Run("payload and sealed in one frame", func(t *testing.T) {
		f, err := proto.Marshal(&linktypes.LinkFrame{RouteId: 5, Payload: []byte("plain"), Sealed: []byte("sealed")})
		require.NoError(t, err)
		_, err = DecodeLinkData(f)
		assert.True(t, stderrors.Is(err, errors.ErrBadFrame), "got %v", err)
	})
	t.Run("truncated", func(t *testing.T) {
		f, err := AppendLinkData(nil, LinkData{RouteID: 1, Payload: p})
		require.NoError(t, err)
		_, err = DecodeLinkData(f[:len(f)-1])
		assert.True(t, stderrors.Is(err, errors.ErrBadFrame), "got %v", err)
	})
	t.Run("encoder refuses", func(t *testing.T) {
		_, err := AppendLinkData(nil, LinkData{Payload: p})
		assert.True(t, stderrors.Is(err, errors.ErrBadArgument), "route 0 is the control space: %v", err)
		_, err = AppendLinkData(nil, LinkData{RouteID: 1, Payload: p, Sealed: sealed})
		assert.True(t, stderrors.Is(err, errors.ErrBadArgument), "payload and sealed together: %v", err)
	})
}

func TestLinkFrameNoCopy(t *testing.T) {
	p := bytes.Repeat([]byte{0x33}, 200)
	f, err := AppendLinkData(nil, LinkData{RouteID: 4, Payload: p})
	require.NoError(t, err)
	d, err := DecodeLinkData(f)
	require.NoError(t, err)
	require.Len(t, d.Payload, len(p))
	f[len(f)-1] ^= 0xFF // payload is the frame's last field
	assert.Equal(t, byte(0x33^0xFF), d.Payload[len(d.Payload)-1], "the decoded payload must alias the frame buffer")
}

func assertLinkData(t *testing.T, want, got LinkData) {
	t.Helper()
	assert.Equal(t, want.RouteID, got.RouteID, "route_id")
	assert.Equal(t, want.Seq, got.Seq, "seq")
	assert.Equal(t, want.Frag, got.Frag, "frag")
	assert.Equal(t, want.Ctr, got.Ctr, "ctr")
	assert.True(t, bytes.Equal(want.Payload, got.Payload), "payload")
	assert.True(t, bytes.Equal(want.Sealed, got.Sealed), "sealed")
}
