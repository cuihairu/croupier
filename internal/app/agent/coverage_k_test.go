package agent

// coverage_k_test.go 补齐本包剩余未覆盖分支：
//   - ProviderManager.Close 汇报 provider 关闭错误（注入 fake provider）
//   - App.Stop 中 telemetry.Shutdown 失败告警（二次 Shutdown 触发 reader 已关闭）
//   - invokeExternalPlatformFunction proto 模式 Marshal 失败（非法 UTF-8 错误串）
//   - discoverExternalPlatformFunctions 对碰撞 FunctionID 的去重
//   - MetricsCollector.collectDisks / collectNetworks 的 gopsutil 错误分支
//     （HOST_PROC 指向假 /proc，注入缺失文件与不存在挂载点）
//   - updateLoop 去抖定时器已触发时的排水分支
//   - heartbeatLoop 恢复后 re-register 失败告警
//   - syncOnce 组装请求期间客户端被置空（dynamicLabels 钩子翻转 client）

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/cuihairu/croupier/internal/telemetry"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	externalv1 "github.com/cuihairu/croupier/pkg/pb/croupier/external/v1"
)

// --- provider.go: Close 汇报 provider 关闭错误 ---

type closeFailProviderK struct {
	closeErr error
}

func (p *closeFailProviderK) Name() string                                         { return "close-fail-k" }
func (p *closeFailProviderK) Init(context.Context, provider.ProviderConfig) error  { return nil }
func (p *closeFailProviderK) IsEnabled() bool                                      { return true }
func (p *closeFailProviderK) SupportedMethods() []string                           { return nil }
func (p *closeFailProviderK) Call(context.Context, string, []byte) ([]byte, error) { return nil, nil }
func (p *closeFailProviderK) Close() error                                         { return p.closeErr }

func TestProviderManagerClosePropagatesProviderError_K(t *testing.T) {
	m := NewProviderManager(agentlocal.NewLocalStore(), t.TempDir(), nil)
	m.providers["boom-k"] = &closeFailProviderK{closeErr: errors.New("close failed k")}
	m.providers["silent-k"] = &closeFailProviderK{}

	err := m.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close failed k")
	assert.Empty(t, m.providers, "Close should drop all provider entries")

	// 二次 Close：providers 已清空，返回 nil。
	assert.NoError(t, m.Close())
}

// --- app.go: Stop 中 telemetry 关闭失败仅告警 ---

func TestAppStopTelemetryShutdownFailureLogged_K(t *testing.T) {
	svc, err := telemetry.NewGameTelemetryService(telemetry.TelemetryConfig{
		ServiceName:   "svc-k",
		CollectorURL:  "http://127.0.0.1:1",
		EnableTracing: true,
		EnableMetrics: true,
	}, nil)
	require.NoError(t, err)

	// 首次 Shutdown 正常完成；第二次（App.Stop 内）会命中
	// meter reader 已关闭的错误路径——App.Stop 只记日志不返回错误。
	_ = svc.Shutdown(context.Background())

	a := New("", "agent-k-telemetry")
	a.WithTelemetry(svc)
	a.Stop()
}

// --- extension_external_bridge.go: proto 模式响应 Marshal 失败 ---

func TestInvokeExternalPlatformFunctionProtoModeMarshalFailure_K(t *testing.T) {
	raw, err := proto.Marshal(&externalv1.CallPlatformRequest{
		Platform: "quicksdk",
		Method:   "day_report",
	})
	require.NoError(t, err)

	// proto3 string 字段含非法 UTF-8 时 proto.Marshal 报错，
	// 以此触发 proto 模式响应序列化失败分支。
	resp, handled, err := invokeExternalPlatformFunction(context.Background(), "external.quicksdk.day_report", raw,
		func(context.Context, string, string, []byte) ([]byte, error) {
			return nil, errors.New(string([]byte{0xff, 0xfe, 0xfd}))
		})
	assert.Nil(t, resp)
	assert.True(t, handled, "external function id should be handled")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UTF-8")
}

