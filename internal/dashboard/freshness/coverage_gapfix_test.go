package freshness

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

// digestRaw 中 marshalCanonical 失败时必须回退原始字节 sha256，
// 与非 JSON 输入的回退算法保持一致（确定性哨兵）。
func TestDigestRaw_MarshalCanonicalFailureFallback(t *testing.T) {
	raw := []byte(`{"a":1,"b":[1,2]}`)
	sum := sha256.Sum256(raw)
	want := hex.EncodeToString(sum[:])

	orig := marshalCanonical
	marshalCanonical = func(v interface{}) ([]byte, error) {
		return nil, errors.New("injected marshal failure")
	}
	t.Cleanup(func() { marshalCanonical = orig })

	if got := digestRaw(raw); got != want {
		t.Fatalf("digestRaw(marshal failure) = %q, want raw sha256 %q", got, want)
	}
	if got := CanonicalDigest(raw); got != want {
		t.Fatalf("CanonicalDigest(marshal failure) = %q, want raw sha256 %q", got, want)
	}
}

// 缝隙还原后正常 canonical 路径不受注入影响。
func TestDigestRaw_SeamRestoredAfterInjection(t *testing.T) {
	orig := marshalCanonical
	marshalCanonical = func(v interface{}) ([]byte, error) {
		return nil, errors.New("injected marshal failure")
	}
	t.Cleanup(func() { marshalCanonical = orig })

	digestRaw([]byte(`{}`))

	marshalCanonical = orig

	a := []byte(`{"x":1}`)
	b := []byte(`{ "x" : 1 }`)
	if CanonicalDigest(a) != CanonicalDigest(b) {
		t.Fatal("canonical digest must recover after seam restore")
	}
}
