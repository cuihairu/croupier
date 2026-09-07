package freshness

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// digestRawBytes 对空输入必须返回空串（旧快照从未写入 digest 的哨兵值）。
func TestDigestRawBytes_EmptyInput(t *testing.T) {
	if got := digestRawBytes(nil); got != "" {
		t.Fatalf("digestRawBytes(nil) = %q, want empty", got)
	}
	if got := digestRawBytes([]byte{}); got != "" {
		t.Fatalf("digestRawBytes(empty) = %q, want empty", got)
	}
}

// 非 JSON 输入回退原始字节哈希，与 digestRawBytes 保持同一算法。
func TestDigestRawBytes_NonJSONFallback(t *testing.T) {
	raw := []byte("not-a-json")
	sum := sha256.Sum256(raw)
	want := hex.EncodeToString(sum[:])
	if got := digestRawBytes(raw); got != want {
		t.Fatalf("digestRawBytes(non-json) = %q, want %q", got, want)
	}
}

// stored digest 为空（旧快照无 digest 字段）或纯空白时必须视为兼容。
func TestDigestMatch_EmptyStored(t *testing.T) {
	schema := []byte(`{"type":"object"}`)
	if !digestMatch(schema, "") {
		t.Fatal("empty stored digest must be treated as compatible")
	}
	if !digestMatch(schema, "   \t ") {
		t.Fatal("whitespace-only stored digest must be treated as compatible")
	}
	if !digestMatch(nil, "") {
		t.Fatal("empty schema with empty stored digest must be compatible")
	}
}

// CanonicalDigest 是发布端共用的公开入口，与内部 digestRaw 一致。
func TestCanonicalDigest_PublicHelper(t *testing.T) {
	a := []byte(`{"b":1,"a":2}`)
	b := []byte(`{ "a" : 2 , "b" : 1 }`)
	if CanonicalDigest(a) != CanonicalDigest(b) {
		t.Fatal("CanonicalDigest must be key-order independent")
	}
	if CanonicalDigest(nil) != "" || CanonicalDigest([]byte{}) != "" {
		t.Fatal("CanonicalDigest of empty input must be empty")
	}
	if CanonicalDigest([]byte("plain")) == "" {
		t.Fatal("CanonicalDigest of non-JSON must fall back to raw hash")
	}
}
