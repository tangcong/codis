// Code generated by protoc-gen-go.
// source: api.proto
// DO NOT EDIT!

/*
Package backup is a generated protocol buffer package.

It is generated from these files:
	api.proto

It has these top-level messages:
	WriteRequest
	WriteResponse
*/
package backup

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type WriteResponse_WritingStatus int32

const (
	WriteResponse_UNKNOWN WriteResponse_WritingStatus = 0
	WriteResponse_OK      WriteResponse_WritingStatus = 1
	WriteResponse_FAIL    WriteResponse_WritingStatus = 2
)

var WriteResponse_WritingStatus_name = map[int32]string{
	0: "UNKNOWN",
	1: "OK",
	2: "FAIL",
}
var WriteResponse_WritingStatus_value = map[string]int32{
	"UNKNOWN": 0,
	"OK":      1,
	"FAIL":    2,
}

func (x WriteResponse_WritingStatus) Enum() *WriteResponse_WritingStatus {
	p := new(WriteResponse_WritingStatus)
	*p = x
	return p
}
func (x WriteResponse_WritingStatus) String() string {
	return proto.EnumName(WriteResponse_WritingStatus_name, int32(x))
}
func (x *WriteResponse_WritingStatus) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(WriteResponse_WritingStatus_value, data, "WriteResponse_WritingStatus")
	if err != nil {
		return err
	}
	*x = WriteResponse_WritingStatus(value)
	return nil
}
func (WriteResponse_WritingStatus) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor0, []int{1, 0}
}

