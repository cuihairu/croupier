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
	// CreateDestroyApproval 创建销毁申请单的审批记录（关联 runID），返回 approval id。
	CreateDestroyApproval(ctx context.Context, run *Run) (string, error)
	// Approved 返回审批是否已批准（false 且 err==nil 表示仍待审/未批）。
	Approved(ctx context.Context, approvalID string) (bool, error)
}

// RunStore 是供给运行的持久化抽象（API 层用 gorm 实现）。
type RunStore interface {
	Create(ctx context.Context, r *Run) error
	Update(ctx context.Context, r *Run) error
	Get(ctx context.Context, id string) (*Run, error)
	List(ctx context.Context, limit int) ([]*Run, error)
	// CompareAndSwapState 仅当当前 state == from 时原子更新为 to（含整行回读）。
	// 返回 (成功, 当前行)；ok=false 表示并发已把 state 改走（状态闸）。
	CompareAndSwapState(ctx context.Context, id, from, to string) (bool, *Run, error)
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
	// InjectResumeParams 在审批续行（beginResume）时把 server 侧敏感/拓扑
	// 参数（HubAddr/BootstrapToken）注入已持久化的 Request。这些字段不入库
	// （json:"-"），ticket 模式等待审批期间 server 配置可轮换 token；续行
	// 时必须重新注入，否则渲染出空 hubAddr/token（#657）。nil 表示不注入。
	InjectResumeParams func(*Request) error

	RegisterTimeout time.Duration // 默认 5m
	DefaultDiskGB   int           // 模板默认盘大小（0 用模板 default）

	// applyMu 串行化 apply 执行段（SubmitAsync/ResumeAsync 后台 goroutine 与
	// 测试同步路径共用）：terraform 本身重量级，且 pending→applying 状态闸
	// 在并发 Resume 下须互斥才成立。单实例内保证；多 server 实例各自跑供给
	// 引用同一工作目录会互相干扰——供给引擎仅应在单一 server 实例启用
	// （见 docs/review 已知边界）。
	applyMu sync.Mutex

	// destroyMu 串行化销毁申请单（RequestDestroy）的 Get→检查→Create 审批→
	// Update 读改写（#661 同类 TOCTOU 防重）：并发重复销毁申请只有第一个能
	// 通过检查并创建审批，其余读到已含 DestroyApprovalID/非 adopted 被拒，
	// 不再产生孤儿销毁审批记录。销毁申请仅走单一 server 实例（同 applyMu
	// 边界）；本锁与 applyMu 独立，避免与在途 apply 互斥放大。
	destroyMu sync.Mutex
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
	// ErrRunNotDestroyPending 销毁续行时运行已非 destroy_pending（状态闸拒绝）。
	ErrRunNotDestroyPending = errors.New("provision: run not destroy pending")
	// ErrRunNotDestroyable 直接销毁时运行已非 adopted（非纳管/已销毁/在途）。
	ErrRunNotDestroyable = errors.New("provision: run not destroyable")
)

