package pipe

import (
	"encoding/binary"
	"fmt"
	"io"
	reflect "reflect"
	sync "sync"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/router"
	types "github.com/itohio/dndm/router/pipe/types"
	pool "github.com/libp2p/go-buffer-pool"
	"google.golang.org/protobuf/proto"
)

var buffers pool.BufferPool

// Size of the preamble: 4 bytes magic number, 4 bytes total message size, 4 bytes header size, 4 bytes message size
const PreambleSize = 4 + 4 + 4 + 4
const MagicNumber = 0xFADABEDA
const MagicNumberHeaderless = 0xCEBAFE4A

var (
	knownTypes         = map[types.Type]reflect.Type{}
	knownTypesReversed = map[reflect.Type]types.Type{}
	mu                 sync.Mutex
)

func init() {
	RegisterType(types.Type_NOTIFY_INTENT, &types.NotifyIntent{})
	RegisterType(types.Type_INTENT, &types.Intent{})
	RegisterType(types.Type_INTENTS, &types.Intents{})
	RegisterType(types.Type_INTEREST, &types.Interest{})
	RegisterType(types.Type_INTERESTS, &types.Interests{})
	RegisterType(types.Type_HANDSHAKE, &types.Handshake{})
	RegisterType(types.Type_PEERS, &types.Peers{})
	RegisterType(types.Type_PING, &types.Ping{})
	RegisterType(types.Type_PONG, &types.Pong{})
}

func RegisterType(t types.Type, msg proto.Message) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := knownTypes[t]; ok {
		panic(fmt.Errorf("type already registered: %v", t))
	}
	tt := reflect.TypeOf(msg).Elem()
	knownTypes[t] = tt
	knownTypesReversed[tt] = t
}

func resolveType(msg proto.Message) types.Type {
	tt := reflect.TypeOf(msg).Elem()
	if t, ok := knownTypesReversed[tt]; ok {
		return t
	}
	return types.Type_MESSAGE
}

// EncodeMessage encodes any proto message into stream bytes. It adds a header and packet part sizes.
func EncodeMessage(msg proto.Message, id string, route router.Route) ([]byte, error) {
	h := &types.Header{
		Id:    id,
		Type:  resolveType(msg),
		Route: route.String(),
	}
	return AppendMessageTo(nil, h, msg)
}

func PacketSize(hdr *types.Header, msg proto.Message) int {
	hSize := proto.Size(hdr)
	mSize := proto.Size(msg)
	return hSize + mSize + PreambleSize
}

func AppendMessageTo(buf []byte, hdr *types.Header, msg proto.Message) ([]byte, error) {
	if hdr == nil {
		return nil, errors.ErrBadArgument
	}
	if msg == nil {
		return nil, errors.ErrBadArgument
	}

	hSize := proto.Size(hdr)
	mSize := proto.Size(msg)

	if buf == nil {
		buf = buffers.Get(PacketSize(hdr, msg))
		buf = buf[:0]
	}

	// Write magic header and total message size
	buf = binary.BigEndian.AppendUint32(buf, MagicNumber)
	buf = binary.BigEndian.AppendUint32(buf, uint32(hSize+mSize+4+4))
	// Write header size
	buf = binary.BigEndian.AppendUint32(buf, uint32(hSize))

	var err error
	if hdr != nil {
		buf, err = proto.MarshalOptions{}.MarshalAppend(buf, hdr)
		if err != nil {
			buffers.Put(buf)
			return nil, err
		}
	}

	buf = binary.BigEndian.AppendUint32(buf, uint32(mSize))
	buf, err = proto.MarshalOptions{}.MarshalAppend(buf, msg)
	if err != nil {
		buffers.Put(buf)
		return nil, err
	}

	return buf, nil
}

func ReadMessage(r io.Reader) ([]byte, error) {
	bufPreamble := [8]byte{}
	n, err := r.Read(bufPreamble[:])
	if err != nil {
		return nil, err
	}
	if n != 8 {
		return nil, errors.ErrNotEnoughBytes
	}
	magic := int(binary.BigEndian.Uint32(bufPreamble[:]))
	if magic != MagicNumber {
		return nil, errors.ErrBadArgument
	}
	size := int(binary.BigEndian.Uint32(bufPreamble[4:]))
	buf := buffers.Get(size)
	n, err = r.Read(buf)
	if err != nil {
		return nil, err
	}
	if n != size {
		return nil, errors.ErrNotEnoughBytes
	}

	return buf, nil
}

// DecodeMessage assumes the data array is already of correct size and preamble size field is removed
func DecodeMessage(data []byte, interests map[string]router.Route) (*types.Header, proto.Message, error) {
	var h types.Header

	hSize := int(binary.BigEndian.Uint32(data))
	data = data[4:]
	if hSize > len(data) || hSize > 2048 {
		return nil, nil, fmt.Errorf("%w: header: %d", errors.ErrNotEnoughBytes, hSize)
	}

	err := proto.Unmarshal(data[:hSize], &h)
	if err != nil {
		return nil, nil, err
	}
	data = data[hSize:]

	mSize := int(binary.BigEndian.Uint32(data))
	data = data[4:]
	if mSize != len(data) {
		return &h, nil, fmt.Errorf("%w: message: %d", errors.ErrNotEnoughBytes, mSize)
	}

	// Allow extendability
	if t, ok := knownTypes[h.Type]; ok {
		instance := reflect.New(t).Interface()
		msg, ok := instance.(proto.Message)
		if !ok {
			panic(fmt.Errorf("bad type encountered: %v", t))
		}

		if err := proto.Unmarshal(data, msg); err != nil {
			return &h, nil, err
		}

		return &h, msg, err
	}

	if interests == nil {
		return &h, nil, nil
	}

	// Decode message types
	route, ok := interests[h.Route]
	if !ok {
		return &h, nil, fmt.Errorf("%w: %s", errors.ErrNoInterest, h.Route)
	}

	t := reflect.TypeOf(route.Type()).Elem()

	// Create a new instance of the appropriate type.
	instance := reflect.New(t).Interface()

	// Assert that the created instance is a proto.Message.
	msg, ok := instance.(proto.Message)
	if !ok {
		return &h, nil, errors.ErrInvalidRoute
	}

	if err := proto.Unmarshal(data, msg); err != nil {
		return &h, nil, err
	}

	return &h, msg, nil
}
