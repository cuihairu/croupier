package sdkv1

// Legacy compatibility types.
// These types use manual protobuf encoding via protobuf-go's lightweight
// marshal/unmarshal functions to avoid needing generated proto.Message stubs.

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ---- RegisterLocalRequest ----

type RegisterLocalRequest struct {
	ServiceId string
	Version   string
	RpcAddr   string
	Functions []*LocalFunctionDescriptor
}

func (m *RegisterLocalRequest) Reset()         { *m = RegisterLocalRequest{} }
func (m *RegisterLocalRequest) String() string { return fmt.Sprintf("%+v", m) }
func (m *RegisterLocalRequest) ProtoMessage()  {}

func (m *RegisterLocalRequest) GetServiceId() string {
	if m != nil {
		return m.ServiceId
	}
	return ""
}
func (m *RegisterLocalRequest) GetVersion() string {
	if m != nil {
		return m.Version
	}
	return ""
}
func (m *RegisterLocalRequest) GetRpcAddr() string {
	if m != nil {
		return m.RpcAddr
	}
	return ""
}
func (m *RegisterLocalRequest) GetFunctions() []*LocalFunctionDescriptor {
	if m != nil {
		return m.Functions
	}
	return nil
}

// ---- RegisterLocalResponse ----

type RegisterLocalResponse struct {
	SessionId string
}

func (m *RegisterLocalResponse) Reset()         { *m = RegisterLocalResponse{} }
func (m *RegisterLocalResponse) String() string { return fmt.Sprintf("%+v", m) }
func (m *RegisterLocalResponse) ProtoMessage()  {}

// ---- HeartbeatRequest ----

type HeartbeatRequest struct {
	ServiceId string
	SessionId string
}

func (m *HeartbeatRequest) Reset()         { *m = HeartbeatRequest{} }
func (m *HeartbeatRequest) String() string { return fmt.Sprintf("%+v", m) }
func (m *HeartbeatRequest) ProtoMessage()  {}

// ---- HeartbeatResponse ----

type HeartbeatResponse struct{}

func (m *HeartbeatResponse) Reset()         {}
func (m *HeartbeatResponse) String() string { return "{}" }
func (m *HeartbeatResponse) ProtoMessage()  {}

// ---- ListLocalRequest ----

type ListLocalRequest struct{}

func (m *ListLocalRequest) Reset()         {}
func (m *ListLocalRequest) String() string { return "{}" }
func (m *ListLocalRequest) ProtoMessage()  {}

// ---- ListLocalResponse ----

type ListLocalResponse struct {
	Functions []*LocalFunction
}

func (m *ListLocalResponse) Reset()         { *m = ListLocalResponse{} }
func (m *ListLocalResponse) String() string { return fmt.Sprintf("%+v", m) }
func (m *ListLocalResponse) ProtoMessage()  {}

// ---- LocalFunction ----

type LocalFunction struct {
	Id        string
	Instances []*LocalInstance
}

func (m *LocalFunction) Reset()         { *m = LocalFunction{} }
func (m *LocalFunction) String() string { return fmt.Sprintf("%+v", m) }
func (m *LocalFunction) ProtoMessage()  {}

// ---- LocalInstance ----

type LocalInstance struct {
	ServiceId string
	Addr      string
	Version   string
}

func (m *LocalInstance) Reset()         { *m = LocalInstance{} }
func (m *LocalInstance) String() string { return fmt.Sprintf("%+v", m) }
func (m *LocalInstance) ProtoMessage()  {}

// ---- Manual protobuf Marshal/Unmarshal ----
// We use protowire to manually encode/decode these types since they
// are not generated protobuf messages but need to be wire-compatible.

// MarshalRegisterLocalRequest manually encodes RegisterLocalRequest as protobuf.
func MarshalRegisterLocalRequest(m *RegisterLocalRequest) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, m.ServiceId)
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, m.Version)
	b = protowire.AppendTag(b, 3, protowire.BytesType)
	b = protowire.AppendString(b, m.RpcAddr)
	for _, f := range m.Functions {
		fb := marshalLocalFunctionDescriptorCompat(f)
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendBytes(b, fb)
	}
	return b
}

