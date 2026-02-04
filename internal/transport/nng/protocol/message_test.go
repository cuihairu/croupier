package protocol

import (
	"encoding/binary"
	"testing"

	"go.nanomsg.org/mangos/v3"
)

// TestPutMsgID_GetMsgID 测试 MsgID 编码解码
func TestPutMsgID_GetMsgID(t *testing.T) {
	buf := make([]byte, 3)

	testCases := []struct {
		input  uint32
		expect uint32
	}{
		{0x000000, 0x000000},
		{0x000001, 0x000001},
		{0x010203, 0x010203},
		{0xFFFFFF, 0xFFFFFF},
		{0x123456, 0x123456},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			PutMsgID(buf, tc.input)
			result := GetMsgID(buf)
			if result != tc.expect {
				t.Errorf("PutMsgID(%X) -> GetMsgID() = %X, want %X", tc.input, result, tc.expect)
			}
		})
	}
}

// TestNewRequestMessage 测试创建请求消息
func TestNewRequestMessage(t *testing.T) {
	body := []byte("test payload")

	msg, err := NewRequestMessage(MsgInvokeRequest, 12345, body)
	if err != nil {
		t.Fatalf("NewRequestMessage() error = %v", err)
	}

	if msg == nil {
		t.Fatal("NewRequestMessage() should return non-nil message")
	}

	if len(msg.Header) != HeaderSize {
		t.Errorf("Header size should be %d, got %d", HeaderSize, len(msg.Header))
	}

	if msg.Header[0] != Version1 {
		t.Errorf("Version should be %d, got %d", Version1, msg.Header[0])
	}

	// 验证 MsgID
	msgID := GetMsgID(msg.Header[1:4])
	if msgID != MsgInvokeRequest {
		t.Errorf("MsgID should be %X, got %X", MsgInvokeRequest, msgID)
	}

	// 验证 RequestID
	reqID := binary.BigEndian.Uint32(msg.Header[4:8])
	if reqID != 12345 {
		t.Errorf("RequestID should be 12345, got %d", reqID)
	}

	if string(msg.Body) != "test payload" {
		t.Errorf("Body should be 'test payload', got '%s'", string(msg.Body))
	}
}

// TestNewResponseMessage 测试创建响应消息
func TestNewResponseMessage(t *testing.T) {
	body := []byte("response payload")

	msg, err := NewResponseMessage(MsgInvokeResponse, 67890, body)
	if err != nil {
		t.Fatalf("NewResponseMessage() error = %v", err)
	}

	if msg == nil {
		t.Fatal("NewResponseMessage() should return non-nil message")
	}

	if len(msg.Header) != HeaderSize {
		t.Errorf("Header size should be %d, got %d", HeaderSize, len(msg.Header))
	}

	// 验证 MsgID
	msgID := GetMsgID(msg.Header[1:4])
	if msgID != MsgInvokeResponse {
		t.Errorf("MsgID should be %X, got %X", MsgInvokeResponse, msgID)
	}
}

// TestNewRequestMessage_EmptyBody 测试空消息体
func TestNewRequestMessage_EmptyBody(t *testing.T) {
	msg, err := NewRequestMessage(MsgHeartbeatRequest, 999, []byte{})
	if err != nil {
		t.Fatalf("NewRequestMessage() with empty body error = %v", err)
	}

	if len(msg.Body) != 0 {
		t.Error("Body should be empty for nil input")
	}
}

// TestParseMessage 测试解析消息
func TestParseMessage(t *testing.T) {
	// 创建测试消息
	originalMsg, _ := NewRequestMessage(MsgRegisterRequest, 0xABCDEF, []byte("test data"))

	// 解析消息
	version, msgID, reqID, body, err := ParseMessage(originalMsg)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}

	if version != Version1 {
		t.Errorf("Version should be %d, got %d", Version1, version)
	}

	if msgID != MsgRegisterRequest {
		t.Errorf("MsgID should be %X, got %X", MsgRegisterRequest, msgID)
	}

	if reqID != 0xABCDEF {
		t.Errorf("RequestID should be 0xABCDEF, got 0x%X", reqID)
	}

	if string(body) != "test data" {
		t.Errorf("Body should be 'test data', got '%s'", string(body))
	}
}

