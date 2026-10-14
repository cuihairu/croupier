package agent

// 受管程序二进制更新（agent-supervisor-design §10.4 更新状态+指纹）。
//
// 五要求的 v1 落地面与诚实边界：
//   - 内容哈希校验：已实现（sha256 fail-closed，expectedSha256 必填）。
//     签名校验未实现（签名随密钥轮换会变，哈希才是稳定可查锚点）。
//   - 运行中文件占用：先停后换（coreStop → 换文件 → coreStart）；旧二进制
//     先改名备份供回滚。
//   - 新版本启动失败回滚：稳定性窗口内退出（waitDone 关闭）即回滚旧版并留
//     update_rolled_back 事件；窗口后退出属运行态，走既有监管/恢复环，不算
//     更新失败。窗口缺省 5s（updateStabilityWindow 可测注入）。
//   - 兼容性检查：无通用实现（DB/配置兼容是游戏侧知识；配置版本由调用方关联）。
//   - 游戏进程维护窗口 / 在线人数阈值 / 流量导入闸：未核实，随首个真实游戏
//     接入定（设计文档 §10.4 已记）。
//
// 指纹记录（硬要求）：更新结果落 supervisor 事件（update_applied /
// update_rolled_back，message 带旧→新指纹）；持久查询主入口=server 侧 execlog
// 审计（source=process_update，入参/结果全文含指纹）；快照携带运行实例指纹。
// 期望状态语义：更新动作自己发起的启动即视为显式运行意图（desired=true），
// 与手动 restart 一致。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// updateStabilityWindow 是「新版本启动失败回滚」的判定窗口：窗口内进程退出
// 即判定启动失败并回滚；窗口后退出属运行态，走既有监管/恢复环。
const updateStabilityWindow = 5 * time.Second

// updateBackupSuffix 是换包前旧二进制备份的后缀（同目录，rename 原子）。
// 更新成功后保留为 last-known-good（滚动覆盖），供人工回滚。
const updateBackupSuffix = ".croupier-bak"

// sha256HexPattern 约束期望指纹为 64 位小写 hex（调用面 fail-closed）。
var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// sha256HexFile 计算文件内容 sha256（hex 小写）；读不到返回空串——指纹缺失
// 本身是可见事实，不伪造值。
func sha256HexFile(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// UpdateProcess swaps a managed process binary (§10.4). Flow: hash-pin the
// staged file → stop → backup + swap → start → stability window → rollback on
// early exit. Events: update_applied / update_rolled_back (fingerprints in the
// message); the persistent query entry is the server-side execlog audit.
func (s *OpsServer) UpdateProcess(ctx context.Context, req *opsv1.UpdateProcessRequest) (*opsv1.UpdateProcessResponse, error) {
	// 独立授权闸（§10.7 程序操作=明确受授权动作）：update 与 restart/exec
	// 同属双闸家族，独立键控。
	if !s.config.Enabled || !s.config.AllowUpdate {
		return nil, fmt.Errorf("ops update is not enabled")
	}

	name := strings.TrimSpace(req.GetProcessName())
	staged := strings.TrimSpace(req.GetStagedPath())
	expected := strings.ToLower(strings.TrimSpace(req.GetExpectedSha256()))
	if name == "" || staged == "" {
		return nil, fmt.Errorf("processName and stagedPath are required")
	}
	if !sha256HexPattern.MatchString(expected) {
		return nil, fmt.Errorf("expectedSha256 must be 64-char lowercase hex (content pin is a hard prerequisite)")
	}

	s.mu.RLock()
	p, ok := s.processes[name]
	s.mu.RUnlock()

	if !ok {
		// 更新目标必须是已声明受管进程（未物化时同 StartProcess 路径物化为停止态）。
		s.mu.RLock()
		cfg, hasCfg := s.config.ManagedProcesses[name]
		s.mu.RUnlock()
		if !hasCfg {
			return nil, fmt.Errorf("process '%s' not configured", name)
		}
		p = &managedProcess{
			name:   name,
			config: cfg,
			state:  opsv1.ProcessState_PROCESS_STATE_STOPPED,
			stopCh: make(chan struct{}),
		}
		s.mu.Lock()
		s.processes[name] = p
		s.mu.Unlock()
	}

	p.mu.RLock()
	target := p.config.Command
	p.mu.RUnlock()
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("process '%s' has no command to update", name)
	}

	// 内容哈希校验（fail-closed）：staged 文件的 sha256 不等于期望值一律拒绝，
	// 不碰运行中的进程与目标文件。
	newFP := sha256HexFile(staged)
	if newFP == "" {
		return nil, fmt.Errorf("staged file unreadable: %s", staged)
	}
	if newFP != expected {
		return nil, fmt.Errorf("staged file sha256 mismatch: expected %s, got %s (update refused)", expected, newFP)
	}

	oldFP := sha256HexFile(target)

	// 先停后换（运行中文件占用处理）：静默核心（不写 manual_* 事件、不动
	// 期望状态——更新成败由 update_applied / update_rolled_back 单事件定论）。
	s.coreStopProcess(p)

	return s.swapAndStart(p, target, staged, oldFP, newFP), nil
}

