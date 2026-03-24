// Package protocol provides NNG message protocol implementation for Croupier.
package protocol

import (
	"encoding/binary"
	"fmt"

	"go.nanomsg.org/mangos/v3"
)

const (
	// Version1 is the current protocol version
	Version1 = 0x01

	// HeaderSize is the size of the message header in bytes
	HeaderSize = 8 // Version(1) + MsgID(3) + RequestID(4)
)

// Message type IDs (24-bit, aligned with SDK)
const (
	// ControlService messages (0x01xx)
	MsgRegisterRequest          = 0x010101
	MsgRegisterResponse         = 0x010102
	MsgHeartbeatRequest         = 0x010103
	MsgHeartbeatResponse        = 0x010104
	MsgRegisterCapabilitiesReq  = 0x010105
	MsgRegisterCapabilitiesResp = 0x010106

	// ClientService messages (0x02xx)
	MsgRegisterClientRequest   = 0x020101
	MsgRegisterClientResponse  = 0x020102
	MsgClientHeartbeatRequest  = 0x020103
	MsgClientHeartbeatResponse = 0x020104
	MsgListClientsRequest      = 0x020105
	MsgListClientsResponse     = 0x020106
	MsgGetJobResultRequest     = 0x020107
	MsgGetJobResultResponse    = 0x020108

	// InvokerService messages (0x03xx)
	MsgInvokeRequest     = 0x030101
	MsgInvokeResponse    = 0x030102
	MsgStartJobRequest   = 0x030103
	MsgStartJobResponse  = 0x030104
	MsgStreamJobRequest  = 0x030105
	MsgJobEvent          = 0x030106
	MsgCancelJobRequest  = 0x030107
	MsgCancelJobResponse = 0x030108

	// OpsService messages (0x04xx)
	MsgGetSystemInfoRequest     = 0x040101
	MsgGetSystemInfoResponse    = 0x040102
	MsgListProcessesRequest     = 0x040103
	MsgListProcessesResponse    = 0x040104
	MsgReportMetricsRequest     = 0x040105
	MsgReportMetricsResponse    = 0x040106
	MsgStreamMetricsRequest     = 0x040107
	MsgMetricEvent              = 0x040108
	MsgRestartProcessRequest    = 0x040109
	MsgRestartProcessResponse   = 0x04010A
	MsgStopProcessRequest       = 0x04010B
	MsgStopProcessResponse      = 0x04010C
	MsgStartProcessRequest      = 0x04010D
	MsgStartProcessResponse     = 0x04010E
	MsgExecuteCommandRequest    = 0x04010F
	MsgExecuteCommandResponse   = 0x040110
	MsgListServicesRequest      = 0x040111
	MsgListServicesResponse     = 0x040112
	MsgGetServiceStatusRequest  = 0x040113
	MsgGetServiceStatusResponse = 0x040114

	// LocalControlService messages (0x05xx)
	MsgRegisterLocalRequest   = 0x050101
	MsgRegisterLocalResponse  = 0x050102
	MsgHeartbeatLocalRequest  = 0x050103
	MsgHeartbeatLocalResponse = 0x050104
	MsgListLocalRequest       = 0x050105
	MsgListLocalResponse      = 0x050106
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

// ParseMessage parses a mangos Message into its components.
func ParseMessage(msg *mangos.Message) (version uint8, msgID uint32, reqID uint32, body []byte, err error) {
	if len(msg.Header) < HeaderSize {
		return 0, 0, 0, nil, fmt.Errorf("header too short: %d < %d", len(msg.Header), HeaderSize)
	}

	version = msg.Header[0]
	msgID = GetMsgID(msg.Header[1:4])
	reqID = binary.BigEndian.Uint32(msg.Header[4:8])
	body = msg.Body

	return version, msgID, reqID, body, nil
}

// NewRequestMessage creates a new Request message.
func NewRequestMessage(msgID uint32, reqID uint32, body []byte) (*mangos.Message, error) {
	msg := mangos.NewMessage(0)
	msg.Header = make([]byte, HeaderSize)

	msg.Header[0] = Version1
	PutMsgID(msg.Header[1:4], msgID)                   // MsgID: 3 bytes
	binary.BigEndian.PutUint32(msg.Header[4:8], reqID) // RequestID: 4 bytes
	msg.Body = body

	return msg, nil
}

// NewResponseMessage creates a new Response message.
func NewResponseMessage(msgID uint32, reqID uint32, body []byte) (*mangos.Message, error) {
	msg := mangos.NewMessage(0)
	msg.Header = make([]byte, HeaderSize)

	msg.Header[0] = Version1
	PutMsgID(msg.Header[1:4], msgID)                   // MsgID: 3 bytes
	binary.BigEndian.PutUint32(msg.Header[4:8], reqID) // RequestID: 4 bytes
	msg.Body = body

	return msg, nil
}

// IsRequest判断是否是Request消息（奇数）
func IsRequest(msgID uint32) bool {
	return msgID%2 == 1 && msgID != MsgJobEvent && msgID != MsgMetricEvent
}

// IsResponse判断是否是Response消息（偶数）
func IsResponse(msgID uint32) bool {
	return msgID%2 == 0 && msgID != MsgJobEvent && msgID != MsgMetricEvent
}

// GetResponseMsgID获取对应的Response MsgID
func GetResponseMsgID(reqMsgID uint32) uint32 {
	return reqMsgID + 1
}

// MsgIDString返回MsgID的字符串表示.
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
	case MsgStartJobRequest:
		return "StartJobRequest"
	case MsgStartJobResponse:
		return "StartJobResponse"
	case MsgStreamJobRequest:
		return "StreamJobRequest"
	case MsgJobEvent:
		return "JobEvent"
	case MsgCancelJobRequest:
		return "CancelJobRequest"
	case MsgCancelJobResponse:
		return "CancelJobResponse"
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
	case MsgRegisterLocalRequest:
		return "RegisterLocalRequest"
	case MsgRegisterLocalResponse:
		return "RegisterLocalResponse"
	case MsgHeartbeatLocalRequest:
		return "HeartbeatLocalRequest"
	case MsgHeartbeatLocalResponse:
		return "HeartbeatLocalResponse"
	case MsgListLocalRequest:
		return "ListLocalRequest"
	case MsgListLocalResponse:
		return "ListLocalResponse"
	default:
		return fmt.Sprintf("Unknown(0x%06X)", msgID)
	}
}

// DebugString返回消息的调试字符串.
func DebugString(msg *mangos.Message) string {
	version, msgID, reqID, _, err := ParseMessage(msg)
	if err != nil {
		return fmt.Sprintf("Message{ParseError: %v}", err)
	}
	return fmt.Sprintf("Message{Ver=%d, MsgID=%s, ReqID=%d}", version, MsgIDString(msgID), reqID)
}
