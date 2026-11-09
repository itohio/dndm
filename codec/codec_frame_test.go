package codec

import (
	"bytes"
	"encoding/binary"
	stderrors "errors"
	"fmt"
	"io"
	"math"
	"sync"
	"testing"
	"testing/iotest"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/errors"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingPool wraps the real pool and counts calls. It refuses to allocate
// huge requests so a broken limit cannot exhaust the test process.
type countingPool struct {
	inner        bufferPool
	mu           sync.Mutex
	gets, puts   int
	maxRequested int
}

func (p *countingPool) Get(length int) []byte {
	p.mu.Lock()
	p.gets++
	p.maxRequested = max(p.maxRequested, length)
	p.mu.Unlock()
	if length > 64<<20 {
		return nil
	}
	return p.inner.Get(length)
}

func (p *countingPool) Put(buf []byte) {
	p.mu.Lock()
	p.puts++
	p.mu.Unlock()
	p.inner.Put(buf)
}

func (p *countingPool) balanced() (gets, puts int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.gets, p.puts
}

// useCountingPool installs a counting pool for one test. Tests using it must not run in parallel.
func useCountingPool(t testing.TB) *countingPool {
	old := buffers
	cp := &countingPool{inner: old}
	buffers = cp
	t.Cleanup(func() { buffers = old })
	return cp
}

func fooFrame(t testing.TB, text string) ([]byte, dndm.Route) {
	route, err := dndm.NewRoute("frame", &testtypes.Foo{})
	require.NoError(t, err)
	buf, err := EncodeMessage(&testtypes.Foo{Text: text}, route)
	require.NoError(t, err)
	frame := bytes.Clone(buf)
	Release(buf)
	return frame, route
}

func preamble(size uint32) []byte {
	b := binary.BigEndian.AppendUint32(nil, MagicNumber)
	return binary.BigEndian.AppendUint32(b, size)
}

// T1: valid frames decode however the reader splits them.
func TestReadMessageFragmented(t *testing.T) {
	f1, route := fooFrame(t, "first")
	f2, _ := fooFrame(t, "second")
	stream := append(bytes.Clone(f1), f2...)
	interests := map[string]dndm.Route{route.ID(): route}

	readers := map[string]func() io.Reader{
		"one-byte": func() io.Reader { return iotest.OneByteReader(bytes.NewReader(stream)) },
		"half":     func() io.Reader { return iotest.HalfReader(bytes.NewReader(stream)) },
	}
	for k := 1; k < len(stream); k++ {
		readers[fmt.Sprintf("split-%03d", k)] = func() io.Reader {
			return io.MultiReader(bytes.NewReader(stream[:k]), bytes.NewReader(stream[k:]))
		}
	}

	for name, newReader := range readers {
		t.Run(name, func(t *testing.T) {
			r := newReader()
			for _, want := range []string{"first", "second"} {
				buf, _, err := ReadMessage(r)
				require.NoError(t, err)
				_, msg, err := DecodeMessage(buf, interests)
				Release(buf)
				require.NoError(t, err)
				assert.Equal(t, want, msg.(*testtypes.Foo).Text)
			}
			_, _, err := ReadMessage(r)
			assert.ErrorIs(t, err, io.EOF, "clean EOF at a frame boundary")
		})
	}
}

// T2: body lengths outside [8, max] fail before any allocation above max.
func TestReadMessageLengthBounds(t *testing.T) {
	const limit = 1024
	tests := []struct {
		size uint32
		ok   bool
	}{
		{0, false}, {1, false}, {3, false}, {7, false},
		{8, true}, {limit, true},
		{limit + 1, false}, {1 << 31, false}, {math.MaxUint32, false},
	}
	for _, tt := range tests {
		cp := useCountingPool(t)
		in := preamble(tt.size)
		if tt.ok {
			in = append(in, make([]byte, tt.size)...)
		}
		buf, _, err := ReadMessageLimit(bytes.NewReader(in), limit)
		if tt.ok {
			assert.NoError(t, err, "size %d", tt.size)
			assert.Len(t, buf, int(tt.size))
			Release(buf)
		} else {
			assert.ErrorIs(t, err, errors.ErrBadFrame, "size %d", tt.size)
		}
		assert.LessOrEqual(t, cp.maxRequested, limit, "size %d allocated above the limit", tt.size)
	}

	cp := useCountingPool(t)
	_, _, err := ReadMessage(bytes.NewReader(preamble(DefaultMaxFrameSize + 1)))
	assert.ErrorIs(t, err, errors.ErrBadFrame)
	assert.LessOrEqual(t, cp.maxRequested, DefaultMaxFrameSize)
}

// T3: truncated or inconsistent nested lengths return an error, never panic.
func TestDecodeMessageTruncated(t *testing.T) {
	frame, _ := fooFrame(t, "x")
	hdrLen := binary.BigEndian.Uint32(frame[8:12])
	header := frame[12 : 12+hdrLen]
	headerOnly := append(binary.BigEndian.AppendUint32(nil, hdrLen), header...)
	wrongMsgLen := binary.BigEndian.AppendUint32(bytes.Clone(headerOnly), 10)
	wrongMsgLen = append(wrongMsgLen, 1, 2)
	bigHeader := append(binary.BigEndian.AppendUint32(nil, 3000), make([]byte, 3000+4)...)

	bodies := map[string][]byte{
		"nil":                   nil,
		"1 byte":                {0},
		"3 bytes":               {0, 0, 0},
		"header len > body":     {0, 0, 0, 10, 1, 2},
		"header len > 2048":     bigHeader,
		"header only":           headerOnly,
		"message len != remain": wrongMsgLen,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			var err error
			require.NotPanics(t, func() { _, _, err = DecodeMessage(body, nil) })
			assert.ErrorIs(t, err, errors.ErrBadFrame)
		})
	}
}