const (
	defaultRegisterTimeout = 5 * time.Minute
	defaultDiskGB          = 40
	// maxCountPerRequest 单次供给请求实例数上限：防止一次性创建海量云实例
	// 造成失控成本/资源（#663）。批量扩容应由多条请求分次提交。
	maxCountPerRequest = 100
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
	if req.Count > maxCountPerRequest {
		return nil, fmt.Errorf("provision: count must be <= %d", maxCountPerRequest)
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
	if req.CloudInitExtra != "" {
		if err := validateCloudInitExtra(req.CloudInitExtra); err != nil {
			return nil, err
		}
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
		return run, err
	}
	return run, nil
}

// apply 是 Create 的核心执行段（含状态推进）。
func (o *Orchestrator) apply(ctx context.Context, run *Run) error {
	fail := func(err error) error {
		run.State = StateFailed
		run.Error = err.Error()
		run.UpdatedAt = time.Now()
		if updateErr := o.Store.Update(ctx, run); updateErr != nil {
			return fmt.Errorf("provision: persist failed state: %w (apply error: %v)", updateErr, err)
		}
		return err
	}
	workDir, err := o.writeWorkDir(run)
	if err != nil {
		return fail(err)
	}
	if err := o.Runner.Init(ctx, workDir); err != nil {
		return fail(fmt.Errorf("provision init: %w", err))
	}
	cloudInit, err := renderCloudInit(CloudInitOptions{
		HubAddr:         run.Request.HubAddr,
		BootstrapToken:  run.Request.BootstrapToken,
		AgentInstallURL: o.InstallURL,
		RunID:           run.ID,
		Extra:           run.Request.CloudInitExtra,
	})
	if err != nil {
		return fail(fmt.Errorf("provision render cloud-init: %w", err))
	}
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
		return fail(fmt.Errorf("provision apply: %w", err))
	}
	run.Outputs = outputs
	run.State = StateWaitingRegister
	run.UpdatedAt = time.Now()
	if err := o.Store.Update(ctx, run); err != nil {
		return fail(fmt.Errorf("provision persist after apply: %w", err))
	}

	timeout := o.RegisterTimeout
	if timeout <= 0 {
		timeout = defaultRegisterTimeout
	}
	reg, err := o.Registry.WaitRegistered(ctx, ProvisionLabelKey, run.ID, timeout)
	if err != nil {
		return fail(fmt.Errorf("wait register: %w", err))
	}
	if len(reg) == 0 {
		// 空注册 = 超时无 agent 拨入：纳管前提（注册成功）不成立，判失败而非空纳管。
		err := fmt.Errorf("wait register: no agent registered within %s", timeout)
		return fail(err)
	}
	run.AgentIDs = reg
	run.State = StateAdopted
	run.UpdatedAt = time.Now()
	if err := o.Store.Update(ctx, run); err != nil {
		return fail(fmt.Errorf("provision persist after adopt: %w", err))
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
	if o.InjectResumeParams != nil {
		if err := o.InjectResumeParams(&run.Request); err != nil {
			return nil, fmt.Errorf("provision: inject resume params: %w", err)
		}
	}
	ok, err := o.Approval.Approved(ctx, run.ApprovalID)
	if err != nil {
		return nil, fmt.Errorf("provision: check approval: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("%w: run %s", ErrRunNotApproved, runID)
	}
	// 原子 pending→applying CAS：并发 ResumeAsync 只有第一个通过，第二个
	// 读到已非 pending（#661，此前为 Get→检查→Update 非原子读改写）。
	ok, current, err := o.Store.CompareAndSwapState(ctx, runID, string(StatePending), string(StateApplying))
	if err != nil {
		return nil, fmt.Errorf("provision: cas applying: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("%w: run %s (state=%s)", ErrRunNotPending, runID, current.State)
	}
	// CAS 回读的 current 不含注入的敏感/拓扑参数（不入库），须把已校验的
	// HubAddr/BootstrapToken 覆盖到 current，供 apply 渲染（#657）。
	current.Request.HubAddr = run.Request.HubAddr
	current.Request.BootstrapToken = run.Request.BootstrapToken
	return current, nil
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
	if run.State != StateAdopted && run.State != StateDestroyFailed {
		return fmt.Errorf("%w: run %s (state=%s)", ErrRunNotDestroyable, runID, run.State)
	}
	from := string(run.State)
	ok, current, err := o.Store.CompareAndSwapState(ctx, runID, from, string(StateDestroying))
	if err != nil {
		return fmt.Errorf("provision: cas destroying: %w", err)
	}
	if !ok {
		return fmt.Errorf("%w: run %s (state=%s)", ErrRunNotDestroyable, runID, current.State)
	}
	o.applyMu.Lock()
	defer o.applyMu.Unlock()
	return o.runDestroy(ctx, current)
}

// RequestDestroy 是销毁申请单入口（workflow 模式）：run 须为 adopted，
// 创建审批记录并推进 destroy_pending（销毁须经批准后执行）。
func (o *Orchestrator) RequestDestroy(ctx context.Context, runID string) (*Run, error) {
	o.destroyMu.Lock()
	defer o.destroyMu.Unlock()

	run, err := o.Store.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.State != StateAdopted {
		return nil, fmt.Errorf("%w: run %s (state=%s)", ErrRunNotDestroyPending, runID, run.State)
	}
	if run.DestroyApprovalID != "" {
		return nil, fmt.Errorf("provision: destroy approval already requested for %s", runID)
	}
	approvalID, err := o.Approval.CreateDestroyApproval(ctx, run)
	if err != nil {
		return nil, fmt.Errorf("provision: create destroy approval: %w", err)
	}
	run.DestroyApprovalID = approvalID
	run.State = StateDestroyPending
	run.UpdatedAt = time.Now()
	if err := o.Store.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("provision: persist destroy approval: %w", err)
	}
	return run, nil
}

// DestroyResume 在销毁审批通过后执行 terraform destroy 原语。同步阻塞直到
// 销毁完成（与 Destroy 同路径，HTTP 入口不阻塞时经后台 goroutine）。
func (o *Orchestrator) DestroyResume(ctx context.Context, runID string) (*Run, error) {
	run, err := o.Store.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.State != StateDestroyPending && run.State != StateDestroyFailed {
		return nil, fmt.Errorf("%w: run %s (state=%s)", ErrRunNotDestroyPending, runID, run.State)
	}
	if run.DestroyApprovalID == "" {
		return nil, fmt.Errorf("provision: destroy approval not requested for %s", runID)
	}
	ok, err := o.Approval.Approved(ctx, run.DestroyApprovalID)
	if err != nil {
		return nil, fmt.Errorf("provision: check destroy approval: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("%w: run %s", ErrRunNotApproved, runID)
	}
	// 原子 destroy_pending→destroying CAS + applyMu：并发双销毁只有一个通过，
	// 且与在途 apply 互斥（同一 workdir terraform state 不竞态）。
	ok, current, err := o.Store.CompareAndSwapState(ctx, runID, string(run.State), string(StateDestroying))
	if err != nil {
		return nil, fmt.Errorf("provision: cas destroying: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("%w: run %s (state=%s)", ErrRunNotDestroyPending, runID, current.State)
	}
	o.applyMu.Lock()
	defer o.applyMu.Unlock()
	if err := o.runDestroy(ctx, current); err != nil {
		return current, err
	}
	return current, nil
}

// persistCtx 派生独立于请求 ctx 的落库上下文（WithoutCancel + 超时）。客户端
// 断开（HTTP ctx 已取消）后 destroy_failed/destroyed 仍必须可靠落库，否则
// run 永久卡 destroying；复用已取消的 ctx 会让真实 DB 随 ctx 中止 UPDATE。
func persistCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
}

// runDestroy 执行 terraform destroy 原语并落 destroyed（共用段）。
func (o *Orchestrator) runDestroy(ctx context.Context, run *Run) error {
	// 已由 CAS/校验把 state 推进 destroying（或 adopted→destroying），
	// 此处只做 I/O + 落 destroyed。
	dir := filepath.Join(o.WorkDir, run.ID)
	if _, err := os.Stat(filepath.Join(dir, "main.tf")); err != nil {
		return o.failDestroy(ctx, run, fmt.Errorf("provision: workdir for %s not found: %w", run.ID, err))
	}
	if err := o.Runner.Destroy(ctx, dir); err != nil {
		return o.failDestroy(ctx, run, fmt.Errorf("provision destroy: %w", err))
	}
	run.State = StateDestroyed
	run.Error = ""
	run.UpdatedAt = time.Now()
	pctx, cancel := persistCtx(ctx)
	defer cancel()
	return o.Store.Update(pctx, run)
}

// failDestroy 把销毁失败持久化为 destroy_failed（而非卡在 destroying），
// 使 Destroy（需 adopted/destroy_failed）与 DestroyResume（需
// destroy_pending/destroy_failed）都能重试。落库失败也上抛（调用方可见）。
func (o *Orchestrator) failDestroy(ctx context.Context, run *Run, err error) error {
	run.State = StateDestroyFailed
	run.Error = err.Error()
	run.UpdatedAt = time.Now()
	pctx, cancel := persistCtx(ctx)
	defer cancel()
	if updateErr := o.Store.Update(pctx, run); updateErr != nil {
		return fmt.Errorf("provision: persist destroy failed state: %w (destroy error: %v)", updateErr, err)
	}
	return err
}
