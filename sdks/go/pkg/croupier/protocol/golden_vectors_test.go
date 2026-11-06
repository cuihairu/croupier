package protocol

// CR-FIX-05 跨 SDK 行为级契约 golden 向量测试（Go SDK 面）。
// 读取 proto/contracts/golden_vectors.json（语言中立字节向量），断言本 SDK 的
// NewMessageBody 产出与向量 frame 逐字节一致，且 ParseMessageFromBody 能还原
// 出 (version, msgId, reqId, body)。任一 wire 语义漂移即失败。

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type goldenVec struct {
	MsgID string `json:"msgId"`
	ReqID string `json:"reqId"`
	Body  string `json:"body"`
	Frame string `json:"frame"`
}

type goldenFile struct {
	Version int                  `json:"version"`
	Vectors map[string]goldenVec `json:"vectors"`
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	// 从仓库根定位：本测试文件在 sdks/go/pkg/croupier/protocol/，向上 5 层到根。
	p := filepath.Join("..", "..", "..", "..", "..", "proto", "contracts", "golden_vectors.json")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read golden vectors: %v", err)
	}
	var gf goldenFile
	if err := json.Unmarshal(data, &gf); err != nil {
		t.Fatalf("parse golden vectors: %v", err)
	}
	return gf
}

func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

func TestGoldenVectors_WireFrames(t *testing.T) {
	gf := loadGolden(t)
	for name, vec := range gf.Vectors {
		t.Run(name, func(t *testing.T) {
			body := decodeHex(t, vec.Body)
			wantFrame := decodeHex(t, vec.Frame)

			mid, rid := msgIDFromVec(t, vec)
			got := NewMessageBody(mid, rid, body)
			if string(got) != string(wantFrame) {
				t.Errorf("NewMessageBody frame mismatch:\n got %s\nwant %s",
					hex.EncodeToString(got), hex.EncodeToString(wantFrame))
			}

			// 解码还原。
			version, gotMsgID, gotReqID, gotBody, err := ParseMessageFromBody(wantFrame)
			if err != nil {
				t.Fatalf("ParseMessageFromBody: %v", err)
			}
			if version != Version1 {
				t.Errorf("version = %d, want %d", version, Version1)
			}
			if gotMsgID != mid {
				t.Errorf("msgId = 0x%06x, want 0x%06x", gotMsgID, mid)
			}
			if gotReqID != rid {
				t.Errorf("reqId = %d, want %d", gotReqID, rid)
			}
			if string(gotBody) != string(body) {
				t.Errorf("body mismatch: got %x want %x", gotBody, body)
			}
		})
	}
}

func msgIDFromVec(t *testing.T, vec goldenVec) (uint32, uint32) {
	t.Helper()
	mid, err := hex.DecodeString(vec.MsgID)
	if err != nil {
		t.Fatalf("bad msgId: %v", err)
	}
	reqID, err := hex.DecodeString(vec.ReqID)
	if err != nil {
		t.Fatalf("bad reqId: %v", err)
	}
	msgID := uint32(mid[0])<<16 | uint32(mid[1])<<8 | uint32(mid[2])
	rid := uint32(reqID[0])<<24 | uint32(reqID[1])<<16 | uint32(reqID[2])<<8 | uint32(reqID[3])
	return msgID, rid
}