// T4: every error path after a pool Get returns the buffer.
func TestReadMessageReleasesOnError(t *testing.T) {
	boom := stderrors.New("boom")
	inputs := map[string]io.Reader{
		"eof after preamble": bytes.NewReader(preamble(100)),
		"eof mid body":       bytes.NewReader(append(preamble(100), make([]byte, 50)...)),
		"error mid body": io.MultiReader(
			bytes.NewReader(append(preamble(100), make([]byte, 10)...)),
			iotest.ErrReader(boom),
		),
		"bad magic": bytes.NewReader([]byte{1, 2, 3, 4, 0, 0, 0, 100}),
	}
	for name, r := range inputs {
		t.Run(name, func(t *testing.T) {
			cp := useCountingPool(t)
			_, _, err := ReadMessage(r)
			require.Error(t, err)
			gets, puts := cp.balanced()
			assert.Equal(t, gets, puts, "pool gets %d, puts %d", gets, puts)
		})
	}
}

// T5: arbitrary input never panics, never allocates above the limit, never leaks a buffer.
func FuzzReadDecode(f *testing.F) {
	frame, route := fooFrame(f, "seed")
	interests := map[string]dndm.Route{route.ID(): route}
	hdrLen := binary.BigEndian.Uint32(frame[8:12])
	headerOnly := append(binary.BigEndian.AppendUint32(nil, hdrLen), frame[12:12+hdrLen]...)

	f.Add(frame)
	f.Add(append(preamble(1), 0))
	f.Add(append(preamble(3), 0, 0, 0))
	f.Add(append(preamble(uint32(len(headerOnly))), headerOnly...))
	f.Add(preamble(1<<16 + 1))

	const limit = 1 << 16
	f.Fuzz(func(t *testing.T, data []byte) {
		cp := useCountingPool(t)
		buf, _, err := ReadMessageLimit(bytes.NewReader(data), limit)
		if err == nil {
			_, _, _ = DecodeMessage(buf, interests)
			Release(buf)
		}
		gets, puts := cp.balanced()
		if gets != puts {
			t.Fatalf("pool gets %d, puts %d", gets, puts)
		}
		if cp.maxRequested > limit {
			t.Fatalf("allocated %d > limit %d", cp.maxRequested, limit)
		}
	})
}
