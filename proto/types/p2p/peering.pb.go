// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        (unknown)
// source: proto/types/p2p/peering.proto

package types

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type HandshakeStage int32

const (
	HandshakeStage_UNSPECIFIED_HANDSHAKE HandshakeStage = 0
	HandshakeStage_INITIAL               HandshakeStage = 1
	HandshakeStage_FINAL                 HandshakeStage = 2
)

// Enum value maps for HandshakeStage.
var (
	HandshakeStage_name = map[int32]string{
		0: "UNSPECIFIED_HANDSHAKE",
		1: "INITIAL",
		2: "FINAL",
	}
	HandshakeStage_value = map[string]int32{
		"UNSPECIFIED_HANDSHAKE": 0,
		"INITIAL":               1,
		"FINAL":                 2,
	}
)

func (x HandshakeStage) Enum() *HandshakeStage {
	p := new(HandshakeStage)
	*p = x
	return p
}

func (x HandshakeStage) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (HandshakeStage) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_types_p2p_peering_proto_enumTypes[0].Descriptor()
}

func (HandshakeStage) Type() protoreflect.EnumType {
	return &file_proto_types_p2p_peering_proto_enumTypes[0]
}

func (x HandshakeStage) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use HandshakeStage.Descriptor instead.
func (HandshakeStage) EnumDescriptor() ([]byte, []int) {
	return file_proto_types_p2p_peering_proto_rawDescGZIP(), []int{0}
}