// --- app.go: discoverExternalPlatformFunctions 对碰撞 FunctionID 去重 ---

func TestDiscoverExternalPlatformFunctionsDeduplicatesCollidingIDs_K(t *testing.T) {
	// provider "alpha" 的操作 "beta.gamma" 与 provider "alpha.beta" 的操作
	// "gamma" 生成相同 FunctionID（SanitizeKey 允许点号）→ 第二个被 seen 去重。
	item := RuntimeInstallation{
		ExtensionID:    "croupier.external-platform",
		ReleaseVersion: "1.0.0",
		Bindings: []RuntimeBinding{
			{
				BindingType: "provider",
				Spec: map[string]any{
					"provider":   "alpha",
					"operations": []any{"beta.gamma"},
				},
			},
			{
				BindingType: "provider",
				Spec: map[string]any{
					"provider":   "alpha.beta",
					"operations": []any{"gamma"},
				},
			},
		},
	}

	out := discoverExternalPlatformFunctions(item)
	require.Len(t, out, 1, "colliding function IDs must be deduplicated")
	assert.Equal(t, "external.alpha.beta.gamma", out[0].FunctionID)
	// DiscoverProviderOperations 返回 map，provider 遍历顺序随机——去重保留
	// 哪个 provider 不确定，只断言胜者是两个碰撞源之一（组合自洽）。
	if out[0].Provider == "alpha" {
		assert.Equal(t, "beta.gamma", out[0].Operation)
	} else {
		assert.Equal(t, "alpha.beta", out[0].Provider)
		assert.Equal(t, "gamma", out[0].Operation)
	}
}

// --- ops_metrics.go: gopsutil 错误分支（HOST_PROC 指向假 /proc） ---

func TestMetricsCollectorDisksPartitionsError_K(t *testing.T) {
	// HOST_PROC 指向空目录：mountinfo/mounts 均缺失 → Partitions 报错提前返回。
	t.Setenv("HOST_PROC", t.TempDir())

	c := NewMetricsCollector("agent-k-disk-err")
	assert.Empty(t, c.collectDisks())
}

func TestMetricsCollectorDisksUsageErrorSkipsMount_K(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "proc")
	pidDir := filepath.Join(procDir, "1")
	require.NoError(t, os.MkdirAll(pidDir, 0o755))

	// filesystems 文件必须存在（all=false 时缺失会直接报错）。
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "filesystems"), []byte("ext4\n"), 0o644))

	realMount := t.TempDir()
	missingMount := filepath.Join(root, "definitely-missing-mount-k")
	require.NoError(t, os.WriteFile(filepath.Join(pidDir, "mountinfo"), []byte(fmt.Sprintf(
		"36 35 98:0 / %s rw,relatime - ext4 /dev/root rw\n"+
			"37 36 99:1 / %s rw,relatime - ext4 /dev/root rw\n",
		missingMount, realMount,
	)), 0o644))

	t.Setenv("HOST_PROC", procDir)

	c := NewMetricsCollector("agent-k-disk-usage")
	disks := c.collectDisks()
	require.Len(t, disks, 1, "unstatable mountpoint must be skipped, real one kept")
	assert.Equal(t, realMount, disks[0].MountPoint)
}

func TestMetricsCollectorNetworksIOCountersError_K(t *testing.T) {
	// HOST_PROC 指向空目录：net/dev 缺失 → IOCounters 报错提前返回。
	t.Setenv("HOST_PROC", t.TempDir())

	c := NewMetricsCollector("agent-k-net-err")
	assert.Empty(t, c.collectNetworks())
}

// --- upstream.go: 心跳恢复后 re-register 失败 ---

type recoveryControlClientK struct {
	mu          sync.Mutex
	connected   bool
	failFirstHB int
	heartbeats  int
	registers   int
}

func (m *recoveryControlClientK) Connected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

func (m *recoveryControlClientK) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	return nil
}

func (m *recoveryControlClientK) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	m.mu.Lock()
	m.registers++
	m.mu.Unlock()
	return nil, errors.New("register rejected k")
}

