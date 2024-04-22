package codec

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/itohio/dndm"
	types "github.com/itohio/dndm/types/core"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func BenchmarkEncodeMessage(b *testing.B) {
	msg := &testtypes.Foo{Text: "This is a test message"}
	route, _ := dndm.NewRoute("path.somewhat.long", msg)

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		_, err := EncodeMessage(msg, route)
		if err != nil {
			b.Fatal("EncodeMessage failed:", err)
		}
	}
}

func BenchmarkDecodeMessage(b *testing.B) {
	msg := &testtypes.Foo{Text: "This is a test message"}
	route, _ := dndm.NewRoute("path.somewhat.long", msg)

	datas := make([][]byte, 0, 1024)
	for i := 0; i < 10240; i++ {
		data, _ := EncodeMessage(msg, route)
		datas = append(datas, data[8:])
	}

	// Prepare the interests map
	interests := make(map[string]dndm.Route)
	interests[route.ID()] = route

	// Preparing the benchmark
	b.ResetTimer()
	j := 1
	for i := 0; i < b.N; i++ {
		header, message, err := DecodeMessage(datas[j], interests)
		if err != nil {
			b.Fatal("DecodeMessage failed:", err)
		}
		j *= 3
		if j >= len(datas) {
			j = j % (len(datas))
		}
		// Optionally use header and message to ensure they are correctly processed,
		// but be mindful not to include heavy operations here
		_ = header
		_ = message
	}
}

func TestDecodeMessage(t *testing.T) {
	tests := []struct {
		name    string
		init    func() ([]byte, map[string]dndm.Route)
		inspect func(t *testing.T, h *types.Header, m proto.Message)
		wantErr bool
	}{
		{
			name: "intent",
			init: func() ([]byte, map[string]dndm.Route) {
				i := &types.Intent{
					Route: "id",
					Hops:  123,
					Ttl:   456,
				}
				r, err := dndm.NewRoute("my-route", &testtypes.Foo{})
				require.NoError(t, err)
				buf, err := EncodeMessage(i, r)
				require.NoError(t, err)
				return buf, map[string]dndm.Route{}
			},
			inspect: func(t *testing.T, h *types.Header, m proto.Message) {
				assert.Equal(t, types.Type_INTENT, h.Type)
				mm, ok := m.(*types.Intent)
				assert.True(t, ok)
				assert.Equal(t, "id", mm.Route)
				assert.Equal(t, uint64(123), mm.Hops)
				assert.Equal(t, uint64(456), mm.Ttl)
			},
			wantErr: false,
		},
		{
			name: "interest",
			init: func() ([]byte, map[string]dndm.Route) {
				i := &types.Interest{
					Route: "id",
					Hops:  123,
					Ttl:   456,
				}
				r, err := dndm.NewRoute("my-route", &testtypes.Foo{})
				require.NoError(t, err)
				buf, err := EncodeMessage(i, r)
				require.NoError(t, err)
				return buf, map[string]dndm.Route{}
			},
			inspect: func(t *testing.T, h *types.Header, m proto.Message) {
				assert.Equal(t, types.Type_INTEREST, h.Type)
				mm, ok := m.(*types.Interest)
				assert.True(t, ok)
				assert.Equal(t, "id", mm.Route)
				assert.Equal(t, uint64(123), mm.Hops)
				assert.Equal(t, uint64(456), mm.Ttl)
			},
			wantErr: false,
		},
		{
			name: "ping",
			init: func() ([]byte, map[string]dndm.Route) {
				i := &testtypes.Foo{
					Text: "some-important-text",
				}
				r, err := dndm.NewRoute("my-route", &testtypes.Foo{})
				require.NoError(t, err)
				buf, err := EncodeMessage(i, r)
				require.NoError(t, err)
				return buf, map[string]dndm.Route{r.ID(): r}
			},
			inspect: func(t *testing.T, h *types.Header, m proto.Message) {
				assert.Equal(t, types.Type_MESSAGE, h.Type)
				mm, ok := m.(*testtypes.Foo)
				assert.True(t, ok)
				assert.Equal(t, "some-important-text", mm.Text)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, interests := tt.init()
			defer buffers.Put(data)

			tMagic := binary.BigEndian.Uint32(data)
			assert.Equal(t, uint32(MagicNumber), tMagic)

			tSize := binary.BigEndian.Uint32(data[4:])
			assert.Equal(t, tSize+8, uint32(len(data)))

			buf := bytes.NewBuffer(append(data, make([]byte, 1024)...))
			data, _, err := ReadMessage(buf)
			if err != nil {
				t.Errorf("ReadMessage() error = %v", err)
				buffers.Put(data)
				return
			}

			h, got, err := DecodeMessage(data, interests)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeMessage() error = %v, wantErr %v", err, tt.wantErr)
				buffers.Put(data)
				return
			}
			tt.inspect(t, h, got)
		})
	}
}
