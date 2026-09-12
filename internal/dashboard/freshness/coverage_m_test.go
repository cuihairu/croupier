package freshness

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// MatchesDigest 是 sync-selectors 验证 prev schema 快照一致性的公开入口：
// canonical / 原始字节双算法任一命中即一致；空 digest 视为兼容（旧快照）。
func TestMatchesDigestWrapper(t *testing.T) {
	schema := []byte(`{"b":1,"a":2}`)

	// canonical digest：键序不同的同一 schema digest 恒定
	if !MatchesDigest(schema, CanonicalDigest([]byte(`{"a":2,"b":1}`))) {
		t.Fatal("canonical digest of the same schema must match")
	}

	// 旧快照的原始字节 digest（发布时存的算法）
	rawSum := sha256.Sum256(schema)
	if !MatchesDigest(schema, hex.EncodeToString(rawSum[:])) {
		t.Fatal("legacy raw-bytes digest must match")
	}

	// 空 / 纯空白 stored：旧快照无 digest，视为兼容
	if !MatchesDigest(schema, "") || !MatchesDigest(schema, "  \t ") {
		t.Fatal("empty stored digest must be treated as compatible")
	}

	// 不同 schema 的 digest：不一致
	otherSum := sha256.Sum256([]byte(`{"b":2,"a":1}`))
	if MatchesDigest(schema, hex.EncodeToString(otherSum[:])) {
		t.Fatal("digest of a different schema must not match")
	}
	if MatchesDigest(schema, CanonicalDigest([]byte(`{"b":2,"a":1}`))) {
		t.Fatal("canonical digest of a different schema must not match")
	}

	// 非 JSON 输入回退原始字节哈希
	plain := []byte("not-a-json")
	plainSum := sha256.Sum256(plain)
	if !MatchesDigest(plain, hex.EncodeToString(plainSum[:])) {
		t.Fatal("non-JSON schema must fall back to raw-bytes digest")
	}
}
