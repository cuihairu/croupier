package dispatch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// requestTimeoutBudget：metadata timeout_ms 约定的解析与 clamp 语义。
func TestRequestTimeoutBudget(t *testing.T) {
	cases := []struct {
		name string
		meta map[string]string
		want time.Duration
	}{
		{"缺失", nil, 0},
		{"空串", map[string]string{"timeout_ms": "  "}, 0},
		{"垃圾值", map[string]string{"timeout_ms": "abc"}, 0},
		{"零", map[string]string{"timeout_ms": "0"}, 0},
		{"负值", map[string]string{"timeout_ms": "-5"}, 0},
		{"有效值", map[string]string{"timeout_ms": "2500"}, 2500 * time.Millisecond},
		{"低于 1s 提到 1s", map[string]string{"timeout_ms": "100"}, time.Second},
		{"高于 60s 截到 60s", map[string]string{"timeout_ms": "120000"}, 60 * time.Second},
		{"边界 1s", map[string]string{"timeout_ms": "1000"}, time.Second},
		{"边界 60s", map[string]string{"timeout_ms": "60000"}, 60 * time.Second},
		{"带空白", map[string]string{"timeout_ms": " 3000 "}, 3 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, requestTimeoutBudget(tc.meta))
		})
	}
}
