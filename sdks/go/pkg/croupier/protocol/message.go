// Package protocol implements the Croupier wire protocol.
//
// Message Format:
//
//	Header (8 bytes):
//	  ┌─────────┬──────────┬─────────────────┐
//	  │ Version │ MsgID    │ RequestID       │
//	  │ (1B)    │ (3B)     │ (4B)            │
//	  └─────────┴──────────┴─────────────────┘
//
//	Body: protobuf serialized message
//
// Request messages have odd MsgID, Response messages have even MsgID.
// The RequestID is used to match responses to their requests.
package protocol

import (
	"encoding/binary"
	"fmt"
)

const (
	// Version1 is the current protocol version.
	Version1 = 0x01

	// HeaderSize is the fixed size of the message header in bytes.
	HeaderSize = 8 // Version(1) + MsgID(3) + RequestID(4)
)

// Message type constants (24 bits).
const (
	// ControlService (0x01xx)
	MsgRegisterRequest          = 0x010101
	MsgRegisterResponse         = 0x010102
	MsgHeartbeatRequest         = 0x010103
	MsgHeartbeatResponse        = 0x010104
	MsgRegisterCapabilitiesReq  = 0x010105
	MsgRegisterCapabilitiesResp = 0x010106

	// ClientService (0x02xx)
	MsgRegisterClientRequest   = 0x020101
	MsgRegisterClientResponse  = 0x020102
	MsgClientHeartbeatRequest  = 0x020103
	MsgClientHeartbeatResponse = 0x020104
	MsgListClientsRequest      = 0x020105
	MsgListClientsResponse     = 0x020106
	MsgGetJobResultRequest     = 0x020107
	MsgGetJobResultResponse    = 0x020108

	// InvokerService (0x03xx)
	MsgInvokeRequest      = 0x030101
	MsgInvokeResponse     = 0x030102
	MsgStartTaskRequest   = 0x030103
	MsgStartTaskResponse  = 0x030104
	MsgStreamTaskRequest  = 0x030105
	MsgTaskEvent          = 0x030106 // Stream event (not request/response)
	MsgCancelTaskRequest  = 0x030107
	MsgCancelTaskResponse = 0x030108

	// ProviderService (0x05xx) - Provider session protocol
	MsgProviderConnectRequest    = 0x050101
	MsgProviderConnectResponse   = 0x050102
	MsgProviderHeartbeatRequest  = 0x050103
	MsgProviderHeartbeatResponse = 0x050104
	MsgProviderDrainRequest      = 0x050105
	MsgProviderDrainResponse     = 0x050106
	MsgGetTaskResultRequest      = 0x050107
	MsgGetTaskResultResponse     = 0x050108
	MsgListLocalRequest          = 0x050109
	MsgListLocalResponse         = 0x05010a

	// Legacy aliases for backward compatibility
	MsgRegisterLocalRequest   = MsgProviderConnectRequest
	MsgRegisterLocalResponse  = MsgProviderConnectResponse
	MsgHeartbeatLocalRequest  = MsgProviderHeartbeatRequest
	MsgHeartbeatLocalResponse = MsgProviderHeartbeatResponse

	// Legacy Job aliases for backward compatibility
	MsgStartJobRequest   = MsgStartTaskRequest
	MsgStartJobResponse  = MsgStartTaskResponse
	MsgStreamJobRequest  = MsgStreamTaskRequest
	MsgJobEvent          = MsgTaskEvent
	MsgCancelJobRequest  = MsgCancelTaskRequest
	MsgCancelJobResponse = MsgCancelTaskResponse
)

// PutMsgID encodes a 24-bit MsgID into buf in big-endian order.
func PutMsgID(buf []byte, msgID uint32) {
	buf[0] = byte(msgID >> 16)
	buf[1] = byte(msgID >> 8)
	buf[2] = byte(msgID)
}

// GetMsgID decodes a 24-bit MsgID from buf in big-endian order.
func GetMsgID(buf []byte) uint32 {
	return uint32(buf[0])<<16 | uint32(buf[1])<<8 | uint32(buf[2])
}

