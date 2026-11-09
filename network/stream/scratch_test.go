package stream

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/codec"
	"github.com/itohio/dndm/errors"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// frameSource serves fixed bytes to Read and discards writes.
type frameSource struct{ r *bytes.Reader }

func (f frameSource) Read(p []byte) (int, error)  { return f.r.Read(p) }
func (f frameSource) Write(p []byte) (int, error) { return len(p), nil }

// Review R-DND-R5 F1: a StreamContext's read buffer must not grow with the frame size.
// codec.ReadMessage asks io.ReadFull for a whole frame body, so a scratch buffer sized to
// the request would keep the largest frame (up to 4 MiB) per stream for its whole life.
func TestStreamContext_ReadScratchStaysSmall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	route, err := dndm.NewRoute("big", &testtypes.Foo{})
	require.NoError(t, err)
	text := strings.Repeat("x", 256<<10)
	frame, err := codec.EncodeMessage(&testtypes.Foo{Text: text}, route)
	require.NoError(t, err)

	s := NewWithContext(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), frameSource{bytes.NewReader(frame)}, nil)
	defer s.Close()
	s.AddRoute(route)

	_, msg, err := s.Read(ctx)
	require.NoError(t, err)
	assert.Len(t, msg.(*testtypes.Foo).Text, len(text), "the large frame must still arrive whole")
	// Run is idle now and handed its last result over a channel, so reading scratch is ordered.
	assert.LessOrEqual(t, cap(s.read.scratch), 4096, "read scratch grew with the frame size")
}
