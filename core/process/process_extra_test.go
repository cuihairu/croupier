package process

// cron.go / lifecycle.go / lifecycle_unix.go / supervisor.go 补测：parseCronPart
// 的区间上下界非数字臂、Next 的 5 年窗口耗尽零值臂（永不命中的 cron，如
// 2 月 30 日）、NextSchedule 对零值 next 的 continue 臂；ExitDetail 的信号
// 臂（SIGKILL 子进程）与非零退出码臂（exit 3 子进程）、IsOOMSuspect 的
// 内存查询失败臂（HOST_PROC 指向空目录令 gopsutil 读 /meminfo 失败）、
// exitSignal 的 Signaled 命中与回落臂；SupervisorState 的 Config 访问器与
// CancelBackoff 清除臂。
//
// 诚实跳过：parseCronPart 的 len(bounds) != 2 臂（cron.go:82-83）不可达——
// 进入该分支前置条件是 rangePart 含 "-"，而 strings.SplitN(rangePart, "-", 2)
// 对含 "-" 的输入恒返回恰好 2 段，长度检查恒假（防御性死代码）。

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseCronFieldRangeBoundErrors(t *testing.T) {
	// 区间下界非数字
	_, err := ParseCron("a-5 * * * *")
	require.ErrorContains(t, err, "field 1")
	require.ErrorContains(t, err, `invalid range "a-5"`)

	// 区间上界非数字
	_, err = ParseCron("5-b * * * *")
	require.ErrorContains(t, err, "field 1")
	require.ErrorContains(t, err, `invalid range "5-b"`)
}

func TestCronNextExhaustsWindowOnNeverMatchingSpec(t *testing.T) {
	// 2 月 30 日永不出现 → Next 扫完 5 年窗口返回零值
	spec, err := ParseCron("0 0 30 2 *")
	require.NoError(t, err)
	require.False(t, spec.Matches(time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)))
	require.True(t, spec.Next(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)).IsZero())
}

func TestNextScheduleSkipsNeverMatchingEntry(t *testing.T) {
	never, err := ParseCron("0 0 30 2 *")
	require.NoError(t, err)
	every5, err := ParseCron("*/5 * * * *")
	require.NoError(t, err)

	now := time.Date(2026, 1, 1, 0, 3, 0, 0, time.UTC)
	// 零值 next 的条目在首位 → continue 臂，被后者取代
	best, action := NextSchedule([]ScheduleEntry{
		{Action: "never", Spec: never},
		{Action: "every5", Spec: every5},
	}, now)
	require.Equal(t, now.Add(2*time.Minute), best)
	require.Equal(t, "every5", action)

	// 全部条目永不命中 → 零值 + 空 action
	best, action = NextSchedule([]ScheduleEntry{{Action: "never", Spec: never}}, now)
	require.True(t, best.IsZero())
	require.Empty(t, action)
}

func TestExitDetailSignalAndExitCodeArms(t *testing.T) {
	// 信号终止：SIGKILL 子进程 → Code -1 + 信号名
	cmd := exec.Command("sleep", "30")
	require.NoError(t, cmd.Start())
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, cmd.Process.Kill())
	st := ExitDetail(cmd.Wait())
	require.Equal(t, int32(-1), st.Code)
	require.NotEmpty(t, st.Signal)

	// 非零退出码：Code 为退出码、无信号
	cmd2 := exec.Command("sh", "-c", "exit 3")
	st = ExitDetail(cmd2.Run())
	require.Equal(t, int32(3), st.Code)
	require.Empty(t, st.Signal)
}

func TestIsOOMSuspectQueryFailure(t *testing.T) {
	// HOST_PROC 指向空目录 → gopsutil 读 /meminfo 失败 → 查询失败臂返回 false
	t.Setenv("HOST_PROC", t.TempDir())
	require.False(t, IsOOMSuspect("killed", 1<<30))
}

func TestSupervisorStateConfigAccessorAndCancelBackoff(t *testing.T) {
	cfg := RestartConfig{InitialBackoff: time.Second, MaxBackoff: time.Second, BreakerLimit: 3}
	s := NewSupervisorState(cfg)
	require.Equal(t, cfg, s.Config())

	now := time.Now()
	d := s.DecideExit(true, now)
	require.Equal(t, SupervisorBackoff, d.Action)
	require.Equal(t, now.Add(time.Second), s.NextRestartAt())

	// CancelBackoff 清除待重启时间 → 到期判定为 false
	s.CancelBackoff()
	require.True(t, s.NextRestartAt().IsZero())
	require.False(t, s.ConsumeBackoff(now.Add(2*time.Second)))
}
