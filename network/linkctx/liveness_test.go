package linkctx

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	linktypes "github.com/itohio/dndm/types/link"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// Liveness (verify DND-022 recheck, follow-up 1): a retired session must not lock out a
// live peer. With random session ids, arrival order cannot tell an old session from a
// new one, so a side may retire the peer's real session; once that side reconnects
// itself, every peer session must be acceptable again.

// settle starts both sides once more (a retry each), pumps, and checks that both are
// open and that a fresh route binds and delivers.
func (p *pair) settle(route string) {
	p.t.Helper()
	require.NoError(p.t, p.a.Start())
	require.NoError(p.t, p.b.Start())
	rounds, ok := p.pumpN(100)
	require.True(p.t, ok, "Hellos still flowing after %d rounds", rounds)
	require.True(p.t, p.a.Open(), "A not open")
	require.True(p.t, p.b.Open(), "B not open (stale drops: %d)", p.b.Stats().DroppedStale)
	p.bindAndDeliver(route, "live")
}

// Verify's minimal reproduction: A reconnects twice, the face delivers A's second
// Hello before the first, then B reconnects. B had retired A's real session.
func TestReorderedReconnectThenPeerReconnectStaysLive(t *testing.T) {
	p := newPair(t, onboard())
	p.open()

	p.a.Restart()
	require.NoError(t, p.a.Start()) // Hello A1
	p.a.Restart()
	require.NoError(t, p.a.Start()) // Hello A2
	slices.Reverse(p.toB)           // the face delivers A2 before A1
	_, ok := p.pumpN(100)
	require.True(t, ok)

	p.b.Restart()
	require.NoError(t, p.b.Start())
	_, ok = p.pumpN(100)
	require.True(t, ok)

	p.settle("route.after")
}

// One spoofed Hello on an unauthenticated face, then B reconnects: B must accept A again.
func TestSpoofThenPeerReconnectStaysLive(t *testing.T) {
	p := newPair(t, onboard())
	p.open()
	spoof, err := proto.Marshal(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Hello{Hello: &linktypes.Hello{
		Version: LinkVersion, Profile: linktypes.Profile_PROFILE_ONBOARD, MaxRoutes: 64, Session: 0xBAD5E55,
	}}})
	require.NoError(t, err)
	require.NoError(t, p.b.Receive(spoof))
	_, ok := p.pumpN(100)
	require.True(t, ok)

	p.b.Restart()
	require.NoError(t, p.b.Start())
	_, ok = p.pumpN(100)
	require.True(t, ok)

	p.settle("route.after")
}

// Random schedules (as in verify's probe): Start retries, reconnects, late duplicate and
// spoofed Hellos, and shuffled delivery. They end with one Start retry each, either
// directly or after one side (chosen at random) reconnects. After that, both must be open,
// a fresh route must deliver, and no Hello storm may occur at any point.
func TestRandomSchedulesStayLive(t *testing.T) {
	const schedules = 1000
	for _, reconnect := range []bool{false, true} {
		failed := 0
		var first string
		for seed := range uint64(schedules) {
			if msg := runSchedule(seed, reconnect); msg != "" {
				failed++
				if first == "" {
					first = fmt.Sprintf("seed %d: %s", seed, msg)
				}
			}
		}
		assert.Zero(t, failed, "final reconnect %v: %d of %d schedules lost liveness; first: %s", reconnect, failed, schedules, first)
	}
}

// runSchedule returns "" if the pair stays live, else what went wrong. With reconnect
// set, one side reconnects before the final retries.
func runSchedule(seed uint64, reconnect bool) string {
	rng := rand.New(rand.NewPCG(seed, 0x5eed))
	p := newPair(nil, onboard()) // this pump does not use p.t
	links := []*Link{p.a, p.b}
	queues := []*[][]byte{&p.toB, &p.toA} // frames to the other side
	var seen [][]byte                     // Hellos sent so far, for late duplicates
	pump := func(maxRounds int) bool {
		for range maxRounds {
			if len(p.toA) == 0 && len(p.toB) == 0 {
				return true
			}
			if rng.IntN(2) == 0 {
				rng.Shuffle(len(p.toA), func(i, j int) { p.toA[i], p.toA[j] = p.toA[j], p.toA[i] })
				rng.Shuffle(len(p.toB), func(i, j int) { p.toB[i], p.toB[j] = p.toB[j], p.toB[i] })
			}
			toA, toB := p.toA, p.toB
			p.toA, p.toB = nil, nil
			for _, f := range toB {
				_ = p.b.Receive(f)
			}
			for _, f := range toA {
				_ = p.a.Receive(f)
			}
		}
		return len(p.toA) == 0 && len(p.toB) == 0
	}
	_ = p.a.Start()
	_ = p.b.Start()
	pump(100)
	for range 12 {
		i := rng.IntN(2)
		switch rng.IntN(5) {
		case 0: // retry
			_ = links[i].Start()
		case 1: // reconnect
			links[i].Restart()
			_ = links[i].Start()
		case 2: // late duplicate of an earlier Hello
			if len(seen) > 0 {
				*queues[i] = append(*queues[i], seen[rng.IntN(len(seen))])
			}
		case 3: // spoofed Hello
			f, _ := proto.Marshal(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Hello{Hello: &linktypes.Hello{
				Version: LinkVersion, Profile: linktypes.Profile_PROFILE_ONBOARD, MaxRoutes: 64, Session: rng.Uint64() | 1,
			}}})
			*queues[i] = append(*queues[i], f)
		case 4: // deliver some frames
			pump(1 + rng.IntN(2))
		}
		seen = append(seen[:0:0], append(p.fa.hellos, p.fb.hellos...)...)
	}
	if !pump(200) {
		return "Hello storm during the schedule"
	}
	if reconnect {
		l := links[rng.IntN(2)]
		l.Restart()
		_ = l.Start()
		if !pump(200) {
			return "Hello storm after the reconnect"
		}
	}
	_ = p.a.Start()
	_ = p.b.Start()
	if !pump(200) {
		return "Hello storm after the retries"
	}
	if !p.a.Open() || !p.b.Open() {
		return fmt.Sprintf("open A=%v B=%v (stale drops A=%d B=%d)", p.a.Open(), p.b.Open(), p.a.Stats().DroppedStale, p.b.Stats().DroppedStale)
	}
	if _, err := p.a.Bind("route.live", "types.T"); err != nil {
		return "bind: " + err.Error()
	}
	pump(50)
	if err := p.a.Send("route.live", "types.T", []byte("x")); err != nil {
		return "send: " + err.Error()
	}
	pump(50)
	if !slices.Contains(p.gotB, delivered{"route.live", "types.T", "x"}) {
		return "DATA not delivered"
	}
	return ""
}
