package bus

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/itohio/dndm/endpoint/direct"
	types "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tagCounter counts received "tag-<n>" messages per tag.
type tagCounter struct {
	mu     sync.Mutex
	counts map[int]int
	warm   int
}

func (c *tagCounter) run(ctx context.Context, consumer *Consumer[*types.Foo]) {
	for {
		m, err := consumer.Receive(ctx)
		if err != nil {
			return
		}
		c.mu.Lock()
		if n, ok := strings.CutPrefix(m.Text, "tag-"); ok {
			i, _ := strconv.Atoi(n)
			c.counts[i]++
		} else {
			c.warm++
		}
		c.mu.Unlock()
	}
}

func (c *tagCounter) snapshot() (map[int]int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[int]int, len(c.counts))
	for k, v := range c.counts {
		out[k] = v
	}
	return out, c.warm
}

// DND-003: every message published on router A reaches the subscriber on router B exactly
// once. The remote interest that B registers on A must have a single reader, the forwarder
// to B; a second reader on A (a router fan-in built from the remote interest) steals
// remote-bound messages.
func TestRemoteForwardingHasOneReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	remoteA, remoteB := newStreamPair(t, ctx, "fwd-a", "fwd-b", 4)
	routerA := newRouterWithEndpoints(t, ctx, direct.New(4), remoteA)
	routerB := newRouterWithEndpoints(t, ctx, direct.New(4), remoteB)

	remoteSub, err := NewConsumer[*types.Foo](ctx, routerB, "fwd.path")
	require.NoError(t, err)
	defer remoteSub.Close()
	producer, err := NewProducer[*types.Foo](ctx, routerA, "fwd.path")
	require.NoError(t, err)
	defer producer.Close()

	remote := &tagCounter{counts: map[int]int{}}
	go remote.run(ctx, remoteSub)

	// Warm up until the remote interest is registered on A and forwarding works.
	require.Eventually(t, func() bool {
		sendCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		_ = producer.Send(sendCtx, &types.Foo{Text: "warm"})
		_, warm := remote.snapshot()
		return warm > 0
	}, 5*time.Second, 50*time.Millisecond, "forwarding to the remote router never started")

	// One message in flight at a time: subscriber queues drop silently when full (DND-006),
	// so pacing keeps this test about stealing, not overload.
	const n = 20
	for i := range n {
		require.NoError(t, producer.Send(ctx, &types.Foo{Text: "tag-" + strconv.Itoa(i)}))
		require.Eventually(t, func() bool {
			counts, _ := remote.snapshot()
			return counts[i] > 0
		}, time.Second, time.Millisecond, "remote subscriber missed tag-%d", i)
	}
	time.Sleep(100 * time.Millisecond) // let any duplicate arrive

	counts, _ := remote.snapshot()
	for i := range n {
		assert.Equal(t, 1, counts[i], "remote subscriber: tag-%d", i)
	}
}
