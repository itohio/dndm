package pipe

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/itohio/dndm/routers"
	"github.com/itohio/dndm/routers/pipe/types"
	"google.golang.org/protobuf/proto"
)

func TestDecodeMessage(t *testing.T) {
	tests := []struct {
		name    string
		init    func() ([]byte, map[string]routers.Route)
		inspect func(t *testing.T, h *types.Header, m proto.Message)
		wantErr bool
	}{
		{
			name: "intent",
			init: func() ([]byte, map[string]routers.Route) {
				i := &types.Intent{
					Id:   "id",
					Hops: 123,
					Ttl:  456,
				}
				r, err := routers.NewRoute("my-route", &types.TextMessage{})
				if err != nil {
					panic(err)
				}
				buf, err := EncodeMessage(i, "my-id", r)
				if err != nil {
					panic(err)
				}
				return buf, map[string]routers.Route{}
			},
			inspect: func(t *testing.T, h *types.Header, m proto.Message) {
				if h.Type != types.Type_INTENT {
					t.Errorf("DecodeMessage() type != Intent")
				}
				mm, ok := m.(*types.Intent)
				if !ok {
					t.Errorf("DecodeMessage() != Intent")
				}
				if mm.Id != "id" || mm.Hops != 123 || mm.Ttl != 456 {
					t.Errorf("DecodeMessage() = %v", mm)
				}
			},
			wantErr: false,
		},
		{
			name: "interest",
			init: func() ([]byte, map[string]routers.Route) {
				i := &types.Interest{
					Id:   "id",
					Hops: 123,
					Ttl:  456,
				}
				r, err := routers.NewRoute("my-route", &types.TextMessage{})
				if err != nil {
					panic(err)
				}
				buf, err := EncodeMessage(i, "my-id", r)
				if err != nil {
					panic(err)
				}
				return buf, map[string]routers.Route{}
			},
			inspect: func(t *testing.T, h *types.Header, m proto.Message) {
				if h.Type != types.Type_INTEREST {
					t.Errorf("DecodeMessage() type != Interest")
				}
				mm, ok := m.(*types.Interest)
				if !ok {
					t.Errorf("DecodeMessage() != Interest")
				}
				if mm.Id != "id" || mm.Hops != 123 || mm.Ttl != 456 {
					t.Errorf("DecodeMessage() = %v", mm)
				}
			},
			wantErr: false,
		},
		{
			name: "ping",
			init: func() ([]byte, map[string]routers.Route) {
				i := &types.TextMessage{
					Text: "some-important-text",
				}
				r, err := routers.NewRoute("my-route", &types.TextMessage{})
				if err != nil {
					panic(err)
				}
				buf, err := EncodeMessage(i, "my-id", r)
				if err != nil {
					panic(err)
				}
				return buf, map[string]routers.Route{r.String(): r}
			},
			inspect: func(t *testing.T, h *types.Header, m proto.Message) {
				if h.Type != types.Type_MESSAGE {
					t.Errorf("DecodeMessage() type != Message")
				}
				mm, ok := m.(*types.TextMessage)
				if !ok {
					t.Errorf("DecodeMessage() != TextMessage")
				}
				if mm.Text != "some-important-text" {
					t.Errorf("DecodeMessage() = %v", mm)
				}
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, interests := tt.init()
			defer buffers.Put(data)

			tMagic := binary.BigEndian.Uint32(data)
			if tMagic != MagicNumber {
				t.Errorf("EncodeMessage() want MagicNumber=%d got %d", MagicNumber, tMagic)
			}

			tSize := binary.BigEndian.Uint32(data[4:])
			if tSize+8 != uint32(len(data)) {
				t.Errorf("EncodeMessage() want BufSize=%d got BufSize=%d", tSize+8, len(data))
			}

			buf := bytes.NewBuffer(append(data, make([]byte, 1024)...))
			data, err := ReadMessage(buf)
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