// swapAndStart 备份旧二进制 → 换入新二进制 → 拉起 → 守稳定性窗口；窗口内退出
// 即回滚旧版。任何失败都先把目标路径恢复到旧内容再启动（不留半更新态）。
func (s *OpsServer) swapAndStart(p *managedProcess, target, staged, oldFP, newFP string) *opsv1.UpdateProcessResponse {
	backup := target + updateBackupSuffix
	hadBackup := false
	if oldFP != "" {
		if err := os.Rename(target, backup); err != nil {
			return s.restoreAndReport(p, target, backup, false, oldFP, newFP, "backup old binary failed: "+err.Error())
		}
		hadBackup = true
	}

	if err := installFile(staged, target); err != nil {
		return s.restoreAndReport(p, target, backup, hadBackup, oldFP, newFP, "install new binary failed: "+err.Error())
	}

	window := s.updateStabilityWindow
	if window <= 0 {
		window = updateStabilityWindow
	}
	if _, _, err := s.coreStartProcess(p); err != nil {
		return s.rollback(p, target, backup, hadBackup, oldFP, newFP, "start new binary failed: "+err.Error())
	}

	// 稳定性窗口：waitDone 由 monitorProcess 在进程退出后关闭——窗口内关闭
	// 即启动失败（新版本崩溃），窗口后关闭属运行态（走既有恢复环）。
	p.mu.RLock()
	waitDone := p.waitDone
	p.mu.RUnlock()
	select {
	case <-waitDone:
		return s.rollback(p, target, backup, hadBackup, oldFP, newFP, "new binary exited within stability window")
	case <-time.After(window):
	}

	s.desired.SetRunning(p.name, true)
	// 备份保留为 last-known-good（同目录 <target>.croupier-bak，滚动覆盖）：
	// 人工回滚的最后手段，不随成功删除。
	ev := newSupervisorEvent(p.name, supervisorEventUpdateApplied)
	ev.Message = fmt.Sprintf("binary updated sha256 %s -> %s", fpOrUnknown(oldFP), newFP)
	s.emitSupervisorEvent(p, ev)

	return &opsv1.UpdateProcessResponse{
		Success:        true,
		Message:        fmt.Sprintf("Process '%s' binary updated", p.name),
		OldFingerprint: oldFP,
		NewFingerprint: newFP,
	}
}

