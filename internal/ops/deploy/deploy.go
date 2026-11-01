// Package deploy 实现 OPS-004 运维部署发版编排：包仓库登记 + 发版单分批 +
// 健康门禁自动暂停 + 回滚。投递与健康检查经 Agent 接口抽象（生产适配复用
// 既有 ops AgentService，测试用 Fake 验证编排语义），不引入新的 wire 消息。
package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

// Agent 是部署编排所需的 agent 能力面（投递包 + 换包 + 健康检查）。
type Agent interface {
	// StagePackage 把包内容投递到 agent 本地 staging 路径，返回该路径。
	// 生产实现复用对象存储签名 URL + agent 下载；测试用 Fake。
	StagePackage(ctx context.Context, agentID string, pkg *model.DeployPackage) (string, error)
	// ApplyUpdate 在 agent 上换入受管进程新二进制（指纹校验由 agent 侧完成）。
	ApplyUpdate(ctx context.Context, agentID, processName, stagedPath, expectedSha256 string) (fingerprint string, err error)
	// Healthy 判断目标机当前健康（探针/资源门禁）；false 时暂停批次。
	Healthy(ctx context.Context, agentID string) (bool, string)
}

// Store 是部署编排的持久化面（model 助手接口，便于测试）。
type Store interface {
	Create(ctx context.Context, d *model.Deployment) error
	Update(ctx context.Context, d *model.Deployment) error
	FindByID(ctx context.Context, id string) (*model.Deployment, error)
	List(ctx context.Context, limit int) ([]model.Deployment, error)
}

// Orchestrator 编排发版单。
type Orchestrator struct {
	store Store
	agent Agent
	// HealthRetry 是单机健康确认前的等待（默认 30s，测试注入）。
	HealthRetry time.Duration
	// HealthRetries 是每机健康确认最大次数（默认 3）。
	HealthRetries int
	// BatchWait 是批次间等待（默认 0，测试注入）。
	BatchWait time.Duration
}

// New 构造编排器。
func New(store Store, agent Agent) *Orchestrator {
	return &Orchestrator{
		store:         store,
		agent:         agent,
		HealthRetry:   30 * time.Second,
		HealthRetries: 3,
		BatchWait:     0,
	}
}

// BatchPercent 闭集：1/10/50/100。
func BatchPercent(v int) bool {
	switch v {
	case 1, 10, 50, 100:
		return true
	}
	return false
}

// NewDeployment 构造发版单（draft），并把目标机按批次切分。
func NewDeployment(packageID, name, gameID, env, role, processName string, batchPercent int, targets []string, createdBy string) (*model.Deployment, error) {
	if packageID == "" {
		return nil, errors.New("deploy: packageId required")
	}
	if processName == "" {
		return nil, errors.New("deploy: processName required")
	}
	if !BatchPercent(batchPercent) {
		return nil, fmt.Errorf("deploy: invalid batch percent %d (must be 1/10/50/100)", batchPercent)
	}
	if len(targets) == 0 {
		return nil, errors.New("deploy: at least one target required")
	}

	// 按百分比切批：每批大小 = max(1, ceil(n*pct/100))。
	n := len(targets)
	perBatch := n * batchPercent / 100
	if perBatch < 1 {
		perBatch = 1
	}
	targetsJSON := make([]model.DeploymentTarget, 0, n)
	batch := 1
	for i, id := range targets {
		if i > 0 && i%perBatch == 0 {
			batch++
		}
		targetsJSON = append(targetsJSON, model.DeploymentTarget{
			AgentID: id,
			Batch:   batch,
			State:   model.DeploymentTargetPending,
		})
	}
	targetIDs := make([]string, 0, n)
	for _, t := range targetsJSON {
		targetIDs = append(targetIDs, t.AgentID)
	}
	idsJSON, _ := json.Marshal(targetIDs)
	tsJSON, _ := json.Marshal(targetsJSON)

	return &model.Deployment{
		DeploymentID:  NewDeploymentID(),
		PackageID:     packageID,
		Name:          name,
		GameID:        gameID,
		Env:           env,
		Role:          role,
		ProcessName:   processName,
		BatchPercent:  batchPercent,
		BatchCount:    batch,
		State:         model.DeploymentStateDraft,
		TargetIDsJSON: model.JSON(idsJSON),
		TargetsJSON:   model.JSON(tsJSON),
		CreatedBy:     createdBy,
	}, nil
}

// Start 从 draft 进入 deploying，并同步执行第一轮（批次推进）。
func (o *Orchestrator) Start(ctx context.Context, d *model.Deployment, pkg *model.DeployPackage) (*model.Deployment, error) {
	if d.State != model.DeploymentStateDraft {
		return d, fmt.Errorf("deploy: cannot start from state %s", d.State)
	}
	if pkg == nil || pkg.ObjectKey == "" {
		return d, errors.New("deploy: package must have objectKey")
	}
	d.State = model.DeploymentStateDeploying
	d.UpdatedAt = time.Now()
	if err := o.store.Update(ctx, d); err != nil {
		return d, err
	}
	return o.Advance(ctx, d, pkg)
}

