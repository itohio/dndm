package stream

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/codec"
	"github.com/itohio/dndm/errors"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DND-008: frame writes are serialised, and contextRW never touches a caller's buffer
// after Request returned.

// chunkWriter accepts each Write in 3-byte pieces with a yield in between, like a
// serial port or any writer that is not atomic per call.
type chunkWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *chunkWriter) Read(p []byte) (int, error) { select {} }

func (w *chunkWriter) Write(p []byte) (int, error) {
	for i := 0; i < len(p); i += 3 {
		end := min(i+3, len(p))
		w.mu.Lock()
		w.buf.Write(p[i:end])
		w.mu.Unlock()
		runtime.Gosched()
	}
	return len(p), nil
}

func TestStream_ConcurrentWritesStayFramed(t *testing.T) {
	ctx := context.Background()
	route, err := dndm.NewRoute("frames", &testtypes.Foo{})
	require.NoError(t, err)
	w := &chunkWriter{}
	s := New(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), w, nil)

	const writers, each = 4, 25
	var wg sync.WaitGroup
	for g := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				assert.NoError(t, s.Write(ctx, route, &testtypes.Foo{Text: fmt.Sprintf("w%d-%d", g, i)}))
			}
		}()
	}
	wg.Wait()

	routes := map[string]dndm.Route{route.ID(): route}
	r := bytes.NewReader(w.buf.Bytes())
	seen := map[string]bool{}
	for r.Len() > 0 {
		buf, _, err := codec.ReadMessage(r)
		require.NoError(t, err, "frames interleaved after %d intact frames", len(seen))
		_, msg, err := codec.DecodeMessage(buf, routes)
		require.NoError(t, err, "frames interleaved after %d intact frames", len(seen))
		seen[msg.(*testtypes.Foo).Text] = true
	}
	assert.Len(t, seen, writers*each)
}

// feedReader blocks each Read until the test feeds bytes.
type feedReader struct {
	feed chan []byte
}

func (r *feedReader) Read(p []byte) (int, error)  { return copy(p, <-r.feed), nil }
func (r *feedReader) Write(p []byte) (int, error) { return len(p), nil }

func TestStreamContext_CancelledReadKeepsBytes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rw := &feedReader{feed: make(chan []byte)}
	s := NewWithContext(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), rw, nil)
	defer s.Close()

	readCtx, readCancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer readCancel()
	first := make([]byte, 4)
	_, err := s.Reader(readCtx).Read(first)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	rw.feed <- []byte("ABCD") // arrives after the first Read gave up

	nextCtx, nextCancel := context.WithTimeout(ctx, time.Second)
	defer nextCancel()
	second := make([]byte, 4)
	n, err := s.Reader(nextCtx).Read(second)
	require.NoError(t, err, "bytes read for an abandoned Read were lost")
	assert.Equal(t, "ABCD", string(second[:n]))
	assert.Equal(t, make([]byte, 4), first, "the abandoned Read's buffer was written after Read returned")
}

// blockingWriter holds the first Write until released, then records what it wrote.
type blockingWriter struct {
	entered chan struct{}
	release chan struct{}
	mu      sync.Mutex
	written [][]byte
}

func (w *blockingWriter) Read(p []byte) (int, error) { select {} }

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.entered <- struct{}{}
	<-w.release
	w.mu.Lock()
	w.written = append(w.written, bytes.Clone(p))
	w.mu.Unlock()
	return len(p), nil
}

func TestStreamContext_CancelledWriteKeepsItsBuffer(t *testing.T) {
	// One P: a buffer the cancelled Write released (if it did) is the next one
	// sync.Pool hands out on this goroutine.
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	route, err := dndm.NewRoute("frames", &testtypes.Foo{})
	require.NoError(t, err)
	w := &blockingWriter{entered: make(chan struct{}), release: make(chan struct{})}
	s := NewWithContext(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), w, nil)
	defer s.Close()

	writeCtx, writeCancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() { result <- s.Write(writeCtx, route, &testtypes.Foo{Text: "original"}) }()
	<-w.entered // the write loop is inside rw.Write with the frame buffer
	writeCancel()
	assert.ErrorIs(t, <-result, context.Canceled, "a cancelled write must report the cancellation")

	// Reuse pooled buffers the way other writers would (same frame size, same size class).
	for range 8 {
		b, err := codec.EncodeMessage(&testtypes.Foo{Text: "REUSED!!"}, route)
		require.NoError(t, err)
		codec.Release(b)
	}
	close(w.release)
	require.Eventually(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return len(w.written) == 1
	}, time.Second, time.Millisecond)

	w.mu.Lock()
	frame := w.written[0]
	w.mu.Unlock()
	buf, _, err := codec.ReadMessage(bytes.NewReader(frame))
	require.NoError(t, err)
	_, msg, err := codec.DecodeMessage(buf, map[string]dndm.Route{route.ID(): route})
	require.NoError(t, err)
	assert.Equal(t, "original", msg.(*testtypes.Foo).Text, "the cancelled write's buffer was reused while it was being written")
}
