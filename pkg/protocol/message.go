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
	"math"
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
	MsgGetTaskResultRequest    = 0x020107
	MsgGetTaskResultResponse   = 0x020108

	// InvokerService (0x03xx)
	MsgInvokeRequest      = 0x030101
	MsgInvokeResponse     = 0x030102
	MsgStartTaskRequest   = 0x030103
	MsgStartTaskResponse  = 0x030104
	MsgStreamTaskRequest  = 0x030105
	MsgTaskEvent          = 0x030106 // Stream event (not request/response)
	MsgCancelTaskRequest  = 0x030107
	MsgCancelTaskResponse = 0x030108

	// OpsService (0x04xx)
	MsgGetSystemInfoRequest   = 0x040101
	MsgGetSystemInfoResponse  = 0x040102
	MsgListProcessesRequest   = 0x040103
	MsgListProcessesResponse  = 0x040104
	MsgReportMetricsRequest   = 0x040105
	MsgReportMetricsResponse  = 0x040106
	MsgStreamMetricsRequest   = 0x040107
	MsgMetricEvent            = 0x040108 // Stream event
	MsgRestartProcessRequest  = 0x040109
	MsgRestartProcessResponse = 0x04010A
	MsgStopProcessRequest     = 0x04010B
	MsgStopProcessResponse    = 0x04010C
	MsgStartProcessRequest    = 0x04010D
	MsgStartProcessResponse   = 0x04010E
	MsgExecuteCommandRequest  = 0x04010F
	MsgExecuteCommandResponse = 0x040110
	// System services (platform-specific)
	MsgListServicesRequest      = 0x040111
	MsgListServicesResponse     = 0x040112
	MsgGetServiceStatusRequest  = 0x040113
	MsgGetServiceStatusResponse = 0x040114

	// ProviderSessionService (0x05xx) - SDK <-> Agent provider session control
	MsgProviderConnectRequest    = 0x050101
	MsgProviderConnectResponse   = 0x050102
	MsgProviderHeartbeatRequest  = 0x050103
	MsgProviderHeartbeatResponse = 0x050104
	MsgProviderDrainRequest      = 0x050105
	MsgProviderDrainResponse     = 0x050106
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

// IsRequest returns true if the MsgID indicates a request message.
func IsRequest(msgID uint32) bool {
	return msgID%2 == 1 && msgID != MsgTaskEvent && msgID != MsgMetricEvent
}

// IsResponse returns true if the MsgID indicates a response message.
func IsResponse(msgID uint32) bool {
	return msgID%2 == 0 && msgID != MsgTaskEvent && msgID != MsgMetricEvent // TaskEvent and MetricEvent are stream events, not responses
}

// IsEvent returns true if the MsgID is a one-way event frame.
func IsEvent(msgID uint32) bool {
	return msgID == MsgTaskEvent || msgID == MsgMetricEvent
}

// GetResponseMsgID returns the response MsgID for a given request MsgID.
func GetResponseMsgID(reqMsgID uint32) uint32 {
	return reqMsgID + 1
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

// NewMessageBody creates a body with protocol header prefix and data.
func NewMessageBody(msgID uint32, reqID uint32, data []byte) []byte {
	totalLen := HeaderSize + len(data)
	if totalLen < HeaderSize || totalLen < len(data) || totalLen > math.MaxInt {
		// 溢出，返回 nil
		return nil
	}
	body := make([]byte, totalLen)
	body[0] = Version1
	PutMsgID(body[1:4], msgID)
	binary.BigEndian.PutUint32(body[4:8], reqID)
	copy(body[HeaderSize:], data)
	return body
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
	case MsgGetTaskResultRequest:
		return "GetTaskResultRequest"
	case MsgGetTaskResultResponse:
		return "GetTaskResultResponse"
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
	case MsgGetSystemInfoRequest:
		return "GetSystemInfoRequest"
	case MsgGetSystemInfoResponse:
		return "GetSystemInfoResponse"
	case MsgListProcessesRequest:
		return "ListProcessesRequest"
	case MsgListProcessesResponse:
		return "ListProcessesResponse"
	case MsgReportMetricsRequest:
		return "ReportMetricsRequest"
	case MsgReportMetricsResponse:
		return "ReportMetricsResponse"
	case MsgStreamMetricsRequest:
		return "StreamMetricsRequest"
	case MsgMetricEvent:
		return "MetricEvent"
	case MsgRestartProcessRequest:
		return "RestartProcessRequest"
	case MsgRestartProcessResponse:
		return "RestartProcessResponse"
	case MsgStopProcessRequest:
		return "StopProcessRequest"
	case MsgStopProcessResponse:
		return "StopProcessResponse"
	case MsgStartProcessRequest:
		return "StartProcessRequest"
	case MsgStartProcessResponse:
		return "StartProcessResponse"
	case MsgExecuteCommandRequest:
		return "ExecuteCommandRequest"
	case MsgExecuteCommandResponse:
		return "ExecuteCommandResponse"
	case MsgListServicesRequest:
		return "ListServicesRequest"
	case MsgListServicesResponse:
		return "ListServicesResponse"
	case MsgGetServiceStatusRequest:
		return "GetServiceStatusRequest"
	case MsgGetServiceStatusResponse:
		return "GetServiceStatusResponse"
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
	default:
		return fmt.Sprintf("Unknown(0x%06X)", msgID)
	}
}
