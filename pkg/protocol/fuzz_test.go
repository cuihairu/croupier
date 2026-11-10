package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// FuzzParseMessageFromBody 夜间 fuzz 入口（CR-FIX-25）：任意字节喂
// ParseMessageFromBody，断言头解析与 body 切片纯视图语义（不复制、不改写）。
func FuzzParseMessageFromBody(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x01})
	f.Add([]byte{0x01, 0x00, 0x00, 0x01})
	f.Add([]byte{0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x2a})
	f.Add(NewMessageBody(MsgHeartbeatRequest, 7, []byte("payload")))
	f.Add(NewMessageBody(MsgRegisterRequest, 0, nil))

	f.Fuzz(func(t *testing.T, body []byte) {
		version, msgID, reqID, data, err := ParseMessageFromBody(body)
		if len(body) < HeaderSize {
			if err == nil {
				t.Fatalf("body shorter than header must error, got version=%d msgID=%d", version, msgID)
			}
			return
		}
		if err != nil {
			t.Fatalf("valid-length body must parse, got error: %v", err)
		}
		if version != body[0] {
			t.Fatalf("version mismatch: got %d want %d", version, body[0])
		}
		if msgID != GetMsgID(body[1:4]) {
			t.Fatalf("msgID mismatch: got %d want %d", msgID, GetMsgID(body[1:4]))
		}
		if reqID != binary.BigEndian.Uint32(body[4:8]) {
			t.Fatalf("reqID mismatch: got %d want %d", reqID, binary.BigEndian.Uint32(body[4:8]))
		}
		if !bytes.Equal(data, body[HeaderSize:]) {
			t.Fatalf("data view mismatch: got %d bytes want %d", len(data), len(body)-HeaderSize)
		}
	})
}

// FuzzMessageBodyRoundTrip 夜间 fuzz 入口（CR-FIX-25）：构造→解析闭环，
// 断言 NewMessageBody 产出的消息经 ParseMessageFromBody 无损还原。
func FuzzMessageBodyRoundTrip(f *testing.F) {
	f.Add(uint32(MsgHeartbeatRequest), uint32(1), []byte("payload"))
	f.Add(uint32(0xffffffff), uint32(0xffffffff), []byte{})
	f.Add(uint32(MsgRegisterRequest), uint32(42), []byte(nil))

	f.Fuzz(func(t *testing.T, msgID uint32, reqID uint32, data []byte) {
		body := NewMessageBody(msgID, reqID, data)
		if body == nil {
			// 仅在长度域溢出时为 nil；uint32 长度 + 8B 头在 64 位 int 下不溢出
			t.Fatalf("NewMessageBody returned nil for msgID=%d len=%d", msgID, len(data))
		}
		gotVersion, gotMsgID, gotReqID, gotData, err := ParseMessageFromBody(body)
		if err != nil {
			t.Fatalf("round trip parse failed: %v", err)
		}
		if gotVersion != Version1 {
			t.Fatalf("version: got %d want %d", gotVersion, Version1)
		}
		// MsgID 线上格式是 3 字节（24 位，PutMsgID/GetMsgID），高位静默截断
		// 是既有 wire 契约——round trip 断言按掩码值比较
		if gotMsgID != msgID&0x00ffffff {
			t.Fatalf("msgID: got %d want %d", gotMsgID, msgID&0x00ffffff)
		}
		if gotReqID != reqID {
			t.Fatalf("reqID: got %d want %d", gotReqID, reqID)
		}
		if !bytes.Equal(gotData, data) {
			t.Fatalf("data: got %d bytes want %d", len(gotData), len(data))
		}
	})
}
