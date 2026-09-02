package dispatch

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/transport"
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
		{"空串", map[string]string{"timeoutMs": "  "}, 0},
		{"垃圾值", map[string]string{"timeoutMs": "abc"}, 0},
		{"零", map[string]string{"timeoutMs": "0"}, 0},
		{"负值", map[string]string{"timeoutMs": "-5"}, 0},
		{"有效值", map[string]string{"timeoutMs": "2500"}, 2500 * time.Millisecond},
		{"低于 1s 提到 1s", map[string]string{"timeoutMs": "100"}, time.Second},
		{"高于 60s 截到 60s", map[string]string{"timeoutMs": "120000"}, 60 * time.Second},
		{"边界 1s", map[string]string{"timeoutMs": "1000"}, time.Second},
		{"边界 60s", map[string]string{"timeoutMs": "60000"}, 60 * time.Second},
		{"带空白", map[string]string{"timeoutMs": " 3000 "}, 3 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, requestTimeoutBudget(tc.meta))
		})
	}
}

// deadlineRecorder 记录 Call 收到的 ctx deadline。
var _ transport.SessionCaller = (*deadlineRecorder)(nil)

type deadlineRecorder struct {
	deadlineMs int64 // 距构造时刻的剩余毫秒；0 = 无 deadline
	has        bool
}

func (r *deadlineRecorder) Call(ctx context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	d, ok := ctx.Deadline()
	r.has = ok
	if ok {
		r.deadlineMs = time.Until(d).Milliseconds()
	}
	return msgID, nil, nil
}

// 声明预算是权威值：ctx 已带预算（30s）时，callAgent 不得用全局默认
// （15s）把它收紧——声明 30s 契约应真实生效。
func TestCallAgentRespectsDeclaredBudget(t *testing.T) {
	rec := &deadlineRecorder{}
	d := NewDispatcher(nil)
	d.SetSessionResolver(&fakeSessionResolver{callers: map[string]transport.SessionCaller{"a1": rec}})

	// 1) 无声明预算 → 全局默认 deadline 生效
	ctx := context.Background()
	_, err := d.callAgent(ctx, "a1", 1, nil)
	assert.NoError(t, err)
	assert.True(t, rec.has)
	assert.InDelta(t, 15000, rec.deadlineMs, 1500, "default budget should be ~15s")

	// 2) 声明 30s 预算 → 不被 15s 默认截断
	bCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = d.callAgent(bCtx, "a1", 1, nil)
	assert.NoError(t, err)
	assert.True(t, rec.has)
	assert.InDelta(t, 30000, rec.deadlineMs, 1500, "declared budget must not be tightened by the default")
}
