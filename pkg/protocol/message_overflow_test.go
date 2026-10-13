//go:build !race

package protocol

import (
	"math"
	"testing"
	"unsafe"
)

// 溢出分支：伪造一个长度溢出 HeaderSize+len(data) 的 slice 头（不触碰、
// 不分配其背内存），验证 NewMessageBody 拒绝而非越界读写。
// 该用例与 -race 互斥：-race 隐含 checkptr，对跨界伪造切片直接
// fatal error: unsafe.Slice result straddles multiple allocations，
// 进程在被测函数执行前即中止。构建标签无运行期等价物，故按标签隔离；
// checkptr 亦可经 -gcflags=all=-d=checkptr 单独开启，同一约束适用。
func TestNewMessageBodyOverflowReturnsNil(t *testing.T) {
	backing := make([]byte, 1)
	forged := unsafe.Slice(unsafe.SliceData(backing), math.MaxInt-7)
	if got := NewMessageBody(MsgInvokeRequest, 1, forged); got != nil {
		t.Fatal("overflowing body should be nil")
	}
}
