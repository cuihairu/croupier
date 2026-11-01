package svc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/provision"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// provisionDBStore 适配 model.ProvisionRunModel 到 provision.RunStore。
// 敏感字段（BootstrapToken）绝不下库——runToModel 只落非敏感快照。
type provisionDBStore struct{ m *model.ProvisionRunModel }

func newProvisionDBStore(m *model.ProvisionRunModel) *provisionDBStore {
	return &provisionDBStore{m: m}
}

func (s *provisionDBStore) Create(ctx context.Context, r *provision.Run) error {
	return s.m.Create(ctx, runToModel(r))
}

func (s *provisionDBStore) Update(ctx context.Context, r *provision.Run) error {
	return s.m.Update(ctx, runToModel(r))
}

func (s *provisionDBStore) Get(ctx context.Context, id string) (*provision.Run, error) {
	row, err := s.m.FindByRunID(ctx, id)
	if err != nil {
		return nil, err
	}
	return runFromModel(row), nil
}

func (s *provisionDBStore) List(ctx context.Context, limit int) ([]*provision.Run, error) {
	rows, err := s.m.List(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*provision.Run, 0, len(rows))
	for i := range rows {
		out = append(out, runFromModel(&rows[i]))
	}
	return out, nil
}

func marshalJSON(v any) model.JSON {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func runToModel(r *provision.Run) *model.ProvisionRun {
	row := &model.ProvisionRun{
		RunID:            r.ID,
		Provider:         string(r.Request.Provider),
		Region:           r.Request.Region,
		GameID:           r.Request.GameID,
		Env:              r.Request.Env,
		Policy:           string(r.Request.Policy),
		Count:            r.Request.Count,
		State:            string(r.State),
		ApprovalID:       r.ApprovalID,
		ErrorMsg:         r.Error,
		RequestedBy:      r.Request.RequestedBy,
		DestroyProtected: r.DestroyProtected,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
	row.SpecJSON = marshalJSON(map[string]any{
		"instanceType": r.Request.Spec.InstanceType,
		"image":        r.Request.Spec.Image,
		"diskSizeGB":   r.Request.Spec.DiskSizeGB,
	})
	if r.Outputs != nil {
		row.OutputsJSON = marshalJSON(r.Outputs)
	}
	if r.AgentIDs != nil {
		row.AgentIDsJSON = marshalJSON(r.AgentIDs)
	}
	if r.Request.Labels != nil {
		row.LabelsJSON = marshalJSON(r.Request.Labels)
	}
	return row
}

func runFromModel(row *model.ProvisionRun) *provision.Run {
	r := &provision.Run{
		ID:               row.RunID,
		State:            provision.State(row.State),
		ApprovalID:       row.ApprovalID,
		Error:            row.ErrorMsg,
		DestroyProtected: row.DestroyProtected,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
	spec := map[string]any{}
	_ = json.Unmarshal(row.SpecJSON, &spec)
	if v, ok := spec["instanceType"].(string); ok {
		r.Request.Spec.InstanceType = v
	}
	if v, ok := spec["image"].(string); ok {
		r.Request.Spec.Image = v
	}
	if v, ok := spec["diskSizeGB"].(float64); ok {
		r.Request.Spec.DiskSizeGB = int(v)
	}
	r.Request.Provider = provision.Provider(row.Provider)
	r.Request.Region = row.Region
	r.Request.GameID = row.GameID
	r.Request.Env = row.Env
	r.Request.Policy = provision.PolicyMode(row.Policy)
	r.Request.Count = row.Count
	r.Request.RequestedBy = row.RequestedBy
	_ = json.Unmarshal(row.OutputsJSON, &r.Outputs)
	_ = json.Unmarshal(row.AgentIDsJSON, &r.AgentIDs)
	_ = json.Unmarshal(row.LabelsJSON, &r.Request.Labels)
	return r
}

// provisionApprovalGate 适配 approvals.Store 到 provision.ApprovalGate。
// 审批记录以 Metadata 关联供给运行 id，批准状态经 Store.Get 读取。
type provisionApprovalGate struct{ store approvals.Store }

func newProvisionApprovalGate(store approvals.Store) *provisionApprovalGate {
	return &provisionApprovalGate{store: store}
}

func (g *provisionApprovalGate) CreateApproval(ctx context.Context, req *provision.Request, runID string) (string, error) {
	a, err := g.store.Create(&approvals.Approval{
		FunctionID: "ops.provision",
		GameID:     req.GameID,
		Env:        req.Env,
		Actor:      req.RequestedBy,
		Mode:       "provision",
		Route:      "/ops/provision",
		Metadata:   map[string]string{"provisionRunId": runID},
	})
	if err != nil {
		return "", err
	}
	return a.ID, nil
}

func (g *provisionApprovalGate) Approved(ctx context.Context, approvalID string) (bool, error) {
	a, err := g.store.Get(approvalID)
	if err != nil {
		return false, err
	}
	return a.State == "approved", nil
}

// provisionAgentRegistry 适配 reg.Store 到 provision.AgentRegistry：轮询在线
// agent 会话，按供给标签（ProvisionLabelKey=runID）匹配——agent 的 AgentID
// 由 agent 自身决定，实例 id 不作为等待判定。
type provisionAgentRegistry struct{ store *reg.Store }

func newProvisionAgentRegistry(store *reg.Store) *provisionAgentRegistry {
	return &provisionAgentRegistry{store: store}
}

func (g *provisionAgentRegistry) WaitRegistered(ctx context.Context, labelKey, labelValue string, timeout time.Duration) ([]string, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		var ids []string
		for id, s := range g.store.AgentsUnsafe() {
			if s.Labels != nil && s.Labels[labelKey] == labelValue {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			return ids, nil
		}
		if time.Now().After(deadline) {
			return nil, nil // 超时：无注册（调用方判失败）
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