// restoreAndReport 处理换包前段失败（备份/安装失败）：目标路径恢复为旧内容后
// 重新拉起旧版，进程状态回到更新前；结果如实上报（无事件——二进制未变）。
func (s *OpsServer) restoreAndReport(p *managedProcess, target, backup string, hadBackup bool, oldFP, newFP, reason string) *opsv1.UpdateProcessResponse {
	if hadBackup {
		_ = os.Remove(target)
		_ = os.Rename(backup, target)
	}
	s.restartRestoredBinary(p, target)
	return &opsv1.UpdateProcessResponse{
		Success:        false,
		Message:        "update failed: " + reason,
		OldFingerprint: oldFP,
		NewFingerprint: newFP,
	}
}

// rollback 处理换包后段失败（新版本启动失败/窗口内退出）：恢复旧内容并重新
// 拉起，留 update_rolled_back 事件（rolledBackTo=旧指纹，回滚路径同样留痕）。
func (s *OpsServer) rollback(p *managedProcess, target, backup string, hadBackup bool, oldFP, newFP, reason string) *opsv1.UpdateProcessResponse {
	s.coreStopProcess(p)

	if !hadBackup {
		// 无旧版可回滚（首次安装即失败）：目标路径已是新内容，不再触碰。
		// 事件闭集仅 update_applied / update_rolled_back——此路径无事件，
		// 留痕在 execlog 审计（持久查询主入口）。
		return &opsv1.UpdateProcessResponse{
			Success:        false,
			Message:        "update failed and no rollback target (no prior binary): " + reason,
			OldFingerprint: oldFP,
			NewFingerprint: newFP,
			RolledBack:     false,
		}
	}

	if err := os.Rename(backup, target); err != nil {
		ev := newSupervisorEvent(p.name, supervisorEventUpdateRolledBack)
		ev.Message = fmt.Sprintf("binary update rollback failed to restore %s: %s", backup, err.Error())
		s.emitSupervisorEvent(p, ev)
		return &opsv1.UpdateProcessResponse{
			Success:        false,
			Message:        "update failed and rollback failed: " + err.Error(),
			OldFingerprint: oldFP,
			NewFingerprint: newFP,
			RolledBack:     false,
		}
	}

	s.restartRestoredBinary(p, target)

	ev := newSupervisorEvent(p.name, supervisorEventUpdateRolledBack)
	ev.Message = fmt.Sprintf("binary update rolled back to sha256 %s (update reason: %s)", fpOrUnknown(oldFP), reason)
	s.emitSupervisorEvent(p, ev)

	return &opsv1.UpdateProcessResponse{
		Success:        false,
		Message:        "update rolled back: " + reason,
		OldFingerprint: oldFP,
		NewFingerprint: newFP,
		RolledBack:     true,
	}
}

// restartRestoredBinary 把恢复后的旧二进制重新拉起（静默核心+显式运行意图）。
// coreStopProcess 已先 close 旧 stopCh，恢复环据此判定主动停止不再并发拉起。
func (s *OpsServer) restartRestoredBinary(p *managedProcess, target string) {
	if _, _, err := s.coreStartProcess(p); err == nil {
		s.desired.SetRunning(p.name, true)
	}
}

// installFile 把 staged 内容落到 target：优先 rename（同目录 staged 时原子）；
// rename 失败（跨设备 EXDEV 等）时写同目录临时文件再 rename，目标路径替换
// 仍保持原子，并尽量保留 staged 文件的可执行位。
func installFile(staged, target string) error {
	if err := os.Rename(staged, target); err == nil {
		return nil
	}

	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".croupier-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	src, err := os.Open(staged)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	defer func() { _ = src.Close() }()
	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if st, err := os.Stat(staged); err == nil && st.Mode()&0111 != 0 {
		if err := os.Chmod(tmpName, st.Mode()&0777); err != nil {
			return err
		}
	} else if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpName, target)
}

// fpOrUnknown 指纹留痕展示：未知（首次安装）记 "-"，不留空白歧义。
func fpOrUnknown(fp string) string {
	if fp == "" {
		return "-"
	}
	return fp
}