// TestParseMessage_HeaderTooShort 测试解析过短的消息头
func TestParseMessage_HeaderTooShort(t *testing.T) {
	msg := mangos.NewMessage(0)
	msg.Header = []byte{0x01} // 只有1字节，需要8字节

	_, _, _, _, err := ParseMessage(msg)
	if err == nil {
		t.Error("ParseMessage() with short header should return error")
	}
}

// TestParseMessage_EmptyHeader 测试空消息头
func TestParseMessage_EmptyHeader(t *testing.T) {
	msg := mangos.NewMessage(0)
	msg.Header = []byte{} // 空头

	_, _, _, _, err := ParseMessage(msg)
	if err == nil {
		t.Error("ParseMessage() with empty header should return error")
	}
}

// TestIsRequest_IsResponse 测试消息类型判断
func TestIsRequest_IsResponse(t *testing.T) {
	tests := []struct {
		msgID  uint32
		isReq  bool
		isResp bool
	}{
		{MsgInvokeRequest, true, false},
		{MsgInvokeResponse, false, true},
		{MsgStartJobRequest, true, false},
		{MsgStartJobResponse, false, true},
		{MsgStreamJobRequest, true, false},
		{MsgJobEvent, false, true}, // MsgJobEvent 是偶数，会被 IsResponse 判断为 true
		{MsgCancelJobRequest, true, false},
		{MsgCancelJobResponse, false, true},
		{MsgRegisterRequest, true, false},
		{MsgRegisterResponse, false, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if gotReq := IsRequest(tt.msgID); gotReq != tt.isReq {
				t.Errorf("IsRequest(%X) = %v, want %v", tt.msgID, gotReq, tt.isReq)
			}
			if gotResp := IsResponse(tt.msgID); gotResp != tt.isResp {
				t.Errorf("IsResponse(%X) = %v, want %v", tt.msgID, gotResp, tt.isResp)
			}
		})
	}
}

// TestGetResponseMsgID 测试获取响应消息ID
func TestGetResponseMsgID(t *testing.T) {
	tests := []struct {
		reqID  uint32
		respID uint32
	}{
		{MsgInvokeRequest, MsgInvokeResponse},
		{MsgStartJobRequest, MsgStartJobResponse},
		{MsgCancelJobRequest, MsgCancelJobResponse},
		{MsgRegisterRequest, MsgRegisterResponse},
		{MsgHeartbeatRequest, MsgHeartbeatResponse},
		{MsgStreamJobRequest, 0x030106}, // StreamJobRequest + 1
		{0x030107, 0x030108},
		{0x030108, 0x030109},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := GetResponseMsgID(tt.reqID)
			if got != tt.respID {
				t.Errorf("GetResponseMsgID(%X) = %X, want %X", tt.reqID, got, tt.respID)
			}
		})
	}
}

// TestMsgIDString 测试消息ID字符串表示
func TestMsgIDString(t *testing.T) {
	tests := []struct {
		msgID     uint32
		expectStr string
	}{
		{MsgRegisterRequest, "RegisterRequest"},
		{MsgRegisterResponse, "RegisterResponse"},
		{MsgHeartbeatRequest, "HeartbeatRequest"},
		{MsgHeartbeatResponse, "HeartbeatResponse"},
		{MsgRegisterClientRequest, "RegisterClientRequest"},
		{MsgRegisterClientResponse, "RegisterClientResponse"},
		{MsgClientHeartbeatRequest, "ClientHeartbeatRequest"},
		{MsgClientHeartbeatResponse, "ClientHeartbeatResponse"},
		{MsgInvokeRequest, "InvokeRequest"},
		{MsgInvokeResponse, "InvokeResponse"},
		{MsgStartJobRequest, "StartJobRequest"},
		{MsgStartJobResponse, "StartJobResponse"},
		{MsgStreamJobRequest, "StreamJobRequest"},
		{MsgJobEvent, "JobEvent"},
		{MsgCancelJobRequest, "CancelJobRequest"},
		{MsgCancelJobResponse, "CancelJobResponse"},
		{0x0, "Unknown(0x000000)"},
		{0xFFFFFFFF, "Unknown(0xFFFFFFFF)"},
	}

	for _, tt := range tests {
		t.Run(tt.expectStr, func(t *testing.T) {
			got := MsgIDString(tt.msgID)
			if got != tt.expectStr {
				t.Errorf("MsgIDString(%X) = '%s', want '%s'", tt.msgID, got, tt.expectStr)
			}
		})
	}
}