type WriteRequest struct {
	ProductName      *string `protobuf:"bytes,1,req,name=productName" json:"productName,omitempty"`
	MultiCmd         []byte  `protobuf:"bytes,2,req,name=multiCmd" json:"multiCmd,omitempty"`
	UpdateTime       *int64  `protobuf:"varint,3,opt,name=updateTime" json:"updateTime,omitempty"`
	MasterOffset     *uint64 `protobuf:"varint,4,opt,name=masterOffset" json:"masterOffset,omitempty"`
	SlaveOffset      *uint64 `protobuf:"varint,5,opt,name=slaveOffset" json:"slaveOffset,omitempty"`
	UsedIndex        *int32  `protobuf:"varint,6,opt,name=usedIndex" json:"usedIndex,omitempty"`
	Cmds             *int32  `protobuf:"varint,7,opt,name=cmds" json:"cmds,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *WriteRequest) Reset()                    { *m = WriteRequest{} }
func (m *WriteRequest) String() string            { return proto.CompactTextString(m) }
func (*WriteRequest) ProtoMessage()               {}
func (*WriteRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *WriteRequest) GetProductName() string {
	if m != nil && m.ProductName != nil {
		return *m.ProductName
	}
	return ""
}

func (m *WriteRequest) GetMultiCmd() []byte {
	if m != nil {
		return m.MultiCmd
	}
	return nil
}

func (m *WriteRequest) GetUpdateTime() int64 {
	if m != nil && m.UpdateTime != nil {
		return *m.UpdateTime
	}
	return 0
}

func (m *WriteRequest) GetMasterOffset() uint64 {
	if m != nil && m.MasterOffset != nil {
		return *m.MasterOffset
	}
	return 0
}

func (m *WriteRequest) GetSlaveOffset() uint64 {
	if m != nil && m.SlaveOffset != nil {
		return *m.SlaveOffset
	}
	return 0
}

func (m *WriteRequest) GetUsedIndex() int32 {
	if m != nil && m.UsedIndex != nil {
		return *m.UsedIndex
	}
	return 0
}

func (m *WriteRequest) GetCmds() int32 {
	if m != nil && m.Cmds != nil {
		return *m.Cmds
	}
	return 0
}

type WriteResponse struct {
	Status           *WriteResponse_WritingStatus `protobuf:"varint,1,req,name=status,enum=backup.WriteResponse_WritingStatus" json:"status,omitempty"`
	Cmds             *int32                       `protobuf:"varint,2,req,name=cmds" json:"cmds,omitempty"`
	XXX_unrecognized []byte                       `json:"-"`
}

func (m *WriteResponse) Reset()                    { *m = WriteResponse{} }
func (m *WriteResponse) String() string            { return proto.CompactTextString(m) }
func (*WriteResponse) ProtoMessage()               {}
func (*WriteResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *WriteResponse) GetStatus() WriteResponse_WritingStatus {
	if m != nil && m.Status != nil {
		return *m.Status
	}
	return WriteResponse_UNKNOWN
}

func (m *WriteResponse) GetCmds() int32 {
	if m != nil && m.Cmds != nil {
		return *m.Cmds
	}
	return 0
}

func init() {
	proto.RegisterType((*WriteRequest)(nil), "backup.WriteRequest")
	proto.RegisterType((*WriteResponse)(nil), "backup.WriteResponse")
	proto.RegisterEnum("backup.WriteResponse_WritingStatus", WriteResponse_WritingStatus_name, WriteResponse_WritingStatus_value)
}

func init() { proto.RegisterFile("api.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 254 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x54, 0x8f, 0xcd, 0x4a, 0xc3, 0x40,
	0x14, 0x85, 0x9d, 0x34, 0x49, 0x9b, 0xdb, 0x28, 0x71, 0x74, 0x31, 0xcb, 0x10, 0x37, 0x59, 0x65,
	0xa1, 0x4f, 0x20, 0x82, 0x50, 0x2a, 0x09, 0xf8, 0x43, 0xd7, 0x63, 0xe6, 0x56, 0x06, 0x3b, 0xc9,
	0x98, 0xb9, 0x23, 0xae, 0x7d, 0x07, 0xdf, 0x57, 0x92, 0x16, 0xa5, 0xcb, 0x73, 0xee, 0xe1, 0xe3,
	0xbb, 0x90, 0x48, 0xab, 0x2b, 0x3b, 0xf4, 0xd4, 0xf3, 0xf8, 0x55, 0xb6, 0xef, 0xde, 0x16, 0x3f,
	0x0c, 0xd2, 0xcd, 0xa0, 0x09, 0x1f, 0xf1, 0xc3, 0xa3, 0x23, 0x7e, 0x01, 0x4b, 0x3b, 0xf4, 0xca,
	0xb7, 0x54, 0x4b, 0x83, 0x82, 0xe5, 0x41, 0x99, 0xf0, 0x0c, 0x16, 0xc6, 0xef, 0x48, 0xdf, 0x19,
	0x25, 0x82, 0x3c, 0x28, 0x53, 0xce, 0x01, 0xbc, 0x55, 0x92, 0xf0, 0x59, 0x1b, 0x14, 0xb3, 0x9c,
	0x95, 0x33, 0x7e, 0x09, 0xa9, 0x91, 0x8e, 0x70, 0x68, 0xb6, 0x5b, 0x87, 0x24, 0xc2, 0x9c, 0x95,
	0xe1, 0x08, 0x74, 0x3b, 0xf9, 0x89, 0x87, 0x32, 0x9a, 0xca, 0x73, 0x48, 0xbc, 0x43, 0xb5, 0xea,
	0x14, 0x7e, 0x89, 0x38, 0x67, 0x65, 0xc4, 0x53, 0x08, 0x5b, 0xa3, 0x9c, 0x98, 0x8f, 0xa9, 0xf8,
	0x66, 0x70, 0x7a, 0xf0, 0x72, 0xb6, 0xef, 0x1c, 0xf2, 0x1b, 0x88, 0x1d, 0x49, 0xf2, 0x6e, 0x72,
	0x3a, 0xbb, 0xbe, 0xaa, 0xf6, 0x2f, 0x54, 0x47, 0xb3, 0x29, 0xe9, 0xee, 0xed, 0x69, 0x9a, 0xfe,
	0x41, 0x47, 0xe9, 0xa8, 0xa8, 0xf6, 0xcc, 0xff, 0xf3, 0x12, 0xe6, 0x2f, 0xf5, 0xba, 0x6e, 0x36,
	0x75, 0x76, 0xc2, 0x63, 0x08, 0x9a, 0x75, 0xc6, 0xf8, 0x02, 0xc2, 0xfb, 0xdb, 0xd5, 0x43, 0x16,
	0xfc, 0x06, 0x00, 0x00, 0xff, 0xff, 0x84, 0x63, 0xfa, 0x37, 0x30, 0x01, 0x00, 0x00,
}
