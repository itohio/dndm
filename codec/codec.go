// Package codec frames protobuf messages on byte streams.
//
// A frame is a big-endian preamble (4-byte MagicNumber, 4-byte body length) followed by
// the body: a 4-byte header length, the encoded types.Header, a 4-byte message length and
// the encoded message. Readers accept bodies of 8 bytes up to a cap (DefaultMaxFrameSize
// unless ReadMessageLimit is given another), read until a frame is complete, and report
// malformed or truncated frames as errors wrapping errors.ErrBadFrame. The format has no
// delimiter, so a stream that produced ErrBadFrame cannot be resynchronised: close it.
package codec

import (
	"encoding/binary"
	"fmt"
	"io"
	reflect "reflect"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/errors"
	types "github.com/itohio/dndm/types/core"
	pool "github.com/libp2p/go-buffer-pool"
	"google.golang.org/protobuf/proto"
)

// bufferPool is the allocator seam for frame buffers; tests swap in a counting pool.
type bufferPool interface {
	Get(length int) []byte
	Put(buf []byte)
}

var buffers bufferPool = &pool.BufferPool{}

func Release(b []byte) {
	buffers.Put(b)
}

// Size of the preamble: 4 bytes magic number, 4 bytes total message size, 4 bytes header size, 4 bytes message size
const PreambleSize = 4 + 4 + 4 + 4
const MagicNumber = 0xFADABEDA
const MagicNumberHeaderless = 0xCEBAFE4A

// DefaultMaxFrameSize caps the body of one stream frame read by ReadMessage.
const DefaultMaxFrameSize = 4 << 20

// minFrameBody is the smallest valid body: a 4-byte header length and a 4-byte message length.
const minFrameBody = 4 + 4

// maxHeaderSize caps the encoded types.Header inside a frame body.
const maxHeaderSize = 2048

// ReadMessage reads one frame with a body of at most DefaultMaxFrameSize bytes.
// See ReadMessageLimit.
func ReadMessage(r io.Reader) ([]byte, uint64, error) {
	return ReadMessageLimit(r, DefaultMaxFrameSize)
}

// ReadMessageLimit reads one frame whose body may not exceed maxBody bytes and returns
// the body (header length, header, message length, message) in a pooled buffer that the
// caller must Release, plus the local receive time in Unix nanoseconds.
//
// It reads until the frame is complete, so readers may return any split of the bytes.
// A clean end of input before a frame starts returns io.EOF. Every other failure (bad
// magic, a body length outside [8, maxBody], input ending inside a frame) returns an error
// wrapping errors.ErrBadFrame. The format has no delimiter to resynchronise on, so after
// such an error the caller must close the stream. No buffer is allocated before the
// length is validated, and none is retained on error. A maxBody below the smallest valid
// body (8) is a caller error and returns errors.ErrBadArgument without reading.
func ReadMessageLimit(r io.Reader, maxBody int) ([]byte, uint64, error) {
	if maxBody < minFrameBody {
		return nil, 0, fmt.Errorf("%w: frame body cap %d below %d", errors.ErrBadArgument, maxBody, minFrameBody)
	}
	var preamble [8]byte
	if _, err := io.ReadFull(r, preamble[:]); err != nil {
		if err == io.EOF {
			return nil, 0, io.EOF
		}
		return nil, 0, fmt.Errorf("%w: preamble: %w", errors.ErrBadFrame, err)
	}
	now := uint64(time.Now().UnixNano())
	if magic := binary.BigEndian.Uint32(preamble[:]); magic != MagicNumber {
		return nil, 0, fmt.Errorf("%w: magic %#08x", errors.ErrBadFrame, magic)
	}
	size := uint64(binary.BigEndian.Uint32(preamble[4:]))
	if size < minFrameBody || size > uint64(maxBody) {
		return nil, 0, fmt.Errorf("%w: body length %d outside [%d, %d]", errors.ErrBadFrame, size, minFrameBody, maxBody)
	}
	buf := buffers.Get(int(size))
	if _, err := io.ReadFull(r, buf); err != nil {
		buffers.Put(buf)
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, 0, fmt.Errorf("%w: body: %w", errors.ErrBadFrame, err)
	}
	return buf, now, nil
}

// EncodeMessage encodes any proto message into stream bytes. It adds a header and packet part sizes.
func EncodeMessage(msg proto.Message, route dndm.Route) ([]byte, error) {
	h := &types.Header{
		Timestamp: uint64(time.Now().UnixNano()),
		Type:      resolveType(msg),
		Route:     route.ID(),
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

// DecodeMessage decodes a frame body returned by ReadMessage (preamble already removed).
// Inconsistent nested lengths return an error wrapping errors.ErrBadFrame.
func DecodeMessage(data []byte, interests map[string]dndm.Route) (*types.Header, proto.Message, error) {
	var h types.Header

	if len(data) < 4 {
		return nil, nil, fmt.Errorf("%w: body of %d bytes has no header length", errors.ErrBadFrame, len(data))
	}
	hSize := uint64(binary.BigEndian.Uint32(data))
	data = data[4:]
	if hSize > maxHeaderSize || hSize > uint64(len(data)) {
		return nil, nil, fmt.Errorf("%w: header length %d, %d bytes left, max %d", errors.ErrBadFrame, hSize, len(data), maxHeaderSize)
	}

	err := proto.Unmarshal(data[:hSize], &h)
	if err != nil {
		return nil, nil, err
	}
	data = data[hSize:]

	if len(data) < 4 {
		return &h, nil, fmt.Errorf("%w: no message length after header", errors.ErrBadFrame)
	}
	mSize := uint64(binary.BigEndian.Uint32(data))
	data = data[4:]
	if mSize != uint64(len(data)) {
		return &h, nil, fmt.Errorf("%w: message length %d, %d bytes left", errors.ErrBadFrame, mSize, len(data))
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
