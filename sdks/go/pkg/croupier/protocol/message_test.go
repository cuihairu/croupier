// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package protocol

import (
	"encoding/binary"
	"testing"
)

func TestPutMsgID(t *testing.T) {
	tests := []struct {
		name  string
		msgID uint32
	}{
		{"small", 0x000101},
		{"medium", 0x010203},
		{"large", 0xFFFFFF},
		{"InvokeRequest", MsgInvokeRequest},
		{"InvokeResponse", MsgInvokeResponse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 3)
			PutMsgID(buf, tt.msgID)

			result := GetMsgID(buf)
			if result != tt.msgID {
				t.Errorf("PutMsgID(%#x) -> GetMsgID() = %#x, want %#x", tt.msgID, result, tt.msgID)
			}
		})
	}
}

func TestGetMsgID(t *testing.T) {
	tests := []struct {
		name     string
		buf      []byte
		expected uint32
	}{
		{
			name:     "basic",
			buf:      []byte{0x01, 0x02, 0x03},
			expected: 0x010203,
		},
		{
			name:     "max value",
			buf:      []byte{0xFF, 0xFF, 0xFF},
			expected: 0xFFFFFF,
		},
		{
			name:     "zero",
			buf:      []byte{0x00, 0x00, 0x00},
			expected: 0x000000,
		},
		{
			name:     "InvokeRequest",
			buf:      []byte{0x03, 0x01, 0x01},
			expected: MsgInvokeRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMsgID(tt.buf)
			if result != tt.expected {
				t.Errorf("GetMsgID(%v) = %#x, want %#x", tt.buf, result, tt.expected)
			}
		})
	}
}

func TestNewMessageBody(t *testing.T) {
	tests := []struct {
		name      string
		msgID     uint32
		reqID     uint32
		data      []byte
		wantLen   int
		checkBody bool
	}{
		{
			name:    "basic message",
			msgID:   MsgInvokeRequest,
			reqID:   12345,
			data:    []byte{1, 2, 3, 4, 5},
			wantLen: HeaderSize + 5,
		},
		{
			name:    "empty data",
			msgID:   MsgHeartbeatRequest,
			reqID:   1,
			data:    []byte{},
			wantLen: HeaderSize,
		},
		{
			name:    "nil data",
			msgID:   MsgInvokeRequest,
			reqID:   999,
			data:    nil,
			wantLen: HeaderSize,
		},
		{
			name:      "large data",
			msgID:     MsgStartTaskRequest,
			reqID:     777,
			data:      make([]byte, 1000),
			wantLen:   HeaderSize + 1000,
			checkBody: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := NewMessageBody(tt.msgID, tt.reqID, tt.data)

			if len(body) != tt.wantLen {
				t.Fatalf("NewMessageBody() len = %d, want %d", len(body), tt.wantLen)
			}

			// Check header
			if body[0] != Version1 {
				t.Errorf("version = %d, want %d", body[0], Version1)
			}

			msgID := GetMsgID(body[1:4])
			if msgID != tt.msgID {
				t.Errorf("msgID = %#x, want %#x", msgID, tt.msgID)
			}

			reqID := binary.BigEndian.Uint32(body[4:8])
			if reqID != tt.reqID {
				t.Errorf("reqID = %d, want %d", reqID, tt.reqID)
			}

			// Check data
			if tt.checkBody || len(tt.data) <= 100 {
				dataPart := body[HeaderSize:]
				if len(dataPart) != len(tt.data) {
					t.Errorf("data part len = %d, want %d", len(dataPart), len(tt.data))
				}
				for i := range tt.data {
					if dataPart[i] != tt.data[i] {
						t.Errorf("dataPart[%d] = %d, want %d", i, dataPart[i], tt.data[i])
					}
				}
			}
		})
	}
}

func TestParseMessageFromBody(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		wantVersion uint8
		wantMsgID   uint32
		wantReqID   uint32
		wantDataLen int
		wantErr     bool
	}{
		{
			name:        "valid message",
			body:        []byte{0x01, 0x03, 0x01, 0x01, 0x00, 0x00, 0x30, 0x39, 0x01, 0x02},
			wantVersion: Version1,
			wantMsgID:   MsgInvokeRequest,
			wantReqID:   12345,
			wantDataLen: 2,
		},
		{
			name:        "message with empty data",
			body:        []byte{0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x01},
			wantVersion: Version1,
			wantMsgID:   MsgRegisterRequest,
			wantReqID:   1,
			wantDataLen: 0,
		},
		{
			name:    "too short",
			body:    []byte{0x01, 0x02},
			wantErr: true,
		},
		{
			name:    "empty",
			body:    []byte{},
			wantErr: true,
		},
		{
			name:        "exactly header size",
			body:        make([]byte, HeaderSize),
			wantVersion: 0,
			wantMsgID:   0,
			wantReqID:   0,
			wantDataLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, msgID, reqID, data, err := ParseMessageFromBody(tt.body)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if version != tt.wantVersion {
				t.Errorf("version = %d, want %d", version, tt.wantVersion)
			}
			if msgID != tt.wantMsgID {
				t.Errorf("msgID = %#x, want %#x", msgID, tt.wantMsgID)
			}
			if reqID != tt.wantReqID {
				t.Errorf("reqID = %d, want %d", reqID, tt.wantReqID)
			}
			if len(data) != tt.wantDataLen {
				t.Errorf("data len = %d, want %d", len(data), tt.wantDataLen)
			}
		})
	}
}

