package codec

import (
	"encoding/binary"
	"fmt"
	"io"
	reflect "reflect"
	"time"

	"github.com/itohio/dndm/errors"
	routers "github.com/itohio/dndm/routers"
	types "github.com/itohio/dndm/types/core"
	pool "github.com/libp2p/go-buffer-pool"
	"google.golang.org/protobuf/proto"
)

var (
	buffers pool.BufferPool
)

func Release(b []byte) {
	buffers.Put(b)
}

// Size of the preamble: 4 bytes magic number, 4 bytes total message size, 4 bytes header size, 4 bytes message size
const PreambleSize = 4 + 4 + 4 + 4
const MagicNumber = 0xFADABEDA
const MagicNumberHeaderless = 0xCEBAFE4A

// EncodeMessage encodes any proto message into stream bytes. It adds a header and packet part sizes.
func EncodeMessage(msg proto.Message, route routers.Route) ([]byte, error) {
	h := &types.Header{
		Timestamp: uint64(time.Now().UnixNano()),
		Type:      resolveType(msg),
		Route:     route.String(),
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
	buf, err = proto.MarshalOptions{}.MarshalAppend(buf, hdr)
	if err != nil {
		buffers.Put(buf)
		return nil, err
	}

	buf = binary.BigEndian.AppendUint32(buf, uint32(mSize))
	buf, err = proto.MarshalOptions{}.MarshalAppend(buf, msg)
	if err != nil {
		buffers.Put(buf)
		return nil, err
	}

	return buf, nil
}

func ReadMessage(r io.Reader) ([]byte, uint64, error) {
	bufPreamble := [8]byte{}
	n, err := r.Read(bufPreamble[:])
	if err != nil {
		return nil, 0, err
	}
	now := uint64(time.Now().UnixNano())
	if n != 8 {
		return nil, 0, errors.ErrNotEnoughBytes
	}
	magic := int(binary.BigEndian.Uint32(bufPreamble[:]))
	if magic != MagicNumber {
		return nil, 0, errors.ErrBadArgument
	}
	size := int(binary.BigEndian.Uint32(bufPreamble[4:]))
	buf := buffers.Get(size)
	n, err = r.Read(buf)
	if err != nil {
		return nil, 0, err
	}
	if n != size {
		return nil, 0, errors.ErrNotEnoughBytes
	}

	return buf, now, nil
}

// DecodeMessage assumes the data array is already of correct size and preamble size field is removed
func DecodeMessage(data []byte, interests map[string]routers.Route) (*types.Header, proto.Message, error) {
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
	if route.Type() == nil {
		return &h, nil, fmt.Errorf("%w: %s type is nil", errors.ErrInvalidType, h.Route)
	}
	t := route.Type().Elem()

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