// NewMessageBody creates a body with protocol header prefix and data.
// Body format: [8-byte header][actual data]
func NewMessageBody(msgID uint32, reqID uint32, data []byte) []byte {
	body := make([]byte, HeaderSize+len(data))
	body[0] = Version1
	PutMsgID(body[1:4], msgID)
	binary.BigEndian.PutUint32(body[4:8], reqID)
	copy(body[HeaderSize:], data)
	return body
}

// ParseMessageFromBody parses a message from a body that contains the protocol header as a prefix.
// Body format: [8-byte header][actual data]
func ParseMessageFromBody(body []byte) (version uint8, msgID uint32, reqID uint32, data []byte, err error) {
	if len(body) < HeaderSize {
		err = fmt.Errorf("body too short for header: %d < %d", len(body), HeaderSize)
		return
	}

	version = body[0]
	msgID = GetMsgID(body[1:4])
	reqID = binary.BigEndian.Uint32(body[4:8])
	data = body[HeaderSize:]

	return
}

// IsRequest returns true if the MsgID indicates a request message.
func IsRequest(msgID uint32) bool {
	return msgID%2 == 1 && msgID != MsgTaskEvent
}

// IsResponse returns true if the MsgID indicates a response message.
func IsResponse(msgID uint32) bool {
	return msgID%2 == 0 && msgID != MsgTaskEvent // TaskEvent is a stream event, not a response
}

// GetResponseMsgID returns the response MsgID for a given request MsgID.
func GetResponseMsgID(reqMsgID uint32) uint32 {
	return reqMsgID + 1
}

// MsgIDString returns a human-readable string representation of the MsgID.
func MsgIDString(msgID uint32) string {
	switch msgID {
	case MsgRegisterRequest:
		return "RegisterRequest"
	case MsgRegisterResponse:
		return "RegisterResponse"
	case MsgHeartbeatRequest:
		return "HeartbeatRequest"
	case MsgHeartbeatResponse:
		return "HeartbeatResponse"
	case MsgRegisterCapabilitiesReq:
		return "RegisterCapabilitiesRequest"
	case MsgRegisterCapabilitiesResp:
		return "RegisterCapabilitiesResponse"
	case MsgRegisterClientRequest:
		return "RegisterClientRequest"
	case MsgRegisterClientResponse:
		return "RegisterClientResponse"
	case MsgClientHeartbeatRequest:
		return "ClientHeartbeatRequest"
	case MsgClientHeartbeatResponse:
		return "ClientHeartbeatResponse"
	case MsgListClientsRequest:
		return "ListClientsRequest"
	case MsgListClientsResponse:
		return "ListClientsResponse"
	case MsgGetJobResultRequest:
		return "GetJobResultRequest"
	case MsgGetJobResultResponse:
		return "GetJobResultResponse"
	case MsgInvokeRequest:
		return "InvokeRequest"
	case MsgInvokeResponse:
		return "InvokeResponse"
	case MsgStartTaskRequest:
		return "StartTaskRequest"
	case MsgStartTaskResponse:
		return "StartTaskResponse"
	case MsgStreamTaskRequest:
		return "StreamTaskRequest"
	case MsgTaskEvent:
		return "TaskEvent"
	case MsgCancelTaskRequest:
		return "CancelTaskRequest"
	case MsgCancelTaskResponse:
		return "CancelTaskResponse"
	case MsgProviderConnectRequest:
		return "ProviderConnectRequest"
	case MsgProviderConnectResponse:
		return "ProviderConnectResponse"
	case MsgProviderHeartbeatRequest:
		return "ProviderHeartbeatRequest"
	case MsgProviderHeartbeatResponse:
		return "ProviderHeartbeatResponse"
	case MsgProviderDrainRequest:
		return "ProviderDrainRequest"
	case MsgProviderDrainResponse:
		return "ProviderDrainResponse"
	case MsgGetTaskResultRequest:
		return "GetTaskResultRequest"
	case MsgGetTaskResultResponse:
		return "GetTaskResultResponse"
	case MsgListLocalRequest:
		return "ListLocalRequest"
	case MsgListLocalResponse:
		return "ListLocalResponse"
	default:
		return fmt.Sprintf("Unknown(0x%06X)", msgID)
	}
}
