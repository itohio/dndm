package codec

import (
	"fmt"
	"slices"

	"github.com/itohio/dndm/errors"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// Link-context DATA frames (DND-022, docs/design/link-context-wire.md §2-§3).
//
// A DATA frame is a types.link.LinkFrame with route_id != 0. This file encodes and decodes
// it with protowire, which produces exactly the bytes of the generated code (fields in
// number order, zero values omitted) without reflection, and decodes the payload as a
// sub-slice of the frame. Control frames (route_id 0) use the generated code.

// LinkFrame field numbers of the data path (types/link/link.proto).
const (
	linkRouteID protowire.Number = 1
	linkPayload protowire.Number = 2
	linkSeq     protowire.Number = 3
	linkFrag    protowire.Number = 4
	linkCtr     protowire.Number = 5
	linkSealed  protowire.Number = 6

	linkFirstControl protowire.Number = 16 // hello ... close, and the reserved 24-31
	linkLastControl  protowire.Number = 31
)

// LinkData is a DATA frame. Zero Seq, Frag and Ctr are absent on the wire. A frame carries
// Payload, or Sealed on a secured link, not both. After DecodeLinkData, Payload and Sealed
// alias the frame buffer.
type LinkData struct {
	RouteID uint32
	Seq     uint32
	Frag    uint32
	Ctr     uint32
	Payload []byte
	Sealed  []byte
}

// LinkDataSize returns the encoded size of d.
func LinkDataSize(d LinkData) int {
	n := protowire.SizeTag(linkRouteID) + protowire.SizeVarint(uint64(d.RouteID))
	n += sizeBytesField(linkPayload, len(d.Payload))
	n += sizeVarintField(linkSeq, d.Seq)
	n += sizeVarintField(linkFrag, d.Frag)
	n += sizeVarintField(linkCtr, d.Ctr)
	n += sizeBytesField(linkSealed, len(d.Sealed))
	return n
}

// AppendLinkData appends d encoded as a LinkFrame to b. A zero RouteID (the control space)
// or both Payload and Sealed set returns ErrBadArgument and b unchanged.
func AppendLinkData(b []byte, d LinkData) ([]byte, error) {
	if d.RouteID == 0 || (len(d.Payload) > 0 && len(d.Sealed) > 0) {
		return b, errors.ErrBadArgument
	}
	b = slices.Grow(b, LinkDataSize(d))
	b = appendVarintField(b, linkRouteID, d.RouteID, true)
	b = appendBytesField(b, linkPayload, d.Payload)
	b = appendVarintField(b, linkSeq, d.Seq, false)
	b = appendVarintField(b, linkFrag, d.Frag, false)
	b = appendVarintField(b, linkCtr, d.Ctr, false)
	b = appendBytesField(b, linkSealed, d.Sealed)
	return b, nil
}

// AppendLinkMessage appends a DATA frame whose payload is msg, marshalled straight into b
// after the tag and length (one buffer, no copy). A zero routeID or nil msg returns
// ErrBadArgument and b unchanged.
func AppendLinkMessage(b []byte, routeID, seq uint32, msg proto.Message) ([]byte, error) {
	if routeID == 0 || msg == nil {
		return b, errors.ErrBadArgument
	}
	size := proto.Size(msg)
	start := len(b)
	b = slices.Grow(b, protowire.SizeTag(linkRouteID)+protowire.SizeVarint(uint64(routeID))+
		sizeBytesField(linkPayload, size)+sizeVarintField(linkSeq, seq))
	b = appendVarintField(b, linkRouteID, routeID, true)
	if size > 0 {
		b = protowire.AppendTag(b, linkPayload, protowire.BytesType)
		b = protowire.AppendVarint(b, uint64(size))
		at := len(b)
		var err error
		b, err = proto.MarshalOptions{UseCachedSize: true}.MarshalAppend(b, msg)
		if err != nil {
			return b[:start], err
		}
		if len(b)-at != size {
			return b[:start], fmt.Errorf("%w: message changed while encoding", errors.ErrBadArgument)
		}
	}
	return appendVarintField(b, linkSeq, seq, false), nil
}

// AppendLinkLength appends the varint length prefix that delimits a frame of n bytes on a
// byte-stream face (TCP).
func AppendLinkLength(b []byte, n int) []byte {
	return protowire.AppendVarint(b, uint64(n))
}

// DecodeLinkData parses a LinkFrame on the DATA path. A zero RouteID in the result means a
// control frame: decode it with proto.Unmarshal into types/link.LinkFrame. Payload and
// Sealed alias frame. An error wrapping ErrBadFrame is returned for:
//   - malformed frames and data-path fields with the wrong wire type;
//   - frames that mix DATA with a control field;
//   - frames that carry both payload and sealed (AppendLinkData refuses to build them);
//   - frames with more than one control field (the control oneof allows one; generated
//     decoding would silently keep the last).
//
// Unknown fields are skipped, as generated code does.
func DecodeLinkData(frame []byte) (LinkData, error) {
	var d LinkData
	controls := 0
	for len(frame) > 0 {
		num, typ, n := protowire.ConsumeTag(frame)
		if n < 0 {
			return LinkData{}, linkFrameErr(n)
		}
		frame = frame[n:]
		switch num {
		case linkRouteID, linkSeq, linkFrag, linkCtr:
			if typ != protowire.VarintType {
				return LinkData{}, fmt.Errorf("%w: field %d has wire type %d", errors.ErrBadFrame, num, typ)
			}
			v, n := protowire.ConsumeVarint(frame)
			if n < 0 {
				return LinkData{}, linkFrameErr(n)
			}
			frame = frame[n:]
			switch num { // uint32 fields keep the low 32 bits, as generated code does
			case linkRouteID:
				d.RouteID = uint32(v)
			case linkSeq:
				d.Seq = uint32(v)
			case linkFrag:
				d.Frag = uint32(v)
			default:
				d.Ctr = uint32(v)
			}
		case linkPayload, linkSealed:
			if typ != protowire.BytesType {
				return LinkData{}, fmt.Errorf("%w: field %d has wire type %d", errors.ErrBadFrame, num, typ)
			}
			v, n := protowire.ConsumeBytes(frame)
			if n < 0 {
				return LinkData{}, linkFrameErr(n)
			}
			frame = frame[n:]
			if num == linkPayload {
				d.Payload = v
			} else {
				d.Sealed = v
			}
		default:
			if num >= linkFirstControl && num <= linkLastControl {
				controls++
			}
			n := protowire.ConsumeFieldValue(num, typ, frame)
			if n < 0 {
				return LinkData{}, linkFrameErr(n)
			}
			frame = frame[n:]
		}
	}
	if controls > 1 {
		return LinkData{}, fmt.Errorf("%w: %d control fields in one frame", errors.ErrBadFrame, controls)
	}
	if d.RouteID == 0 {
		return LinkData{}, nil
	}
	if controls > 0 {
		return LinkData{}, fmt.Errorf("%w: DATA frame with a control field", errors.ErrBadFrame)
	}
	if len(d.Payload) > 0 && len(d.Sealed) > 0 {
		// Review R-DND-R9 L1: plaintext must never ride next to a sealed blob.
		return LinkData{}, fmt.Errorf("%w: DATA frame with both payload and sealed", errors.ErrBadFrame)
	}
	return d, nil
}

func linkFrameErr(n int) error {
	return fmt.Errorf("%w: %v", errors.ErrBadFrame, protowire.ParseError(n))
}

func sizeVarintField(num protowire.Number, v uint32) int {
	if v == 0 {
		return 0
	}
	return protowire.SizeTag(num) + protowire.SizeVarint(uint64(v))
}

func sizeBytesField(num protowire.Number, n int) int {
	if n == 0 {
		return 0
	}
	return protowire.SizeTag(num) + protowire.SizeBytes(n)
}

func appendVarintField(b []byte, num protowire.Number, v uint32, always bool) []byte {
	if v == 0 && !always {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(v))
}

func appendBytesField(b []byte, num protowire.Number, v []byte) []byte {
	if len(v) == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, v)
}