func TestIsRequest(t *testing.T) {
	tests := []struct {
		name     string
		msgID    uint32
		expected bool
	}{
		{"RegisterRequest", MsgRegisterRequest, true},
		{"InvokeRequest", MsgInvokeRequest, true},
		{"HeartbeatRequest", MsgHeartbeatRequest, true},
		{"InvokeResponse", MsgInvokeResponse, false},
		{"RegisterResponse", MsgRegisterResponse, false},
		{"TaskEvent", MsgTaskEvent, false}, // Event, not request
		{"odd non-event", 0x010101, true},
		{"even non-event", 0x010102, false},
		{"odd event", 0x010106, false}, // Matches TaskEvent pattern
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRequest(tt.msgID)
			if result != tt.expected {
				t.Errorf("IsRequest(%#x) = %v, want %v", tt.msgID, result, tt.expected)
			}
		})
	}
}

func TestIsResponse(t *testing.T) {
	tests := []struct {
		name     string
		msgID    uint32
		expected bool
	}{
		{"RegisterResponse", MsgRegisterResponse, true},
		{"InvokeResponse", MsgInvokeResponse, true},
		{"HeartbeatResponse", MsgHeartbeatResponse, true},
		{"InvokeRequest", MsgInvokeRequest, false},
		{"RegisterRequest", MsgRegisterRequest, false},
		{"TaskEvent", MsgTaskEvent, false}, // Event, not response
		{"even non-event", 0x010102, true},
		{"odd non-event", 0x010101, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsResponse(tt.msgID)
			if result != tt.expected {
				t.Errorf("IsResponse(%#x) = %v, want %v", tt.msgID, result, tt.expected)
			}
		})
	}
}

func TestGetResponseMsgID(t *testing.T) {
	tests := []struct {
		name     string
		reqMsgID uint32
		expected uint32
	}{
		{"RegisterRequest", MsgRegisterRequest, MsgRegisterResponse},
		{"InvokeRequest", MsgInvokeRequest, MsgInvokeResponse},
		{"HeartbeatRequest", MsgHeartbeatRequest, MsgHeartbeatResponse},
		{"odd value", 0x010101, 0x010102},
		{"even value", 0x010102, 0x010103},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetResponseMsgID(tt.reqMsgID)
			if result != tt.expected {
				t.Errorf("GetResponseMsgID(%#x) = %#x, want %#x", tt.reqMsgID, result, tt.expected)
			}
		})
	}
}

func TestMsgIDString(t *testing.T) {
	tests := []struct {
		name     string
		msgID    uint32
		expected string
	}{
		{"RegisterRequest", MsgRegisterRequest, "RegisterRequest"},
		{"RegisterResponse", MsgRegisterResponse, "RegisterResponse"},
		{"InvokeRequest", MsgInvokeRequest, "InvokeRequest"},
		{"InvokeResponse", MsgInvokeResponse, "InvokeResponse"},
		{"HeartbeatRequest", MsgHeartbeatRequest, "HeartbeatRequest"},
		{"TaskEvent", MsgTaskEvent, "TaskEvent"},
		{"Unknown", 0x999999, "Unknown(0x999999)"},
		{"Zero", 0x000000, "Unknown(0x000000)"},
		{"Max", 0xFFFFFF, "Unknown(0xFFFFFF)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MsgIDString(tt.msgID)
			if result != tt.expected {
				t.Errorf("MsgIDString(%#x) = %s, want %s", tt.msgID, result, tt.expected)
			}
		})
	}
}

func TestRoundTripMessage(t *testing.T) {
	// Test that creating and parsing a message preserves the data
	originalData := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	msgID := uint32(MsgInvokeRequest)
	reqID := uint32(54321)

	body := NewMessageBody(msgID, reqID, originalData)

	version, parsedMsgID, parsedReqID, data, err := ParseMessageFromBody(body)
	if err != nil {
		t.Fatalf("ParseMessageFromBody failed: %v", err)
	}

	if version != Version1 {
		t.Errorf("version = %d, want %d", version, Version1)
	}
	if parsedMsgID != msgID {
		t.Errorf("msgID = %#x, want %#x", parsedMsgID, msgID)
	}
	if parsedReqID != reqID {
		t.Errorf("reqID = %d, want %d", parsedReqID, reqID)
	}

	if len(data) != len(originalData) {
		t.Fatalf("data len = %d, want %d", len(data), len(originalData))
	}

	for i := range originalData {
		if data[i] != originalData[i] {
			t.Errorf("data[%d] = %d, want %d", i, data[i], originalData[i])
		}
	}
}
