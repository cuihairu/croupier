package keymanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGroupN_PKCS7PadOverflowGuard 覆盖 pkcs7Pad 的溢出守卫分支：
// blockSize 为负数时（blockSize=-16, len(data)=5）padding=-21、
// totalLen=-16 < len(data)，无需巨大输入即可触发防御性返回原数据。
func TestGroupN_PKCS7PadOverflowGuard(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	got := pkcs7Pad(data, -16)
	assert.Equal(t, data, got, "overflow guard must return the original data")
}
