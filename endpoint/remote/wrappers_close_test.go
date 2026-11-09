package remote

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/network"
	types "github.com/itohio/dndm/types/core"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// writeRecorder is a network.Conn that only supports Write; it counts nil messages.
type writeRecorder struct {
	network.Conn
	nils, writes atomic.Int32
}

func (w *writeRecorder) Write(_ context.Context, _ dndm.Route, msg proto.Message) error {
	w.writes.Add(1)
	if msg == nil {
		w.nils.Add(1)
	}
	return nil
}

// DND-008 G4 (found while stressing the alias subtest for DND-022 follow-up 3): closing a
// remote interest cancels its context and then closes its channel. The forwarder selected
// on both, so it could take the closed channel's zero value and write a nil message,
// which panicked in codec.EncodeMessage (3 of 48 stressed processes crashed).
func TestRemoteInterestCloseWritesNoNil(t *testing.T) {
	route, err := dndm.NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	conn := &writeRecorder{}
	wrap := wrapRemoteInterest(slog.Default(), conn, &types.Interest{Route: route.ID()})

	const n = 300
	var wg sync.WaitGroup
	for range n {
		li := dndm.NewInterest(context.Background(), route, 1)
		ri, err := wrap(li)
		require.NoError(t, err)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ri.Close()
		}()
	}
	wg.Wait()
	time.Sleep(50 * time.Millisecond) // let the forwarders observe the close
	assert.Zero(t, conn.nils.Load(), "nil messages written after close (of %d writes)", conn.writes.Load())
}