// UnmarshalRegisterLocalRequest manually decodes RegisterLocalRequest from protobuf.
func UnmarshalRegisterLocalRequest(data []byte) *RegisterLocalRequest {
	m := &RegisterLocalRequest{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		switch num {
		case 1:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n < 0 {
					break
				}
				m.ServiceId = s
				data = data[n:]
			}
		case 2:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n < 0 {
					break
				}
				m.Version = s
				data = data[n:]
			}
		case 3:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n < 0 {
					break
				}
				m.RpcAddr = s
				data = data[n:]
			}
		case 4:
			if typ == protowire.BytesType {
				raw, n := protowire.ConsumeBytes(data)
				if n < 0 {
					break
				}
				m.Functions = append(m.Functions, unmarshalLocalFunctionDescriptorCompat(raw))
				data = data[n:]
			}
		default:
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 {
				break
			}
			data = data[n:]
		}
	}
	return m
}

// MarshalRegisterLocalResponse manually encodes RegisterLocalResponse.
func MarshalRegisterLocalResponse(m *RegisterLocalResponse) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, m.SessionId)
	return b
}

// UnmarshalRegisterLocalResponse manually decodes RegisterLocalResponse.
func UnmarshalRegisterLocalResponse(data []byte) *RegisterLocalResponse {
	m := &RegisterLocalResponse{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		if num == 1 && typ == protowire.BytesType {
			s, n := protowire.ConsumeString(data)
			if n >= 0 {
				m.SessionId = s
				data = data[n:]
			}
		} else {
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 {
				break
			}
			data = data[n:]
		}
	}
	return m
}

// MarshalHeartbeatRequestCompat manually encodes HeartbeatRequest.
func MarshalHeartbeatRequestCompat(m *HeartbeatRequest) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, m.ServiceId)
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, m.SessionId)
	return b
}

// UnmarshalHeartbeatRequestCompat manually decodes HeartbeatRequest.
func UnmarshalHeartbeatRequestCompat(data []byte) *HeartbeatRequest {
	m := &HeartbeatRequest{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		switch num {
		case 1:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.ServiceId = s
					data = data[n:]
				}
			}
		case 2:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.SessionId = s
					data = data[n:]
				}
			}
		default:
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 {
				break
			}
			data = data[n:]
		}
	}
	return m
}

// MarshalListLocalResponse manually encodes ListLocalResponse.
func MarshalListLocalResponse(m *ListLocalResponse) []byte {
	var b []byte
	for _, f := range m.Functions {
		fb := marshalLocalFunctionCompat(f)
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, fb)
	}
	return b
}

// UnmarshalListLocalResponse manually decodes ListLocalResponse from protobuf.
func UnmarshalListLocalResponse(data []byte) *ListLocalResponse {
	m := &ListLocalResponse{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		switch num {
		case 1:
			if typ == protowire.BytesType {
				fb, fn := protowire.ConsumeBytes(data)
				if fn >= 0 {
					m.Functions = append(m.Functions, unmarshalLocalFunctionCompat(fb))
					data = data[fn:]
				}
			}
		default:
			fn := protowire.ConsumeFieldValue(num, typ, data)
			if fn < 0 {
				break
			}
			data = data[fn:]
		}
	}
	return m
}

func unmarshalLocalFunctionCompat(data []byte) *LocalFunction {
	m := &LocalFunction{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		switch num {
		case 1:
			if typ == protowire.BytesType {
				s, sn := protowire.ConsumeString(data)
				if sn >= 0 {
					m.Id = s
					data = data[sn:]
				}
			}
		case 2:
			if typ == protowire.BytesType {
				ib, in := protowire.ConsumeBytes(data)
				if in >= 0 {
					m.Instances = append(m.Instances, unmarshalLocalInstanceCompat(ib))
					data = data[in:]
				}
			}
		default:
			fn := protowire.ConsumeFieldValue(num, typ, data)
			if fn < 0 {
				break
			}
			data = data[fn:]
		}
	}
	return m
}

func unmarshalLocalInstanceCompat(data []byte) *LocalInstance {
	m := &LocalInstance{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		switch num {
		case 1:
			if typ == protowire.BytesType {
				s, sn := protowire.ConsumeString(data)
				if sn >= 0 {
					m.ServiceId = s
					data = data[sn:]
				}
			}
		case 2:
			if typ == protowire.BytesType {
				s, sn := protowire.ConsumeString(data)
				if sn >= 0 {
					m.Addr = s
					data = data[sn:]
				}
			}
		case 3:
			if typ == protowire.BytesType {
				s, sn := protowire.ConsumeString(data)
				if sn >= 0 {
					m.Version = s
					data = data[sn:]
				}
			}
		default:
			fn := protowire.ConsumeFieldValue(num, typ, data)
			if fn < 0 {
				break
			}
			data = data[fn:]
		}
	}
	return m
}

