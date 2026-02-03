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
	MsgRegisterRequest   = 0x010101
	MsgRegisterResponse  = 0x010102
	MsgHeartbeatRequest  = 0x010103
	MsgHeartbeatResponse = 0x010104

	// ClientService messages (0x02xx)
	MsgRegisterClientRequest   = 0x020101
	MsgRegisterClientResponse  = 0x020102
	MsgClientHeartbeatRequest  = 0x020103
	MsgClientHeartbeatResponse = 0x020104

	// InvokerService messages (0x03xx)
	MsgInvokeRequest     = 0x030101
	MsgInvokeResponse    = 0x030102
	MsgStartJobRequest   = 0x030103
	MsgStartJobResponse  = 0x030104
	MsgStreamJobRequest  = 0x030105
	MsgJobEvent          = 0x030106
	MsgCancelJobRequest  = 0x030107
	MsgCancelJobResponse = 0x030108
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
	return msgID%2 == 1 && msgID != MsgJobEvent
}

// IsResponse判断是否是Response消息（偶数）
func IsResponse(msgID uint32) bool {
	return msgID%2 == 0
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
	case MsgRegisterClientRequest:
		return "RegisterClientRequest"
	case MsgRegisterClientResponse:
		return "RegisterClientResponse"
	case MsgClientHeartbeatRequest:
		return "ClientHeartbeatRequest"
	case MsgClientHeartbeatResponse:
		return "ClientHeartbeatResponse"
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
