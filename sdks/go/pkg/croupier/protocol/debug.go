// Package protocol provides debugging utilities for Croupier protocol.
package protocol

import (
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// DebugStringForBody returns a human-readable string representation of a message body.
// It attempts to parse the body as protobuf and format it as JSON if possible.
func DebugStringForBody(msgID uint32, reqID uint32, body []byte, bodyMsg proto.Message) string {
	bodyStr := "<body>"
	if bodyMsg != nil {
		if jsonBytes, err := protojson.Marshal(bodyMsg); err == nil {
			bodyStr = string(jsonBytes)
		} else {
			bodyStr = fmt.Sprintf("<json error: %v>", err)
		}
	} else if len(body) > 0 {
		// Try to show hex dump for unknown body types
		if len(body) <= 32 {
			bodyStr = fmt.Sprintf("%x", body)
		} else {
			bodyStr = fmt.Sprintf("%x... (%d bytes)", body[:32], len(body))
		}
	}

	return fmt.Sprintf("Message{MsgID=%s(0x%06X), ReqID=%d, Body=%s}",
		MsgIDString(msgID), msgID, reqID, bodyStr)
}

// FormatHeader returns a formatted string representation of just the header.
func FormatHeader(header []byte) string {
	if len(header) < HeaderSize {
		return fmt.Sprintf("<invalid header: %d bytes>", len(header))
	}

	version := header[0]
	msgID := GetMsgID(header[1:4])
	reqID := binary.BigEndian.Uint32(header[4:8])

	return fmt.Sprintf("Header{Ver=%d, MsgID=%s(0x%06X), ReqID=%d}",
		version, MsgIDString(msgID), msgID, reqID)
}

// MessageInfo holds parsed message information.
type MessageInfo struct {
	Version uint8
	MsgID   uint32
	ReqID   uint32
	BodyLen int
	IsReq   bool
	IsResp  bool
}

// ParseMessageInfo parses a message body and returns structured information.
func ParseMessageInfo(body []byte) (*MessageInfo, error) {
	info := &MessageInfo{}

	version, msgID, reqID, data, err := ParseMessageFromBody(body)
	if err != nil {
		return nil, err
	}

	info.Version = version
	info.MsgID = msgID
	info.ReqID = reqID
	info.BodyLen = len(data)
	info.IsReq = IsRequest(msgID)
	info.IsResp = IsResponse(msgID)

	return info, nil
}

// String returns a string representation of MessageInfo.
func (i *MessageInfo) String() string {
	return fmt.Sprintf("MessageInfo{Ver=%d, MsgID=%s(0x%06X), ReqID=%d, BodyLen=%d, IsReq=%v, IsResp=%v}",
		i.Version, MsgIDString(i.MsgID), i.MsgID, i.ReqID, i.BodyLen, i.IsReq, i.IsResp)
}