// Advance 执行当前批次（分批推进 + 健康门禁 + 自动暂停）。
func (o *Orchestrator) Advance(ctx context.Context, d *model.Deployment, pkg *model.DeployPackage) (*model.Deployment, error) {
	targets := decodeTargets(d.TargetsJSON)
	currentBatch := nextBatch(targets)
	if currentBatch == 0 {
		d.State = model.DeploymentStateComplete
		d.UpdatedAt = time.Now()
		if err := o.store.Update(ctx, d); err != nil {
			return d, err
		}
		return d, nil
	}

	// 当前批的目标：逐个部署。
	for i := range targets {
		t := &targets[i]
		if t.Batch != currentBatch || t.State != model.DeploymentTargetPending {
			continue
		}
		t.State = model.DeploymentTargetDeploying
		t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		staged, err := o.agent.StagePackage(ctx, t.AgentID, pkg)
		if err != nil {
			t.State = model.DeploymentTargetFailed
			t.Message = "stage: " + err.Error()
			t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			d.State = model.DeploymentStateFailed
			d.PausedReason = "target " + t.AgentID + " stage failed"
			d.UpdatedAt = time.Now()
			_ = saveTargets(d, targets)
			_ = o.store.Update(ctx, d)
			return d, fmt.Errorf("deploy: stage %s: %w", t.AgentID, err)
		}
		fp, err := o.agent.ApplyUpdate(ctx, t.AgentID, d.ProcessName, staged, pkg.Checksum)
		if err != nil {
			t.State = model.DeploymentTargetFailed
			t.Message = "apply: " + err.Error()
			t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			d.State = model.DeploymentStateFailed
			d.PausedReason = "target " + t.AgentID + " apply failed"
			d.UpdatedAt = time.Now()
			_ = saveTargets(d, targets)
			_ = o.store.Update(ctx, d)
			return d, fmt.Errorf("deploy: apply %s: %w", t.AgentID, err)
		}
		t.NewFingerprint = fp

		// 健康门禁：确认成功才标 healthy。
		if !o.healthGate(ctx, t.AgentID) {
			t.State = model.DeploymentTargetFailed
			t.Message = "health gate failed"
			t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			d.State = model.DeploymentStatePaused
			d.PausedReason = "target " + t.AgentID + " health gate failed; batch paused"
			d.UpdatedAt = time.Now()
			_ = saveTargets(d, targets)
			_ = o.store.Update(ctx, d)
			return d, errors.New("deploy: health gate failed; batch paused")
		}
		t.State = model.DeploymentTargetHealthy
		t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	// 当前批全部健康：标记批完成，等待后进入下一批。
	if o.BatchWait > 0 {
		select {
		case <-ctx.Done():
			return d, ctx.Err()
		case <-time.After(o.BatchWait):
		}
	}
	_ = saveTargets(d, targets)
	d.UpdatedAt = time.Now()
	if err := o.store.Update(ctx, d); err != nil {
		return d, err
	}
	return o.Advance(ctx, d, pkg)
}

// Rollback 指回上一版本重放：把目标机恢复为上一指纹（由调用方传入 oldFingerprint
// 包），本编排只标记回滚状态并逐机调用 ApplyUpdate。
func (o *Orchestrator) Rollback(ctx context.Context, d *model.Deployment, pkg *model.DeployPackage) (*model.Deployment, error) {
	targets := decodeTargets(d.TargetsJSON)
	for i := range targets {
		t := &targets[i]
		if t.State != model.DeploymentTargetHealthy && t.State != model.DeploymentTargetFailed {
			continue
		}
		staged, err := o.agent.StagePackage(ctx, t.AgentID, pkg)
		if err != nil {
			t.State = model.DeploymentTargetRolledBack
			t.Message = "rollback stage: " + err.Error()
			continue
		}
		_, err = o.agent.ApplyUpdate(ctx, t.AgentID, d.ProcessName, staged, pkg.Checksum)
		if err != nil {
			t.State = model.DeploymentTargetRolledBack
			t.Message = "rollback apply: " + err.Error()
			continue
		}
		t.State = model.DeploymentTargetRolledBack
		t.Message = "rolled back to " + pkg.Version
		t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	d.State = model.DeploymentStateRolledBack
	d.UpdatedAt = time.Now()
	_ = saveTargets(d, targets)
	if err := o.store.Update(ctx, d); err != nil {
		return d, err
	}
	return d, nil
}

// healthGate 等待目标机健康确认。
func (o *Orchestrator) healthGate(ctx context.Context, agentID string) bool {
	retries := o.HealthRetries
	if retries <= 0 {
		retries = 3
	}
	delay := o.HealthRetry
	if delay <= 0 {
		delay = 30 * time.Second
	}
	for i := 0; i < retries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(delay):
			}
		}
		ok, _ := o.agent.Healthy(ctx, agentID)
		if ok {
			return true
		}
	}
	return false
}

// nextBatch 返回下一待执行批号；0=全部完成。
func nextBatch(targets []model.DeploymentTarget) int {
	next := 0
	for _, t := range targets {
		if t.State == model.DeploymentTargetPending {
			if next == 0 || t.Batch < next {
				next = t.Batch
			}
		}
	}
	return next
}

func decodeTargets(raw model.JSON) []model.DeploymentTarget {
	var out []model.DeploymentTarget
	_ = json.Unmarshal(raw, &out)
	return out
}

func saveTargets(d *model.Deployment, targets []model.DeploymentTarget) error {
	b, err := json.Marshal(targets)
	if err != nil {
		return err
	}
	d.TargetsJSON = model.JSON(b)
	return nil
}
