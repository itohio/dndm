package stream

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/codec"
	"github.com/itohio/dndm/errors"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/require"
)

// loopReader serves the same frame again and again, like a peer that keeps sending.
type loopReader struct {
	frame []byte
	r     *bytes.Reader
}

func (l *loopReader) Read(p []byte) (int, error) {
	if l.r.Len() == 0 {
		l.r.Reset(l.frame)
	}
	return l.r.Read(p)
}
func (l *loopReader) Write(p []byte) (int, error) { return len(p), nil }

func readAllocs(t *testing.T, payload int) float64 {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	route, err := dndm.NewRoute("alloc", &testtypes.Foo{})
	require.NoError(t, err)
	frame, err := codec.EncodeMessage(&testtypes.Foo{Text: strings.Repeat("x", payload)}, route)
	require.NoError(t, err)
	src := &loopReader{frame: bytes.Clone(frame), r: bytes.NewReader(frame)}
	s := NewWithContext(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), src, nil)
	defer s.Close()
	s.AddRoute(route)
	for range 8 { // warm the buffer pools
		_, _, err := s.Read(ctx)
		require.NoError(t, err)
	}
	return testing.AllocsPerRun(50, func() {
		if _, _, err := s.Read(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

// Review R-DND-R5 F4: since the read loop uses a fixed 1 KiB buffer, a frame body is
// read in 1 KiB requests, each allocating its own channels, so allocations grow with the
// frame size. Reading a 64 KiB frame must cost about as many allocations as a 64 B one.
func TestStreamContext_ReadAllocsDoNotGrowWithFrameSize(t *testing.T) {
	small := readAllocs(t, 64)
	large := readAllocs(t, 64<<10)
	t.Logf("allocations per frame: 64 B %.0f, 64 KiB %.0f", small, large)
	require.LessOrEqual(t, large, small+4, "allocations grow with the frame size")
}

// A large read (pooled buffer) that gives up keeps its bytes for the next read, like a
// small one (scratch buffer), and never writes the abandoned caller buffer.
func TestStreamContext_CancelledLargeReadKeepsBytes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rw := &signalFeed{feed: make(chan []byte), entered: make(chan struct{}, 1)}
	s := NewWithContext(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), rw, nil)
	defer s.Close()

	size := 4 * readScratchSize
	readCtx, readCancel := context.WithCancel(ctx)
	first := make([]byte, size)
	done := make(chan error, 1)
	go func() {
		_, err := s.Reader(readCtx).Read(first)
		done <- err
	}()
	<-rw.entered // the loop is inside Read with a pooled buffer for this request
	readCancel()
	require.ErrorIs(t, <-done, context.Canceled)

	data := bytes.Repeat([]byte("L"), size)
	rw.feed <- data // arrives after the first read gave up

	second := make([]byte, size)
	n, err := io.ReadFull(s.Reader(ctx), second)
	require.NoError(t, err)
	require.Equal(t, data, second[:n])
	require.Equal(t, make([]byte, size), first, "the abandoned read's buffer was written")
}

// signalFeed is a feedReader that reports when a Read starts.
type signalFeed struct {
	feed    chan []byte
	entered chan struct{}
}

func (r *signalFeed) Read(p []byte) (int, error) {
	select {
	case r.entered <- struct{}{}:
	default:
	}
	return copy(p, <-r.feed), nil
}
func (r *signalFeed) Write(p []byte) (int, error) { return len(p), nil }