// TestDebugString 测试调试字符串
func TestDebugString(t *testing.T) {
	msg, _ := NewRequestMessage(MsgInvokeRequest, 12345, []byte("test"))

	debugStr := DebugString(msg)
	if debugStr == "" {
		t.Error("DebugString() should return non-empty string")
	}

	// 验证字符串包含关键信息
	expected := "InvokeRequest"
	if !contains(debugStr, expected) {
		t.Errorf("DebugString() should contain '%s', got '%s'", expected, debugStr)
	}
}

// TestDebugString_ParseError 测试错误消息的调试字符串
func TestDebugString_ParseError(t *testing.T) {
	msg := mangos.NewMessage(0)
	msg.Header = []byte{0x01} // 太短

	debugStr := DebugString(msg)
	if debugStr == "" {
		t.Error("DebugString() should return non-empty string even with parse error")
	}

	// 应该包含 ParseError
	if !contains(debugStr, "ParseError") {
		t.Errorf("DebugString() with error should contain 'ParseError', got '%s'", debugStr)
	}
}

// TestNewRequestMessage_MaxValues 测试最大值
func TestNewRequestMessage_MaxValues(t *testing.T) {
	body := []byte("max values test")

	msg, err := NewRequestMessage(0xFFFFFF, 0xFFFFFFFF, body)
	if err != nil {
		t.Fatalf("NewRequestMessage() with max values error = %v", err)
	}

	// 验证 RequestID 编码正确
	reqID := binary.BigEndian.Uint32(msg.Header[4:8])
	if reqID != 0xFFFFFFFF {
		t.Errorf("RequestID should be 0xFFFFFFFF, got 0x%X", reqID)
	}

	// 验证 MsgID 编码正确
	msgID := GetMsgID(msg.Header[1:4])
	if msgID != 0xFFFFFF {
		t.Errorf("MsgID should be 0xFFFFFF, got 0x%X", msgID)
	}
}

// TestMessageRoundTrip 测试消息往返
func TestMessageRoundTrip(t *testing.T) {
	// 创建原始消息
	originalBody := []byte("round trip test")
	originalReqID := uint32(0xDEADBEEF)

	// 创建消息
	msg1, _ := NewRequestMessage(MsgStartJobRequest, originalReqID, originalBody)

	// 解析消息
	_, msgID, reqID, body, err := ParseMessage(msg1)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}

	// 验证解析出的 body 与原始 body 一致
	if string(body) != string(originalBody) {
		t.Errorf("Parsed body mismatch: got '%s', want '%s'", string(body), string(originalBody))
	}

	// 创建响应消息（使用响应 MsgID）
	msg2, _ := NewResponseMessage(GetResponseMsgID(msgID), reqID, []byte("response"))

	// 解析响应
	_, respMsgID, respReqID, respBody, err := ParseMessage(msg2)
	if err != nil {
		t.Fatalf("ParseMessage() response error = %v", err)
	}

	// 验证往返一致性
	if respMsgID != GetResponseMsgID(msgID) {
		t.Errorf("Response MsgID mismatch: got %X, want %X", respMsgID, GetResponseMsgID(msgID))
	}

	if respReqID != reqID {
		t.Errorf("RequestID mismatch: got %d, want %d", respReqID, reqID)
	}

	if string(respBody) != "response" {
		t.Errorf("Response body mismatch: got '%s'", string(respBody))
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