type Handshake struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id        string         `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Stage     HandshakeStage `protobuf:"varint,2,opt,name=stage,proto3,enum=types.p2p.HandshakeStage" json:"stage,omitempty"`
	PublicKey []byte         `protobuf:"bytes,3,opt,name=public_key,json=publicKey,proto3" json:"public_key,omitempty"`
}

func (x *Handshake) Reset() {
	*x = Handshake{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_types_p2p_peering_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Handshake) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Handshake) ProtoMessage() {}

func (x *Handshake) ProtoReflect() protoreflect.Message {
	mi := &file_proto_types_p2p_peering_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Handshake.ProtoReflect.Descriptor instead.
func (*Handshake) Descriptor() ([]byte, []int) {
	return file_proto_types_p2p_peering_proto_rawDescGZIP(), []int{0}
}

func (x *Handshake) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Handshake) GetStage() HandshakeStage {
	if x != nil {
		return x.Stage
	}
	return HandshakeStage_UNSPECIFIED_HANDSHAKE
}

func (x *Handshake) GetPublicKey() []byte {
	if x != nil {
		return x.PublicKey
	}
	return nil
}

type Peers struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ids    []string `protobuf:"bytes,1,rep,name=ids,proto3" json:"ids,omitempty"`
	Remove bool     `protobuf:"varint,2,opt,name=remove,proto3" json:"remove,omitempty"`
}

func (x *Peers) Reset() {
	*x = Peers{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_types_p2p_peering_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Peers) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Peers) ProtoMessage() {}

func (x *Peers) ProtoReflect() protoreflect.Message {
	mi := &file_proto_types_p2p_peering_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Peers.ProtoReflect.Descriptor instead.
func (*Peers) Descriptor() ([]byte, []int) {
	return file_proto_types_p2p_peering_proto_rawDescGZIP(), []int{1}
}

func (x *Peers) GetIds() []string {
	if x != nil {
		return x.Ids
	}
	return nil
}

func (x *Peers) GetRemove() bool {
	if x != nil {
		return x.Remove
	}
	return false
}

var File_proto_types_p2p_peering_proto protoreflect.FileDescriptor

var file_proto_types_p2p_peering_proto_rawDesc = []byte{
	0x0a, 0x1d, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2f, 0x70, 0x32,
	0x70, 0x2f, 0x70, 0x65, 0x65, 0x72, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x09, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70, 0x32, 0x70, 0x22, 0x6b, 0x0a, 0x09, 0x48, 0x61,
	0x6e, 0x64, 0x73, 0x68, 0x61, 0x6b, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x2f, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x67, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x19, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70,
	0x32, 0x70, 0x2e, 0x48, 0x61, 0x6e, 0x64, 0x73, 0x68, 0x61, 0x6b, 0x65, 0x53, 0x74, 0x61, 0x67,
	0x65, 0x52, 0x05, 0x73, 0x74, 0x61, 0x67, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x70, 0x75, 0x62, 0x6c,
	0x69, 0x63, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09, 0x70, 0x75,
	0x62, 0x6c, 0x69, 0x63, 0x4b, 0x65, 0x79, 0x22, 0x31, 0x0a, 0x05, 0x50, 0x65, 0x65, 0x72, 0x73,
	0x12, 0x10, 0x0a, 0x03, 0x69, 0x64, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x03, 0x69,
	0x64, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x72, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x06, 0x72, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x2a, 0x43, 0x0a, 0x0e, 0x48, 0x61,
	0x6e, 0x64, 0x73, 0x68, 0x61, 0x6b, 0x65, 0x53, 0x74, 0x61, 0x67, 0x65, 0x12, 0x19, 0x0a, 0x15,
	0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x5f, 0x48, 0x41, 0x4e, 0x44,
	0x53, 0x48, 0x41, 0x4b, 0x45, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x49, 0x4e, 0x49, 0x54, 0x49,
	0x41, 0x4c, 0x10, 0x01, 0x12, 0x09, 0x0a, 0x05, 0x46, 0x49, 0x4e, 0x41, 0x4c, 0x10, 0x02, 0x42,
	0x8a, 0x01, 0x0a, 0x0d, 0x63, 0x6f, 0x6d, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70, 0x32,
	0x70, 0x42, 0x0c, 0x50, 0x65, 0x65, 0x72, 0x69, 0x6e, 0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50,
	0x01, 0x5a, 0x26, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x69, 0x74,
	0x6f, 0x68, 0x69, 0x6f, 0x2f, 0x64, 0x6e, 0x64, 0x6d, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2f,
	0x70, 0x32, 0x70, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0xa2, 0x02, 0x03, 0x54, 0x50, 0x58, 0xaa,
	0x02, 0x09, 0x54, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x50, 0x32, 0x70, 0xca, 0x02, 0x09, 0x54, 0x79,
	0x70, 0x65, 0x73, 0x5c, 0x50, 0x32, 0x70, 0xe2, 0x02, 0x15, 0x54, 0x79, 0x70, 0x65, 0x73, 0x5c,
	0x50, 0x32, 0x70, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea,
	0x02, 0x0a, 0x54, 0x79, 0x70, 0x65, 0x73, 0x3a, 0x3a, 0x50, 0x32, 0x70, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_types_p2p_peering_proto_rawDescOnce sync.Once
	file_proto_types_p2p_peering_proto_rawDescData = file_proto_types_p2p_peering_proto_rawDesc
)

func file_proto_types_p2p_peering_proto_rawDescGZIP() []byte {
	file_proto_types_p2p_peering_proto_rawDescOnce.Do(func() {
		file_proto_types_p2p_peering_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_types_p2p_peering_proto_rawDescData)
	})
	return file_proto_types_p2p_peering_proto_rawDescData
}

var file_proto_types_p2p_peering_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_proto_types_p2p_peering_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_proto_types_p2p_peering_proto_goTypes = []interface{}{
	(HandshakeStage)(0), // 0: types.p2p.HandshakeStage
	(*Handshake)(nil),   // 1: types.p2p.Handshake
	(*Peers)(nil),       // 2: types.p2p.Peers
}
var file_proto_types_p2p_peering_proto_depIdxs = []int32{
	0, // 0: types.p2p.Handshake.stage:type_name -> types.p2p.HandshakeStage
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_proto_types_p2p_peering_proto_init() }
func file_proto_types_p2p_peering_proto_init() {
	if File_proto_types_p2p_peering_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_types_p2p_peering_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Handshake); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_types_p2p_peering_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Peers); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_types_p2p_peering_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proto_types_p2p_peering_proto_goTypes,
		DependencyIndexes: file_proto_types_p2p_peering_proto_depIdxs,
		EnumInfos:         file_proto_types_p2p_peering_proto_enumTypes,
		MessageInfos:      file_proto_types_p2p_peering_proto_msgTypes,
	}.Build()
	File_proto_types_p2p_peering_proto = out.File
	file_proto_types_p2p_peering_proto_rawDesc = nil
	file_proto_types_p2p_peering_proto_goTypes = nil
	file_proto_types_p2p_peering_proto_depIdxs = nil
}