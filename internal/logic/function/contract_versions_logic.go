package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/function/schemadiff"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

// B2：函数契约变更历史读端。以 (game_id, env, function_id) 为主维度分页
// 查询版本快照流；diff 端点按两个 seq 的快照现算（历史链路本身已逐条
// 存 diff，两版对比是任意子区间的汇总，以快照为准不拼接链上 diff）。

type ContractVersionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewContractVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ContractVersionsLogic {
	return &ContractVersionsLogic{ctx: ctx, svcCtx: svcCtx}
}

// ContractVersionDiffEntry 与写路径 contractFieldChange 同一 JSON 形态。
type ContractVersionDiffEntry struct {
	Field    string               `json:"field"`
	From     string               `json:"from,omitempty"`
	To       string               `json:"to,omitempty"`
	Change   string               `json:"change,omitempty"`
	Findings []schemadiff.Finding `json:"findings,omitempty"`
}

type ContractVersionItem struct {
	Seq          int64                      `json:"seq"`
	Version      string                     `json:"version,omitempty"`
	Source       string                     `json:"source,omitempty"`
	SourceDigest string                     `json:"sourceDigest,omitempty"`
	ChangeType   string                     `json:"changeType"`
	Breaking     bool                       `json:"breaking"`
	Actor        string                     `json:"actor,omitempty"`
	CreatedAt    string                     `json:"createdAt"`
	Diff         []ContractVersionDiffEntry `json:"diff,omitempty"`
}

type ContractVersionsResult struct {
	Items []ContractVersionItem `json:"items"`
	Total int64                 `json:"total"`
	Page  int                   `json:"page"`
	Size  int                   `json:"size"`
}

type ContractVersionDetailResult struct {
	ContractVersionItem
	Snapshot json.RawMessage `json:"snapshot,omitempty"`
}

type ContractVersionDiffResult struct {
	FromSeq  int64                      `json:"fromSeq"`
	ToSeq    int64                      `json:"toSeq"`
	Breaking bool                       `json:"breaking"`
	Changes  []ContractVersionDiffEntry `json:"changes"`
}

// RequireScope 读取请求作用域（X-Game-ID/X-Env 中间件注入）。
func (l *ContractVersionsLogic) RequireScope() svc.GameScope {
	scope := svc.GameScopeFromContext(l.ctx)
	return svc.GameScope{
		GameID: strings.TrimSpace(scope.GameID),
		Env:    strings.TrimSpace(scope.Env),
	}
}

// CheckAccess 与 DescriptorsLogic.checkReadPermission 同一准入面
// （functions:read 或 admin）。
func (l *ContractVersionsLogic) CheckAccess() error {
	return NewDescriptorsLogic(l.ctx, l.svcCtx).checkReadPermission()
}

func (l *ContractVersionsLogic) List(gameID, env, functionID string, page, pageSize int) (*ContractVersionsResult, error) {
	if l.svcCtx == nil || l.svcCtx.DB == nil {
		return nil, errorx.NewInternalError("database is not initialized")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	rows, total, err := model.NewFunctionContractVersionModel(l.svcCtx.DB).
		ListByFunctionPaged(l.ctx, gameID, env, functionID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]ContractVersionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toContractVersionItem(row))
	}
	return &ContractVersionsResult{Items: items, Total: total, Page: page, Size: pageSize}, nil
}

func (l *ContractVersionsLogic) Detail(gameID, env, functionID string, seq int64) (*ContractVersionDetailResult, error) {
	row, err := l.find(gameID, env, functionID, seq)
	if err != nil {
		return nil, err
	}
	return &ContractVersionDetailResult{
		ContractVersionItem: toContractVersionItem(row),
		Snapshot:            json.RawMessage(row.Snapshot),
	}, nil
}

func (l *ContractVersionsLogic) Diff(gameID, env, functionID string, fromSeq, toSeq int64) (*ContractVersionDiffResult, error) {
	from, err := l.find(gameID, env, functionID, fromSeq)
	if err != nil {
		return nil, err
	}
	to, err := l.find(gameID, env, functionID, toSeq)
	if err != nil {
		return nil, err
	}
	oldSpec, err := decodeSnapshotSpec(from)
	if err != nil {
		return nil, err
	}
	newSpec, err := decodeSnapshotSpec(to)
	if err != nil {
		return nil, err
	}
	changes := diffFunctionSpecs(oldSpec, newSpec)
	breaking := false
	for _, entry := range changes {
		for _, finding := range entry.Findings {
			if finding.Severity == schemadiff.SeverityBreaking {
				breaking = true
			}
		}
	}
	return &ContractVersionDiffResult{
		FromSeq:  fromSeq,
		ToSeq:    toSeq,
		Breaking: breaking,
		Changes:  changes,
	}, nil
}

