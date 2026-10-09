package healthprobe

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// HeartbeatFile Heartbeat 语义的打点文件：mtime 即最近活体时刻（supervisor
// 的 last_heartbeat_unix 事件上下文采样同源）。core 系 agent 周期 Beat()，
// 监管方/面板经 LastBeat/Age 判断活性。
type HeartbeatFile struct {
	path string
}

// NewHeartbeatFile 构造打点文件句柄（不立即落盘；首次 Beat 时创建）。
func NewHeartbeatFile(path string) *HeartbeatFile { return &HeartbeatFile{path: path} }

// Path 返回打点文件路径。
func (h *HeartbeatFile) Path() string { return h.path }

// Beat 打点：写当前 unix 毫秒（mtime 随之刷新）。
func (h *HeartbeatFile) Beat() error {
	if dir := filepath.Dir(h.path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create heartbeat dir: %w", err)
		}
	}
	if err := os.WriteFile(h.path, []byte(strconv.FormatInt(time.Now().UnixMilli(), 10)), 0o644); err != nil {
		return fmt.Errorf("write heartbeat: %w", err)
	}
	return nil
}

// LastBeat 返回最近打点时刻（文件 mtime；文件不存在返回 false）。
func (h *HeartbeatFile) LastBeat() (time.Time, bool) {
	info, err := os.Stat(h.path)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

// Age 返回距最近打点的时长（文件不存在返回 false）。
func (h *HeartbeatFile) Age() (time.Duration, bool) {
	last, ok := h.LastBeat()
	if !ok {
		return 0, false
	}
	return time.Since(last), true
}
