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

// Review R-DND-022-followups D1: A reconnects three times, B reconnects, the pair settles
// and binds a route, then the face delivers late duplicates of A's old Hellos. Clearing
// the retired list on Restart let B take an old session for a reconnect and drop the
// route, while A still believed it bound: Sends succeeded and were silently lost.
func TestLateDuplicatesAfterReconnectKeepRoutes(t *testing.T) {
	p := newPair(t, onboard())
	p.open()
	for range 3 {
		p.a.Restart()
		require.NoError(t, p.a.Start())
		_, ok := p.pumpN(100)
		require.True(t, ok)
	}
	old := slices.Clone(p.fa.hellos[:len(p.fa.hellos)-1]) // A's earlier sessions

	p.b.Restart()
	require.NoError(t, p.b.Start())
	_, ok := p.pumpN(100)
	require.True(t, ok)
	require.True(t, p.a.Open() && p.b.Open())
	p.bindAndDeliver("route.live", "first")

	hellos := p.hellos()
	for _, f := range old { // the late duplicates
		require.NoError(t, p.b.Receive(f))
	}
	_, ok = p.pumpN(100)
	require.True(t, ok)
	assert.Equal(t, 1, p.b.Routes(), "B must keep A's route")
	assert.Equal(t, hellos, p.hellos(), "late duplicates get no answer")
	require.NoError(t, p.a.Send("route.live", "types.T", []byte("second")))
	p.pumpN(20)
	assert.Contains(t, p.gotB, delivered{"route.live", "types.T", "second"}, "B dropped it (unbound drops %d)", p.b.Stats().DroppedUnbound)
}

// Random schedules (as in verify's probe): Start retries, reconnects, late duplicate and
// spoofed Hellos, and shuffled delivery. They end with one Start retry each, either
// directly or after one side (chosen at random) reconnects. After that, both must be open
// and a fresh route must deliver; then late duplicates of every earlier Hello arrive, and
// the bound route must still deliver. No Hello storm may occur at any point.
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

	// Review R-DND-022-followups D1: now every Hello of an earlier session arrives once
	// more, late (a reordering face), at the side it was sent to. None may disturb the
	// settled pair: the bound route must still deliver. (A repeat of the current session is
	// a retry and gets one answer; it is not injected.)
	hellosBefore := len(p.fa.hellos) + len(p.fb.hellos)
	for _, f := range p.fa.hellos {
		if helloSession(f) != p.a.session {
			p.toB = append(p.toB, f)
		}
	}
	for _, f := range p.fb.hellos {
		if helloSession(f) != p.b.session {
			p.toA = append(p.toA, f)
		}
	}
	injected := len(p.toA) + len(p.toB)
	if !pump(200) {
		return "Hello storm after the late duplicates"
	}
	if err := p.a.Send("route.live", "types.T", []byte("y")); err != nil {
		return "send after late duplicates: " + err.Error()
	}
	pump(50)
	if !slices.Contains(p.gotB, delivered{"route.live", "types.T", "y"}) {
		return fmt.Sprintf("late duplicates dropped the bound route (B routes %d, unbound drops %d)", p.b.Routes(), p.b.Stats().DroppedUnbound)
	}
	// Answers are not checked here: if a side's probation after its own Restart was ended
	// by a late duplicate that overtook the peer's live answer, that side takes the old
	// session for the peer's and answers its later duplicates (the residual in design §4;
	// TestLateDuplicatesAfterReconnectKeepRoutes checks the plain case). The route still
	// delivers, and there is at most one answer per injected Hello.
	if n := len(p.fa.hellos) + len(p.fb.hellos) - hellosBefore; n > injected {
		return fmt.Sprintf("%d late duplicates caused %d Hellos", injected, n)
	}
	return ""
}

func helloSession(frame []byte) uint64 {
	var f linktypes.LinkFrame
	_ = proto.Unmarshal(frame, &f)
	return f.GetHello().GetSession()
}
