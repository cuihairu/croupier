// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package protocol

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestFormatHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   []byte
		contains []string
	}{
		{
			name:     "valid header",
			header:   []byte{0x01, 0x03, 0x01, 0x01, 0x00, 0x00, 0x30, 0x39},
			contains: []string{"Ver=1", "MsgID=InvokeRequest", "ReqID=12345"},
		},
		{
			name:     "invalid header - too short",
			header:   []byte{0x01, 0x02},
			contains: []string{"invalid header"},
		},
		{
			name:     "empty header",
			header:   []byte{},
			contains: []string{"invalid header"},
		},
		{
			name:     "partial header",
			header:   []byte{0x01, 0x03, 0x01},
			contains: []string{"invalid header"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatHeader(tt.header)

			for _, substr := range tt.contains {
				if !contains(result, substr) {
					t.Errorf("FormatHeader() output = %s, should contain %s", result, substr)
				}
			}
		})
	}
}

func TestParseMessageInfo(t *testing.T) {
	tests := []struct {
		name         string
		body         []byte
		wantErr      bool
		checkVersion uint8
		checkMsgID   uint32
		checkReqID   uint32
		checkBodyLen int
		checkIsReq   bool
		checkIsResp  bool
	}{
		{
			name:         "valid request message",
			body:         []byte{0x01, 0x03, 0x01, 0x01, 0x00, 0x00, 0x30, 0x39, 0x01, 0x02},
			wantErr:      false,
			checkVersion: Version1,
			checkMsgID:   MsgInvokeRequest,
			checkReqID:   12345,
			checkBodyLen: 2,
			checkIsReq:   true,
			checkIsResp:  false,
		},
		{
			name:         "valid response message",
			body:         []byte{0x01, 0x03, 0x01, 0x02, 0x00, 0x00, 0x30, 0x39, 0x01},
			wantErr:      false,
			checkVersion: Version1,
			checkMsgID:   MsgInvokeResponse,
			checkReqID:   12345,
			checkBodyLen: 1,
			checkIsReq:   false,
			checkIsResp:  true,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseMessageInfo(tt.body)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if info.Version != tt.checkVersion {
				t.Errorf("Version = %d, want %d", info.Version, tt.checkVersion)
			}
			if info.MsgID != tt.checkMsgID {
				t.Errorf("MsgID = %#x, want %#x", info.MsgID, tt.checkMsgID)
			}
			if info.ReqID != tt.checkReqID {
				t.Errorf("ReqID = %d, want %d", info.ReqID, tt.checkReqID)
			}
			if info.BodyLen != tt.checkBodyLen {
				t.Errorf("BodyLen = %d, want %d", info.BodyLen, tt.checkBodyLen)
			}
			if info.IsReq != tt.checkIsReq {
				t.Errorf("IsReq = %v, want %v", info.IsReq, tt.checkIsReq)
			}
			if info.IsResp != tt.checkIsResp {
				t.Errorf("IsResp = %v, want %v", info.IsResp, tt.checkIsResp)
			}
		})
	}
}

func TestMessageInfoString(t *testing.T) {
	body := []byte{0x01, 0x03, 0x01, 0x01, 0x00, 0x00, 0x30, 0x39, 0x01, 0x02}
	info, err := ParseMessageInfo(body)
	if err != nil {
		t.Fatalf("ParseMessageInfo failed: %v", err)
	}

	str := info.String()

	// Check that String() contains key information
	checks := []string{
		"Ver=1",
		"MsgID=InvokeRequest",
		"ReqID=12345",
		"BodyLen=2",
		"IsReq=true",
		"IsResp=false",
	}

	for _, check := range checks {
		if !contains(str, check) {
			t.Errorf("String() output = %s, should contain %s", str, check)
		}
	}
}

func TestDebugStringForBody(t *testing.T) {
	tests := []struct {
		name         string
		msgID        uint32
		reqID        uint32
		body         []byte
		bodyMsg      proto.Message
		shouldErrMsg bool
		contains     []string
	}{
		{
			name:     "with nil body message",
			msgID:    MsgInvokeRequest,
			reqID:    12345,
			body:     []byte{0x01, 0x02, 0x03},
			contains: []string{"MsgID=InvokeRequest", "ReqID=12345", "010203"},
		},
		{
			name:     "with empty body",
			msgID:    MsgInvokeRequest,
			reqID:    1,
			body:     []byte{},
			contains: []string{"MsgID=InvokeRequest", "ReqID=1", "<body>"},
		},
		{
			name:     "with small body",
			msgID:    MsgHeartbeatRequest,
			reqID:    999,
			body:     []byte{0xAA, 0xBB},
			contains: []string{"MsgID=HeartbeatRequest", "ReqID=999", "aabb"},
		},
		{
			name:     "with large body",
			msgID:    MsgInvokeRequest,
			reqID:    1,
			body:     make([]byte, 100),
			contains: []string{"MsgID=InvokeRequest", "...", "100 bytes"},
		},
		{
			name:     "exactly 32 bytes",
			msgID:    MsgInvokeRequest,
			reqID:    1,
			body:     make([]byte, 32),
			contains: []string{"00000000"}, // Should show full hex dump
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DebugStringForBody(tt.msgID, tt.reqID, tt.body, tt.bodyMsg)

			for _, substr := range tt.contains {
				if !contains(result, substr) {
					t.Errorf("DebugStringForBody() output = %s, should contain %s", result, substr)
				}
			}

			// All results should contain MsgID in hex
			if !contains(result, "0x") {
				t.Errorf("DebugStringForBody() output should contain hex MsgID: %s", result)
			}
		})
	}
}

func TestDebugStringForBodyWithProtoMessage(t *testing.T) {
	// Test with nil proto message - should show hex dump
	msgID := uint32(MsgInvokeRequest)
	reqID := uint32(12345)
	body := []byte{0x01, 0x02, 0x03}

	result := DebugStringForBody(msgID, reqID, body, nil)

	// Should contain hex dump since bodyMsg is nil
	if !contains(result, "010203") {
		t.Errorf("expected hex dump in output, got: %s", result)
	}
}

func TestDebugStringForBody_LargeBody(t *testing.T) {
	// Test that large bodies are truncated
	msgID := uint32(MsgInvokeRequest)
	reqID := uint32(12345)
	body := make([]byte, 100)

	result := DebugStringForBody(msgID, reqID, body, nil)

	// Should indicate truncation
	if !contains(result, "...") {
		t.Errorf("expected truncation indicator in output, got: %s", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
	 indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