func (m *recoveryControlClientK) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	m.mu.Lock()
	seen := m.heartbeats
	m.heartbeats++
	m.mu.Unlock()
	if seen < m.failFirstHB {
		return nil, errors.New("transient heartbeat failure k")
	}
	return &agentv1.HeartbeatResponse{}, nil
}

func (m *recoveryControlClientK) RegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	return &agentv1.RegisterCapabilitiesResponse{}, nil
}

func (m *recoveryControlClientK) SendTaskEvent(ctx context.Context, body []byte) error   { return nil }
func (m *recoveryControlClientK) SendMetricEvent(ctx context.Context, body []byte) error { return nil }

func (m *recoveryControlClientK) counts() (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.heartbeats, m.registers
}

func TestUpstreamHeartbeatLoopRecoveryReRegisterFails_K(t *testing.T) {
	// 心跳 #1 失败（连续失败计数 1 < 阈值 2，不触发重拨），
	// 心跳 #2 成功 → 走“恢复后重新注册”分支；Register 永远失败 →
	// re-register 失败告警。心跳 #3 成功且计数已归零（不再注册），
	// 由于 heartbeatLoop 单线程顺序处理，观测到 #3 即证明 #2 分支（含
	// syncWithRetry 200ms 退避与告警）已完整执行。
	fake := &recoveryControlClientK{connected: true, failFirstHB: 1}
	client := NewUpstreamClient("mock", "agent-k-hb", agentlocal.NewLocalStore(), nil)
	client.setClient(fake)
	client.WithMetadata(UpstreamMetadata{HeartbeatInterval: 20 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.heartbeatLoop(ctx)

	require.Eventually(t, func() bool {
		heartbeats, registers := fake.counts()
		return heartbeats >= 3 && registers >= 1
	}, 5*time.Second, 20*time.Millisecond, "recovery re-register attempt must run")
}

// --- upstream.go: syncOnce 组装期间客户端被置空 ---

func TestUpstreamSyncOnceClientClearedWhileComposingLabels_K(t *testing.T) {
	client := NewUpstreamClient("mock", "agent-k-nilflip", agentlocal.NewLocalStore(), nil)
	client.setClient(&mockControlClientV9{connected: true})

	// composeLabels 在两次 currentClient() 之间同步执行：
	// 借 dynamicLabels 钩子翻转 client，覆盖第二次 nil 检查。
	client.SetDynamicLabelsProvider(func() map[string]string {
		client.setClient(nil)
		return nil
	})

	err := client.syncOnce(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// --- upstream.go: updateLoop 排水已触发的去抖定时器 ---

func TestUpstreamUpdateLoopDrainsFiredTimer_K(t *testing.T) {
	store := agentlocal.NewLocalStore()
	client := NewUpstreamClient("mock", "agent-k-drain", store, nil)
	mock := &mockControlClientV9{connected: true}
	client.setClient(mock)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client.updateCh = make(chan struct{}, 128)
	// debounce 必须远小于灌消息间隔：若 debounce 大于消息间隔，去抖
	// 语义会让 timer 在每次触发前被 Reset，永不 fire，sync 永不发生。
	// 用亚纳秒 debounce 让 Reset 后 timer.C 立即就绪，select 在 updateCh
	// 与已触发的 timer.C 之间均匀随机挑选；选中 updateCh 时 Stop() 返回
	// false 进入排水分支。每轮 50%，数千轮内命中是必然事件。
	go client.updateLoop(ctx, time.Nanosecond)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case client.updateCh <- struct{}{}:
		default:
		}
		time.Sleep(100 * time.Microsecond)
	}
	// 停止生产后 updateCh 清空，已 Reset 的 timer 到期，select 必走
	// timer.C 分支完成一次 sync，使 registers 非零成为确定性结果。
	time.Sleep(100 * time.Millisecond)
	cancel()

	mock.mu.Lock()
	registers := len(mock.registers)
	mock.mu.Unlock()
	assert.NotZero(t, registers, "updateLoop must have run debounced syncs")
}
