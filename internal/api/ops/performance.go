package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// PerformanceSnapshotResponse GET /ops/performance 响应：生效性能参数（L2∧L3
// 合成 + 逐键来源）+ Go 运行时统计 + 宿主机 CPU/内存/磁盘占用 + 阈值超限注记。
type PerformanceSnapshotResponse struct {
	Settings settings.PerformanceSettingsSnapshot `json:"settings"`
	Runtime  PerformanceRuntime                   `json:"runtime"`
	Host     PerformanceHost                      `json:"host"`
	// Overload 当前占用是否越过对应阈值（阈值 0 = 不启用该判定，恒 false）
	Overload PerformanceOverload `json:"overload"`
}

// PerformanceRuntime Go 进程运行时统计（OPEN-ISSUES #53）。
type PerformanceRuntime struct {
	GoMaxProcs     int     `json:"goMaxProcs"`     // GOMAXPROCS（当前调度并行度）
	Goroutines     int     `json:"goroutines"`     // 存活 goroutine 数
	HeapAllocBytes uint64  `json:"heapAllocBytes"` // 堆存活对象字节
	HeapSysBytes   uint64  `json:"heapSysBytes"`   // 堆向 OS 申请的字节
	SysBytes       uint64  `json:"sysBytes"`       // 进程总内存（runtime.MemStats.Sys）
	NumGC          uint32  `json:"numGC"`          // 累计 GC 次数
	GCPauseMs      float64 `json:"gcPauseMs"`      // 累计 GC STW 毫秒
	UptimeSeconds  int64   `json:"uptimeSeconds"`  // 进程在线时长（同 /ops/system/info）
}

// PerformanceHost 宿主机资源占用（gopsutil，进程外视角）。
type PerformanceHost struct {
	CPUPercent       float64 `json:"cpuPercent"`       // 自上次采样以来的 CPU 占用 %
	MemoryUsedPct    float64 `json:"memoryUsedPct"`    // 物理内存占用 %
	MemoryTotalBytes uint64  `json:"memoryTotalBytes"` // 物理内存总量
	MemoryUsedBytes  uint64  `json:"memoryUsedBytes"`  // 物理内存已用
	DiskPath         string  `json:"diskPath"`         // 采样目录（进程工作目录所在盘）
	DiskUsedPct      float64 `json:"diskUsedPct"`      // 磁盘占用 %
	DiskTotalBytes   uint64  `json:"diskTotalBytes"`
	DiskUsedBytes    uint64  `json:"diskUsedBytes"`
}

// PerformanceOverload 阈值超限判定（settings 侧 0 = 不过滤则恒 false）。
type PerformanceOverload struct {
	CPU    bool `json:"cpu"`
	Memory bool `json:"memory"`
	Disk   bool `json:"disk"`
}

// PerformanceGet GET /api/v1/ops/performance —— 性能参数生效值 + 运行时快照。
//
// 边界（OPEN-ISSUES #53，诚实）：本端点是**读侧快照**——perf.* 六键在本批
// 不挂请求拦截/自动限流（超限仅在此注记，不改变请求行为）；GOMAXPROCS 不随
// perf.maxThreadCount 热改（全局调度变更风险大，另行评估）；perf.cacheSize
// 本批仅存储回显（internal/cache 尚无容量上限语义可接）。
func (h *Handler) PerformanceGet(c *gin.Context) {
	response.Success(c, buildPerformanceSnapshot(h.service.svcCtx))
}

// PerformanceUpdate 写 L3 覆盖并热重载，返回更新后快照。
func (s *Service) PerformanceUpdate(ctx context.Context, req map[string]jsonNumber, updatedBy string) (PerformanceSnapshotResponse, error) {
	if s == nil || s.svcCtx == nil || s.svcCtx.PlatformSettingModel == nil {
		return PerformanceSnapshotResponse{}, errors.New("settings store unavailable")
	}
	valid := perfWritableKeys()
	for key, num := range req {
		if !valid[key] {
			return PerformanceSnapshotResponse{}, &errorx.CodeError{
				Code:       http.StatusBadRequest,
				Message:    fmt.Sprintf("未知或不可写的性能参数键：%s", key),
				StableCode: "invalid_setting_key",
			}
		}
		if num.Int64() < 0 {
			return PerformanceSnapshotResponse{}, &errorx.CodeError{
				Code:       http.StatusBadRequest,
				Message:    fmt.Sprintf("%s 不能为负数", key),
				StableCode: "validation_failed",
			}
		}
		if perfPctKeys[key] && num.Int64() > 100 {
			return PerformanceSnapshotResponse{}, &errorx.CodeError{
				Code:       http.StatusBadRequest,
				Message:    fmt.Sprintf("%s 为百分比，取值 0-100", key),
				StableCode: "validation_failed",
			}
		}
	}
	store := s.svcCtx.PlatformSettingModel
	for key, num := range req {
		if err := store.Set(ctx, key, num.Raw(), updatedBy); err != nil {
			return PerformanceSnapshotResponse{}, err
		}
	}
	settings.Current().Reload(ctx, store)
	return buildPerformanceSnapshot(s.svcCtx), nil
}

