package croupier

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func newFilePushManager(t *testing.T, enable bool) *TCPManager {
	t.Helper()
	config := ClientConfig{
		EnableFileTransfer: enable,
		MaxFileSize:        1024,
		FileStagingDir:     filepath.Join(t.TempDir(), "staging"),
	}
	manager, err := NewTCPManager(config, map[string]FunctionHandler{})
	if err != nil {
		t.Fatalf("NewTCPManager: %v", err)
	}
	m := manager.(*TCPManager)
	m.functions = []*sdkv1.ProviderFunctionDescriptor{{Id: "f1"}}
	return m
}

// F：文件下发原语——编解码回路。
func TestFilePushCodecRoundTrip(t *testing.T) {
	req := &filePushRequest{
		transferID:    "t-1",
		fileName:      "patch.lua",
		contentSha256: "abc123",
		data:          []byte("payload-bytes"),
	}
	decoded, err := decodeFilePushRequest(encodeFilePushRequest(req))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.transferID != "t-1" || decoded.fileName != "patch.lua" ||
		decoded.contentSha256 != "abc123" || string(decoded.data) != "payload-bytes" {
		t.Fatalf("round trip mismatch: %+v", decoded)
	}
}

func TestFilePushResponseCodec(t *testing.T) {
	resp := encodeFilePushResponse(filePushResponse{
		transferID: "t-9",
		ok:         true,
		storedPath: "/staging/patch.lua",
	})
	if resp[0] != 0x0A {
		t.Fatalf("field1 tag = %x", resp[0])
	}
	body := string(resp)
	if !strings.Contains(body, "t-9") || !strings.Contains(body, "/staging/patch.lua") {
		t.Fatalf("response content mismatch: %x", resp)
	}
	if !strings.Contains(body, string([]byte{0x10, 0x01})) {
		t.Fatalf("ok=true not encoded: %x", resp)
	}
}

// F：安全约束——每条一条用例。
func TestFilePushValidation(t *testing.T) {
	validFile := []byte("print('hotfix')")
	validSha := sha256Hex(validFile)

	cases := []struct {
		name       string
		enable     bool
		transferID string
		fileName   string
		sha        string
		data       []byte
		errSubstr  string
	}{
		{"disabled flag", false, "t-1", "patch.lua", validSha, validFile, "file transfer is disabled"},
		{"empty transfer_id", true, "", "patch.lua", validSha, validFile, "transfer_id is required"},
		{"path traversal ../", true, "t-1", "../evil.lua", validSha, validFile, "bare basename"},
		{"absolute path", true, "t-1", "/etc/evil.lua", validSha, validFile, "bare basename"},
		{"subdir escape", true, "t-1", "sub/dir/evil.lua", validSha, validFile, "bare basename"},
		{"dotdot name rejected", true, "t-1", "..evil", validSha, validFile, "bare basename"},
		{"empty payload", true, "t-1", "patch.lua", validSha, nil, "file payload is empty"},
		{"oversize", true, "t-1", "patch.lua", validSha, make([]byte, 2048), "exceeds max"},
		{"missing sha", true, "t-1", "patch.lua", "", validFile, "content_sha256 is required"},
		{"checksum mismatch", true, "t-1", "patch.lua", strings.Repeat("ab", 32), validFile, "checksum mismatch"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newFilePushManager(t, tc.enable)
			req := &filePushRequest{
				transferID:    tc.transferID,
				fileName:      tc.fileName,
				contentSha256: tc.sha,
				data:          tc.data,
			}
			err := m.validateFilePush(req)
			if tc.errSubstr == "" {
				if err != nil {
					t.Fatalf("expected pass, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.errSubstr) {
				t.Fatalf("expected error containing %q, got %v", tc.errSubstr, err)
			}
		})
	}
}

// F：落盘行为——原子写 + 暂存目录落点。
func TestFilePushWritesToStaging(t *testing.T) {
	m := newFilePushManager(t, true)
	fileName := "hotfix.lua"
	data := []byte("print('hotfix')")
	body := encodeFilePushRequest(&filePushRequest{
		transferID:    "t-7",
		fileName:      fileName,
		contentSha256: sha256Hex(data),
		data:          data,
	})
	respBody, err := m.handleFilePushRequest(body)
	if err != nil {
		t.Fatalf("handleFilePushRequest: %v", err)
	}
	// 确认帧应包含 ok=true 与暂存路径
	resp := string(respBody)
	if !strings.Contains(resp, "hotfix.lua") {
		t.Fatalf("stored path missing in response: %x", respBody)
	}
	stored, readErr := os.ReadFile(filepath.Join(m.fileStagingDir(), fileName))
	if readErr != nil {
		t.Fatalf("staged file not found: %v", readErr)
	}
	if string(stored) != string(data) {
		t.Fatalf("staged content mismatch: %q", stored)
	}
}

// F：agent 帧回路——FilePushRequest 帧进、FilePushResponse 帧出。
func TestFilePushFrameRoundTrip(t *testing.T) {
	m := newFilePushManager(t, true)
	data := []byte("code-v2")
	body := encodeFilePushRequest(&filePushRequest{
		transferID:    "t-frame",
		fileName:      "code.lua",
		contentSha256: sha256Hex(data),
		data:          data,
	})
	wire := protocol.NewMessageBody(protocol.MsgProviderFilePushRequest, 42, body)

	_, msgID, reqID, payload, err := protocol.ParseMessageFromBody(wire)
	if err != nil {
		t.Fatalf("parse frame: %v", err)
	}
	if msgID != protocol.MsgProviderFilePushRequest {
		t.Fatalf("msgID = %x", msgID)
	}
	respBody, err := m.handleFilePushRequest(payload)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	respFrame := protocol.NewMessageBody(protocol.GetResponseMsgID(msgID), reqID, respBody)
	_, respMsgID, respReqID, respPayload, err := protocol.ParseMessageFromBody(respFrame)
	if err != nil {
		t.Fatalf("parse response frame: %v", err)
	}
	if respMsgID != protocol.MsgProviderFilePushResponse {
		t.Fatalf("response msgID = %x", respMsgID)
	}
	if respReqID != 42 {
		t.Fatalf("reqID = %d", respReqID)
	}
	if len(respPayload) == 0 {
		t.Fatal("empty response payload")
	}
}