func marshalLocalFunctionCompat(m *LocalFunction) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, m.Id)
	for _, inst := range m.Instances {
		ib := marshalLocalInstanceCompat(inst)
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, ib)
	}
	return b
}

func marshalLocalInstanceCompat(m *LocalInstance) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, m.ServiceId)
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, m.Addr)
	b = protowire.AppendTag(b, 3, protowire.BytesType)
	b = protowire.AppendString(b, m.Version)
	return b
}

// marshalLocalFunctionDescriptorCompat encodes a LocalFunctionDescriptor using the
// same wire format as the generated protobuf type (tags 1-13).
func marshalLocalFunctionDescriptorCompat(m *LocalFunctionDescriptor) []byte {
	var b []byte
	if m.Id != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, m.Id)
	}
	if m.Version != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, m.Version)
	}
	for _, t := range m.Tags {
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendString(b, t)
	}
	if m.Summary != "" {
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendString(b, m.Summary)
	}
	if m.Description != "" {
		b = protowire.AppendTag(b, 5, protowire.BytesType)
		b = protowire.AppendString(b, m.Description)
	}
	if m.OperationId != "" {
		b = protowire.AppendTag(b, 6, protowire.BytesType)
		b = protowire.AppendString(b, m.OperationId)
	}
	if m.Deprecated {
		b = protowire.AppendTag(b, 7, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	if m.InputSchema != "" {
		b = protowire.AppendTag(b, 8, protowire.BytesType)
		b = protowire.AppendString(b, m.InputSchema)
	}
	if m.OutputSchema != "" {
		b = protowire.AppendTag(b, 9, protowire.BytesType)
		b = protowire.AppendString(b, m.OutputSchema)
	}
	if m.Category != "" {
		b = protowire.AppendTag(b, 10, protowire.BytesType)
		b = protowire.AppendString(b, m.Category)
	}
	if m.Risk != "" {
		b = protowire.AppendTag(b, 11, protowire.BytesType)
		b = protowire.AppendString(b, m.Risk)
	}
	if m.Entity != "" {
		b = protowire.AppendTag(b, 12, protowire.BytesType)
		b = protowire.AppendString(b, m.Entity)
	}
	if m.Operation != "" {
		b = protowire.AppendTag(b, 13, protowire.BytesType)
		b = protowire.AppendString(b, m.Operation)
	}
	return b
}

// unmarshalLocalFunctionDescriptorCompat decodes a LocalFunctionDescriptor from raw bytes.
func unmarshalLocalFunctionDescriptorCompat(data []byte) *LocalFunctionDescriptor {
	m := &LocalFunctionDescriptor{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		switch num {
		case 1:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.Id = s
					data = data[n:]
				}
			}
		case 2:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.Version = s
					data = data[n:]
				}
			}
		case 3:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.Tags = append(m.Tags, s)
					data = data[n:]
				}
			}
		case 4:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.Summary = s
					data = data[n:]
				}
			}
		case 5:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.Description = s
					data = data[n:]
				}
			}
		case 6:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.OperationId = s
					data = data[n:]
				}
			}
		case 7:
			if typ == protowire.VarintType {
				v, n := protowire.ConsumeVarint(data)
				if n >= 0 {
					m.Deprecated = v != 0
					data = data[n:]
				}
			}
		case 8:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.InputSchema = s
					data = data[n:]
				}
			}
		case 9:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.OutputSchema = s
					data = data[n:]
				}
			}
		case 10:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.Category = s
					data = data[n:]
				}
			}
		case 11:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.Risk = s
					data = data[n:]
				}
			}
		case 12:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.Entity = s
					data = data[n:]
				}
			}
		case 13:
			if typ == protowire.BytesType {
				s, n := protowire.ConsumeString(data)
				if n >= 0 {
					m.Operation = s
					data = data[n:]
				}
			}
		default:
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 {
				break
			}
			data = data[n:]
		}
	}
	return m
}

// Suppress unused import warning
var _ protoreflect.Message = nil