// perfWritableKeys 性能参数白名单（与 ValidKeys 的 perf.* 子集一致）。
func perfWritableKeys() map[string]bool {
	return map[string]bool{
		settings.KeyPerfMaxCpuPct:      true,
		settings.KeyPerfMaxMemoryPct:   true,
		settings.KeyPerfMaxDiskPct:     true,
		settings.KeyPerfMaxConcurrent:  true,
		settings.KeyPerfMaxThreadCount: true,
		settings.KeyPerfCacheSize:      true,
	}
}

// perfPctKeys 百分比语义键（0-100 校验用）。
var perfPctKeys = map[string]bool{
	settings.KeyPerfMaxCpuPct:    true,
	settings.KeyPerfMaxMemoryPct: true,
	settings.KeyPerfMaxDiskPct:   true,
}

// PerformancePut PUT /api/v1/ops/performance —— 逐键写 perf.* L3 覆盖。
// 只收六键；负数/百分比越界 400。
func (h *Handler) PerformancePut(c *gin.Context) {
	var req map[string]jsonNumber
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	snap, err := h.service.PerformanceUpdate(c.Request.Context(), req, perfUsername(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, snap)
}

// perfStartedAt 进程启动时刻兜底（svcCtx.StartTime 不可达时用，秒级精度
// 足够在线时长展示；可达时两处端点同源）。
var perfStartedAt = time.Now()

// buildPerformanceSnapshot 采集运行时 + 宿主机占用并叠加阈值判定。
// 采集失败（如容器内宿主机指标不可读）按零值降级，不阻塞快照。
func buildPerformanceSnapshot(svcCtx *svc.ServiceContext) PerformanceSnapshotResponse {
	snap := PerformanceSnapshotResponse{
		Settings: settings.Current().PerformanceSettings(),
	}
	// 在线时长与 /ops/system/runtime 同源（svcCtx.StartTime），缺失才落包级兜底
	startedAt := perfStartedAt
	if svcCtx != nil && !svcCtx.StartTime.IsZero() {
		startedAt = svcCtx.StartTime
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	snap.Runtime = PerformanceRuntime{
		GoMaxProcs:     runtime.GOMAXPROCS(0),
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocBytes: ms.HeapAlloc,
		HeapSysBytes:   ms.HeapSys,
		SysBytes:       ms.Sys,
		NumGC:          ms.NumGC,
		GCPauseMs:      float64(ms.PauseTotalNs) / 1e6,
		UptimeSeconds:  int64(time.Since(startedAt) / time.Second),
	}

	// CPU：0 间隔 = 与上次调用之间窗口，非阻塞
	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		snap.Host.CPUPercent = pct[0]
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		snap.Host.MemoryTotalBytes = vm.Total
		snap.Host.MemoryUsedBytes = vm.Used
		snap.Host.MemoryUsedPct = vm.UsedPercent
	}
	if du, err := disk.Usage("."); err == nil {
		snap.Host.DiskPath = du.Path
		snap.Host.DiskTotalBytes = du.Total
		snap.Host.DiskUsedBytes = du.Used
		snap.Host.DiskUsedPct = du.UsedPercent
	}

	snap.Overload = PerformanceOverload{
		CPU:    snap.Settings.MaxCpuPct > 0 && snap.Host.CPUPercent >= float64(snap.Settings.MaxCpuPct),
		Memory: snap.Settings.MaxMemoryPct > 0 && snap.Host.MemoryUsedPct >= float64(snap.Settings.MaxMemoryPct),
		Disk:   snap.Settings.MaxDiskPct > 0 && snap.Host.DiskUsedPct >= float64(snap.Settings.MaxDiskPct),
	}
	return snap
}

// perfUsername 审计写入人（同 sitesettings.currentUsername 口径：gin context
// 的 username，缺省 system）。
func perfUsername(c *gin.Context) string {
	if v, ok := c.Get("username"); ok {
		if str, ok := v.(string); ok && str != "" {
			return str
		}
	}
	return "system"
}

// jsonNumber 接受任意 JSON 数字（int/int64/float），供 PUT body 逐键解析。
type jsonNumber struct {
	raw json.RawMessage
	f   float64
}

func (n jsonNumber) Raw() json.RawMessage { return n.raw }
func (n jsonNumber) Int64() int64         { return int64(n.f) }

func (n *jsonNumber) UnmarshalJSON(data []byte) error {
	n.raw = append(n.raw[:0], data...)
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	n.f = f
	return nil
}