func (l *ContractVersionsLogic) find(gameID, env, functionID string, seq int64) (*model.FunctionContractVersion, error) {
	if l.svcCtx == nil || l.svcCtx.DB == nil {
		return nil, errorx.NewInternalError("database is not initialized")
	}
	row, err := model.NewFunctionContractVersionModel(l.svcCtx.DB).
		FindBySeq(l.ctx, gameID, env, functionID, seq)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errorx.NewNotFound("版本不存在")
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func toContractVersionItem(row *model.FunctionContractVersion) ContractVersionItem {
	item := ContractVersionItem{
		Seq:          row.Seq,
		Version:      row.Version,
		Source:       row.Source,
		SourceDigest: row.SourceDigest,
		ChangeType:   row.ChangeType,
		Breaking:     row.Breaking,
		Actor:        row.Actor,
		CreatedAt:    row.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if len(row.Diff) > 0 {
		var entries []ContractVersionDiffEntry
		if err := json.Unmarshal(row.Diff, &entries); err == nil {
			item.Diff = entries
		}
	}
	return item
}

func decodeSnapshotSpec(row *model.FunctionContractVersion) (*spec.FunctionSpec, error) {
	if len(row.Snapshot) == 0 {
		return nil, errorx.NewInternalError("版本快照缺失")
	}
	var fn spec.FunctionSpec
	if err := json.Unmarshal(row.Snapshot, &fn); err != nil {
		return nil, errorx.NewInternalError("版本快照解析失败")
	}
	return &fn, nil
}

// diffFunctionSpecs 两个 FunctionSpec 快照的字段级 diff（Diagnostics/
// previousSchema 属噪音与派生物，不参与对比）。schema 用 schemadiff 产出
// 结构化 findings（兼容历史写路径的 diff 形态）。
func diffFunctionSpecs(oldSpec, newSpec *spec.FunctionSpec) []ContractVersionDiffEntry {
	entries := make([]ContractVersionDiffEntry, 0, 8)
	add := func(field, from, to string) {
		if from != to {
			entries = append(entries, ContractVersionDiffEntry{Field: field, From: from, To: to})
		}
	}
	addBool := func(field string, from, to bool) {
		add(field, fmt.Sprint(from), fmt.Sprint(to))
	}
	add("version", oldSpec.Version, newSpec.Version)
	addBool("enabled", oldSpec.Enabled, newSpec.Enabled)
	addBool("deprecated", oldSpec.Deprecated, newSpec.Deprecated)
	add("executionState",
		string(spec.NormalizeExecutionState(string(oldSpec.ExecutionState))),
		string(spec.NormalizeExecutionState(string(newSpec.ExecutionState))))
	add("resourceKey", oldSpec.Resource, newSpec.Resource)
	add("operationKey", oldSpec.Operation, newSpec.Operation)
	add("capability", string(oldSpec.Capability), string(newSpec.Capability))
	add("execution", string(oldSpec.Execution), string(newSpec.Execution))
	add("risk", string(oldSpec.Risk), string(newSpec.Risk))
	add("permission", oldSpec.Permission, newSpec.Permission)
	add("timeoutMs", fmt.Sprint(oldSpec.TimeoutMs), fmt.Sprint(newSpec.TimeoutMs))
	add("approval", compactJSON(oldSpec.Approval), compactJSON(newSpec.Approval))
	add("summary", compactJSON(oldSpec.Summary), compactJSON(newSpec.Summary))
	add("description", compactJSON(oldSpec.Description), compactJSON(newSpec.Description))
	add("tags", compactJSON(oldSpec.Tags), compactJSON(newSpec.Tags))
	addSchema := func(field string, oldV, newV spec.JSONSchema) {
		oldRaw, _ := json.Marshal(oldV)
		newRaw, _ := json.Marshal(newV)
		if strings.TrimSpace(string(oldRaw)) == strings.TrimSpace(string(newRaw)) || isEmptySchemaJSON(oldRaw) && isEmptySchemaJSON(newRaw) {
			return
		}
		entry := ContractVersionDiffEntry{Field: field, Change: "schema_replaced"}
		entry.Findings = schemadiff.DiffSchemas(field, oldRaw, newRaw)
		entries = append(entries, entry)
	}
	addSchema("inputSchema", oldSpec.InputSchema, newSpec.InputSchema)
	addSchema("outputSchema", oldSpec.OutputSchema, newSpec.OutputSchema)
	return entries
}

func isEmptySchemaJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null" || trimmed == "{}"
}

func compactJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
