package provision

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AgentRegistry 是等待 agent 自注册的抽象（server RegistryStore 适配）。
// 判定按供给标签匹配（labelValue=run id）——agent 的 AgentID 由 agent 自身
// 决定，不与 terraform instance_id 等同，故以 cloud-init 注入的专属标签关联。
type AgentRegistry interface {
	// WaitRegistered 阻塞直到带 labelKey=labelValue 的 agent 注册，或超时。
	// 返回已确认注册的 agent id；为空表示超时无注册。
	WaitRegistered(ctx context.Context, labelKey, labelValue string, timeout time.Duration) ([]string, error)
}

// ApprovalGate 是团队工单（ticket）模式的审批抽象（server approvals store 适配）。
type ApprovalGate interface {
	// CreateApproval 创建审批记录（关联 runID），返回 approval id。
	CreateApproval(ctx context.Context, req *Request, runID string) (string, error)
	// Approved 返回审批是否已批准（false 且 err==nil 表示仍待审/未批）。
	Approved(ctx context.Context, approvalID string) (bool, error)
}

// RunStore 是供给运行的持久化抽象（API 层用 gorm 实现）。
type RunStore interface {
	Create(ctx context.Context, r *Run) error
	Update(ctx context.Context, r *Run) error
	Get(ctx context.Context, id string) (*Run, error)
	List(ctx context.Context, limit int) ([]*Run, error)
}

// Orchestrator 执行供给闭环：validate → render → apply → waitRegister → adopt。
// Runner/Registry/Approval/Store 全部经接口注入，测试用 Fake 走同一代码路径。
type Orchestrator struct {
	Runner     Runner
	Registry   AgentRegistry
	Approval   ApprovalGate
	Store      RunStore
	WorkDir    string // 各运行的工作目录前缀
	InstallURL string // agent 安装脚本 URL（注入 cloud-init）

	RegisterTimeout time.Duration // 默认 5m
	DefaultDiskGB   int           // 模板默认盘大小（0 用模板 default）

	// applyMu 串行化 apply 执行段（SubmitAsync/ResumeAsync 后台 goroutine 与
	// 测试同步路径共用）：terraform 本身重量级，且 pending→applying 状态闸
	// 在并发 Resume 下须互斥才成立。单实例内保证；多 server 实例各自跑供给
	// 引用同一工作目录会互相干扰——供给引擎仅应在单一 server 实例启用
	// （见 docs/review 已知边界）。
	applyMu sync.Mutex
}

// asyncApplyBudget 是异步 apply 全闭环的时间上限：terraform apply 自身无
// 超时，后台 goroutine 若不封顶可能永久悬挂泄漏。覆盖 apply + 等待注册
// 的合理窗口（注册超时默认 5m）；超过即 cancel，run 落 failed。
const asyncApplyBudget = 30 * time.Minute

var (
	// ErrRunNotPending 续行时运行已非 pending（状态闸拒绝，防双跑）。
	ErrRunNotPending = errors.New("provision: run not pending")
	// ErrRunNotApproved 审批未通过（ticket 续行前置检查拒绝）。
	ErrRunNotApproved = errors.New("provision: run not yet approved")
)

const (
	defaultRegisterTimeout = 5 * time.Minute
	defaultDiskGB          = 40
)

// NewRunID 生成运行 id（12 字节 hex 前缀）。
func NewRunID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "prov-" + hex.EncodeToString(b)
}

