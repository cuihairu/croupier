package metrics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// --- gopsutil 错误分支（HOST_PROC 指向假 /proc，随上收自 sidecar 迁入） ---

func TestCollectDisksPartitionsError(t *testing.T) {
	// HOST_PROC 指向空目录：mountinfo/mounts 均缺失 → Partitions 报错提前返回。
	t.Setenv("HOST_PROC", t.TempDir())

	c := NewCollector("agent-disk-err")
	assert.Empty(t, c.collectDisks())
}

func TestCollectDisksUsageErrorSkipsMount(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "proc")
	pidDir := filepath.Join(procDir, "1")
	require.NoError(t, os.MkdirAll(pidDir, 0o755))

	// filesystems 文件必须存在（all=false 时缺失会直接报错）。
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "filesystems"), []byte("ext4\n"), 0o644))

	realMount := t.TempDir()
	missingMount := filepath.Join(root, "definitely-missing-mount")
	require.NoError(t, os.WriteFile(filepath.Join(pidDir, "mountinfo"), []byte(fmt.Sprintf(
		"36 35 98:0 / %s rw,relatime - ext4 /dev/root rw\n"+
			"37 36 99:1 / %s rw,relatime - ext4 /dev/root rw\n",
		missingMount, realMount,
	)), 0o644))

	t.Setenv("HOST_PROC", procDir)

	c := NewCollector("agent-disk-usage")
	disks := c.collectDisks()
	require.Len(t, disks, 1, "unstatable mountpoint must be skipped, real one kept")
	assert.Equal(t, realMount, disks[0].MountPoint)
}

func TestCollectNetworksIOCountersError(t *testing.T) {
	// HOST_PROC 指向空目录：net/dev 缺失 → IOCounters 报错提前返回。
	t.Setenv("HOST_PROC", t.TempDir())

	c := NewCollector("agent-net-err")
	assert.Empty(t, c.collectNetworks())
}

// --- 采集与上报循环 ---

func TestCollectBasic(t *testing.T) {
	c := NewCollector("agent-collect")
	report := c.Collect(context.Background())
	require.NotNil(t, report)
	assert.Equal(t, "agent-collect", report.AgentId)
	assert.NotZero(t, report.Timestamp)
	assert.NotEmpty(t, report.Custom)
	assert.Contains(t, report.Custom, "go_num_goroutines")
	// 无采样器时不产生受管进程字段
	assert.Empty(t, report.SupervisedProcesses)
}

func TestStartReportingImmediateAndPeriodic(t *testing.T) {
	c := NewCollector("agent-mc")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reports := make(chan *opsv1.MetricsReport, 8)
	go func() {
		c.StartReporting(ctx, 50*time.Millisecond, func(r *opsv1.MetricsReport) {
			reports <- r
		})
	}()

	select {
	case r := <-reports:
		require.NotNil(t, r)
		assert.Equal(t, "agent-mc", r.AgentId)
	case <-time.After(3 * time.Second):
		t.Fatal("no immediate report received")
	}

	select {
	case r := <-reports:
		require.NotNil(t, r)
	case <-time.After(3 * time.Second):
		t.Fatal("no periodic report received")
	}
}

type fakeSampler struct {
	snaps  []*opsv1.SupervisedProcessSnapshot
	events []*opsv1.SupervisorEvent
}

func (f *fakeSampler) SampleSupervisedProcesses() []*opsv1.SupervisedProcessSnapshot {
	return f.snaps
}

func (f *fakeSampler) SupervisorEventsSince(lastSeq int64) []*opsv1.SupervisorEvent {
	var out []*opsv1.SupervisorEvent
	for _, e := range f.events {
		if e.Seq > lastSeq {
			out = append(out, e)
		}
	}
	return out
}

func TestCollectFoldsSamplerSnapshotAndEvents(t *testing.T) {
	s := &fakeSampler{
		snaps:  []*opsv1.SupervisedProcessSnapshot{{Name: "sleeper"}},
		events: []*opsv1.SupervisorEvent{{Seq: 7}},
	}
	c := NewCollector("agent-1").WithSampler(s)
	report := c.Collect(context.Background())
	require.NotEmpty(t, report.SupervisedProcesses)
	assert.Equal(t, "sleeper", report.SupervisedProcesses[0].Name)
	require.Len(t, report.SupervisorEvents, 1)
	assert.Equal(t, int64(7), report.SupervisorEvents[0].Seq)
	assert.Equal(t, int64(7), c.lastSamplerSeq()) // 游标推进

	// 游标之后无新事件：不再携带
	report2 := c.Collect(context.Background())
	assert.Empty(t, report2.SupervisorEvents)
}

func TestNilCollectorSafe(t *testing.T) {
	var c *Collector
	assert.Nil(t, c.WithSampler(nil))
	// nil 接收者全部为安全 no-op：读游标 0，写游标不 panic 也不生效。
	assert.Equal(t, int64(0), c.lastSamplerSeq())
	c.setLastSamplerSeq(3)
	assert.Equal(t, int64(0), c.lastSamplerSeq())
}
