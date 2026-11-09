package healthprobe

// healthprobe 补测（跨文件零块）：icmp.go 的 DNS 解析失败臂（用非 IP 且
// 非法字面量的主机名确定性触发——本环境 DNS 通配解析 .invalid，原
// no-such-host.invalid 守卫在本机被跳过）、timeline.go 的写失败臂
// （/dev/full 恒 ENOSPC）。
//
// 诚实跳过（均为不可达防御臂）：icmp.go 的 message.Marshal 错误臂
// （116-117，请求体恒为 &icmp.Echo{...} 合法结构，Marshal 不可失败）、
// timeline.go 的 json.Marshal 错误臂（45-47，WindowEvent 全为 string/
// int64 字段，序列化恒成功）、process_unix.go 的 FindProcess 错误臂
// （24-25，unix 的 findProcess 恒构造 &Process{pid} 返回 nil 错误，
// 实测 -1 也走 Signal(0) 的 already-released 臂而非此处）。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 非法主机名字面量（非 IP 且含非法字符）→ ResolveIPAddr 确定性失败，
// 不依赖 DNS 通配解析行为。
func TestICMPCheckDNSFailureDeterministic(t *testing.T) {
	err := NewICMPChecker("999.999.999.999", time.Second, 1).Check(context.Background())
	require.ErrorContains(t, err, "dns:")
}

// /dev/full 写恒 ENOSPC → Write 错误透传（timeline 48-50）。
func TestFileTimelineWriteFailure(t *testing.T) {
	tl, err := OpenFileTimeline("/dev/full")
	require.NoError(t, err)
	t.Cleanup(func() { _ = tl.Close() })

	err = tl.AppendWindow(WindowEvent{Kind: "unavailable", Window: Window{Target: "x"}})
	require.Error(t, err)
	require.False(t, errors.Is(err, nil), "write must fail, not succeed")
}