// Validate 校验请求；返回规范化后的请求。
func Validate(req *Request) (*Request, error) {
	if req == nil {
		return nil, fmt.Errorf("provision: nil request")
	}
	if req.Provider == "" {
		return nil, fmt.Errorf("provision: provider required")
	}
	valid := false
	for _, p := range AllProviders() {
		if p == req.Provider {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("provision: unsupported provider %q", req.Provider)
	}
	if req.Region == "" {
		return nil, fmt.Errorf("provision: region required")
	}
	if req.Count < 1 {
		return nil, fmt.Errorf("provision: count must be >= 1")
	}
	if req.Spec.InstanceType == "" {
		return nil, fmt.Errorf("provision: spec.instanceType required")
	}
	if req.GameID == "" {
		return nil, fmt.Errorf("provision: gameId required (纳管作用域)")
	}
	if req.Env == "" {
		return nil, fmt.Errorf("provision: env required (纳管作用域)")
	}
	if req.Policy == "" {
		req.Policy = PolicyDirect
	}
	if req.Policy != PolicyDirect && req.Policy != PolicyTicket {
		return nil, fmt.Errorf("provision: unsupported policy %q", req.Policy)
	}
	if req.Spec.DiskSizeGB <= 0 {
		req.Spec.DiskSizeGB = defaultDiskGB
	}
	return req, nil
}

// Submit 按策略入口：direct 立即执行 Create；ticket 先建审批记录（pending），
// 批准后经 Resume 执行。
func (o *Orchestrator) Submit(ctx context.Context, req *Request) (*Run, error) {
	req, err := Validate(req)
	if err != nil {
		return nil, err
	}
	if req.Policy == PolicyTicket {
		run := &Run{
			ID:        NewRunID(),
			Request:   *req,
			State:     StatePending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := o.Store.Create(ctx, run); err != nil {
			return nil, fmt.Errorf("provision: persist pending run: %w", err)
		}
		approvalID, err := o.Approval.CreateApproval(ctx, req, run.ID)
		if err != nil {
			return nil, fmt.Errorf("provision: create approval: %w", err)
		}
		run.ApprovalID = approvalID
		run.UpdatedAt = time.Now()
		if err := o.Store.Update(ctx, run); err != nil {
			return nil, fmt.Errorf("provision: persist approval link: %w", err)
		}
		return run, nil
	}
	return o.Create(ctx, req)
}

// Create 执行完整扩容：render → apply → waitRegister → adopt。阻塞直到纳管或失败。
func (o *Orchestrator) Create(ctx context.Context, req *Request) (*Run, error) {
	req, err := Validate(req)
	if err != nil {
		return nil, err
	}
	run := &Run{
		ID:        NewRunID(),
		Request:   *req,
		State:     StateApplying,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := o.Store.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("provision: persist run: %w", err)
	}
	if err := o.apply(ctx, run); err != nil {
		run.State = StateFailed
		run.Error = err.Error()
		run.UpdatedAt = time.Now()
		_ = o.Store.Update(ctx, run)
		return run, err
	}
	return run, nil
}

// apply 是 Create 的核心执行段（含状态推进）。
func (o *Orchestrator) apply(ctx context.Context, run *Run) error {
	workDir, err := o.writeWorkDir(run)
	if err != nil {
		return err
	}
	if err := o.Runner.Init(ctx, workDir); err != nil {
		return fmt.Errorf("provision init: %w", err)
	}
	cloudInit := renderCloudInit(CloudInitOptions{
		HubAddr:         run.Request.HubAddr,
		BootstrapToken:  run.Request.BootstrapToken,
		AgentInstallURL: o.InstallURL,
		RunID:           run.ID,
		Extra:           run.Request.CloudInitExtra,
	})
	vars := map[string]string{
		"region":        run.Request.Region,
		"instance_type": run.Request.Spec.InstanceType,
		"user_data":     cloudInit,
		"count":         fmt.Sprintf("%d", run.Request.Count),
		"name_prefix":   "croupier-" + run.Request.GameID + "-" + run.Request.Env,
		"disk_size":     fmt.Sprintf("%d", run.Request.Spec.DiskSizeGB),
	}
	if run.Request.Spec.Image != "" {
		vars["image"] = run.Request.Spec.Image
	}
	outputs, err := o.Runner.Apply(ctx, workDir, vars)
	if err != nil {
		return fmt.Errorf("provision apply: %w", err)
	}
	run.Outputs = outputs
	run.State = StateWaitingRegister
	run.UpdatedAt = time.Now()
	if err := o.Store.Update(ctx, run); err != nil {
		return fmt.Errorf("provision persist after apply: %w", err)
	}

	timeout := o.RegisterTimeout
	if timeout <= 0 {
		timeout = defaultRegisterTimeout
	}
	reg, err := o.Registry.WaitRegistered(ctx, ProvisionLabelKey, run.ID, timeout)
	if err != nil {
		run.State = StateFailed
		run.Error = fmt.Sprintf("wait register: %v", err)
		run.UpdatedAt = time.Now()
		_ = o.Store.Update(ctx, run)
		return err
	}
	if len(reg) == 0 {
		// 空注册 = 超时无 agent 拨入：纳管前提（注册成功）不成立，判失败而非空纳管。
		err := fmt.Errorf("wait register: no agent registered within %s", timeout)
		run.State = StateFailed
		run.Error = err.Error()
		run.UpdatedAt = time.Now()
		_ = o.Store.Update(ctx, run)
		return err
	}
	run.AgentIDs = reg
	run.State = StateAdopted
	run.UpdatedAt = time.Now()
	if err := o.Store.Update(ctx, run); err != nil {
		return fmt.Errorf("provision persist after adopt: %w", err)
	}
	return nil
}

// writeWorkDir 渲染 main.tf 到工作目录。
func (o *Orchestrator) writeWorkDir(run *Run) (string, error) {
	mainTF, err := renderTemplate(run.Request.Provider, run.Request)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(o.WorkDir, run.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("provision: mkdir %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(mainTF), 0o600); err != nil {
		return "", fmt.Errorf("provision: write main.tf: %w", err)
	}
	return dir, nil
}

// Resume 在 ticket 审批通过后执行 Create（复用已持久化 Run 的请求）。
// 同步阻塞直到 apply/等注册完成（测试与批处理路径）；HTTP 入口用
// ResumeAsync（不阻塞请求）。
func (o *Orchestrator) Resume(ctx context.Context, runID string) (*Run, error) {
	run, err := o.beginResume(ctx, runID)
	if err != nil {
		return nil, err
	}
	o.applyMu.Lock()
	defer o.applyMu.Unlock()
	if err := o.apply(ctx, run); err != nil {
		return run, err
	}
	return run, nil
}

// SubmitAsync 是 HTTP 入口的策略提交（不阻塞请求）：ticket 走 Submit（建
// 审批记录即返回 pending，本身不阻塞）；direct 持久化 applying 后在后台
// goroutine 走 apply 全闭环。apply + 等注册可持续数分钟，不能挂在 HTTP
// 请求上下文上——请求一结束 ctx 就取消，terraform 会被中途杀掉。
func (o *Orchestrator) SubmitAsync(ctx context.Context, req *Request) (*Run, error) {
	req, err := Validate(req)
	if err != nil {
		return nil, err
	}
	if req.Policy == PolicyTicket {
		return o.Submit(ctx, req)
	}
	run := &Run{
		ID:        NewRunID(),
		Request:   *req,
		State:     StateApplying,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := o.Store.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("provision: persist run: %w", err)
	}
	submitted := *run // 拷贝先于 spawn：goroutine 建立 happens-before 其后改写 run 指针本体
	o.spawnApply(run)
	return &submitted, nil
}

// ResumeAsync 是 HTTP 入口的审批续行（不阻塞请求）：同步做 pending+批准
// 双检查并把状态推进 applying（并发 Resume 的状态闸——推进先于后台执行，
// 双续行只有第一个能通过 pending 检查），随后后台执行 apply 全闭环。
func (o *Orchestrator) ResumeAsync(ctx context.Context, runID string) (*Run, error) {
	run, err := o.beginResume(ctx, runID)
	if err != nil {
		return nil, err
	}
	resumed := *run // 拷贝先于 spawn：goroutine 建立 happens-before 其后改写 run 指针本体
	o.spawnApply(run)
	return &resumed, nil
}

// beginResume 是 Resume/ResumeAsync 共用的同步前置：pending 检查 → 批准
// 检查 → 推进 applying 并持久化。状态推进先行保证并发续行互斥（第二
// 个进来读到的已非 pending）。
func (o *Orchestrator) beginResume(ctx context.Context, runID string) (*Run, error) {
	run, err := o.Store.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.State != StatePending {
		return nil, fmt.Errorf("%w: run %s (state=%s)", ErrRunNotPending, runID, run.State)
	}
	ok, err := o.Approval.Approved(ctx, run.ApprovalID)
	if err != nil {
		return nil, fmt.Errorf("provision: check approval: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("%w: run %s", ErrRunNotApproved, runID)
	}
	run.State = StateApplying
	run.UpdatedAt = time.Now()
	if err := o.Store.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("provision: persist applying: %w", err)
	}
	return run, nil
}

// spawnApply 在后台 goroutine 执行 apply 全闭环。用独立 context（async
// ApplyBudget 封顶防 terraform 悬挂泄漏 goroutine），不继承 HTTP 请求 ctx；
// 失败已由 apply 落库 failed，此处仅日志（后台无返回通道）。
func (o *Orchestrator) spawnApply(run *Run) {
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), asyncApplyBudget)
		defer cancel()
		o.applyMu.Lock()
		defer o.applyMu.Unlock()
		if err := o.apply(bg, run); err != nil {
			slog.Default().Error("provision: async apply failed", "run", run.ID, "error", err)
		}
	}()
}

// Destroy 回收资源（OPS-002 排水后调用；本方法只做 terraform destroy 原语）。
func (o *Orchestrator) Destroy(ctx context.Context, runID string) error {
	run, err := o.Store.Get(ctx, runID)
	if err != nil {
		return err
	}
	dir := filepath.Join(o.WorkDir, run.ID)
	if _, err := os.Stat(filepath.Join(dir, "main.tf")); err != nil {
		return fmt.Errorf("provision: workdir for %s not found: %w", runID, err)
	}
	if err := o.Runner.Destroy(ctx, dir); err != nil {
		return fmt.Errorf("provision destroy: %w", err)
	}
	run.State = StateDestroyed
	run.UpdatedAt = time.Now()
	return o.Store.Update(ctx, run)
}
