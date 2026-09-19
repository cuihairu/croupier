package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/function/schemadiff"
	"github.com/cuihairu/croupier/internal/model"
)

// B2：函数契约版本历史写路径。以 (game_id, env, function_id) 为主维度，
// 每次内容变化追加一条快照；「是否变化」判据与 model.UpsertContract 的
// 跳过写完全一致（model.ContractSemanticallyEqual），保证不存在
// 「写了历史没落契约」或「契约变了没历史」的漂移。

const (
	ContractVersionChangeCreated = "created"
	ContractVersionChangeUpdated = "updated"
	ContractVersionChangeRemoved = "removed"
)

// contractFieldChange diff_json 条目：标量字段记 from/to；schema 类字段
// 不存整值（快照已含全量），只记 change 与 schemadiff findings。
type contractFieldChange struct {
	Field    string               `json:"field"`
	From     string               `json:"from,omitempty"`
	To       string               `json:"to,omitempty"`
	Change   string               `json:"change,omitempty"`
	Findings []schemadiff.Finding `json:"findings,omitempty"`
}

// contractContentChanged 复刻 UpsertContract 的写入判据（service 侧视角：
// existing 由 FindByScopeAndFunctionID 加载，软删行按 NotFound 返回 nil，
// 即「复活」在这里天然是 created，与 model 侧 Unscoped 命中软删后必写一致）。
func contractContentChanged(existing, contract *model.FunctionContract) bool {
	if existing == nil {
		return true
	}
	if !existing.DeletedAt.Time.IsZero() {
		return true
	}
	return !model.ContractSemanticallyEqual(existing, contract)
}

func contractScalarDiff(existing, contract *model.FunctionContract) []contractFieldChange {
	if existing == nil {
		return nil
	}
	entries := make([]contractFieldChange, 0, 8)
	add := func(field, from, to string) {
		if from != to {
			entries = append(entries, contractFieldChange{Field: field, From: from, To: to})
		}
	}
	addBool := func(field string, from, to bool) {
		add(field, jsonBool(from), jsonBool(to))
	}
	add("version", existing.Version, contract.Version)
	addBool("enabled", existing.Enabled, contract.Enabled)
	addBool("deprecated", existing.Deprecated, contract.Deprecated)
	add("resourceKey", existing.ResourceKey, contract.ResourceKey)
	add("operationKey", existing.OperationKey, contract.OperationKey)
	add("capability", existing.Capability.String(), contract.Capability.String())
	add("execution", existing.Execution, contract.Execution)
	add("executionState", existing.ExecutionState, contract.ExecutionState)
	add("timeoutMs", jsonInt(int(existing.TimeoutMs)), jsonInt(int(contract.TimeoutMs)))
	add("risk", existing.Risk.String(), contract.Risk.String())
	add("permission", existing.Permission, contract.Permission)
	add("approval", jsonCompact(existing.Approval), jsonCompact(contract.Approval))
	add("summary", jsonCompact(existing.Summary), jsonCompact(contract.Summary))
	add("description", jsonCompact(existing.Description), jsonCompact(contract.Description))
	add("tags", string(existing.Tags), string(contract.Tags))
	return entries
}

func schemaDiffEntries(
	findings []schemadiff.Finding,
	inputChanged, outputChanged bool,
) []contractFieldChange {
	entries := make([]contractFieldChange, 0, 2)
	for _, item := range []struct {
		field   string
		changed bool
	}{{"inputSchema", inputChanged}, {"outputSchema", outputChanged}} {
		if !item.changed {
			continue
		}
		entry := contractFieldChange{Field: item.field, Change: "schema_replaced"}
		for _, finding := range findings {
			if finding.Source == item.field {
				entry.Findings = append(entry.Findings, finding)
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

// appendContractVersion 记录 created/updated 历史。错误由调用方降级为告警
// （历史是衍生审计数据，与 proposals/templates 重建同档；表存在性由
// 迁移 0029 + 启动期 MinimumRequiredVersion=29 保证）。
func (s *ContractService) appendContractVersion(
	ctx context.Context,
	existing, contract *model.FunctionContract,
	findings []schemadiff.Finding,
) error {
	if s.contractVersions == nil || contract == nil {
		return nil
	}
	changeType := ContractVersionChangeUpdated
	if existing == nil {
		changeType = ContractVersionChangeCreated
	}
	diff := contractScalarDiff(existing, contract)
	if existing != nil {
		diff = append(diff, schemaDiffEntries(
			findings,
			!canonicalEqualJSON(existing.InputSchema, contract.InputSchema),
			!canonicalEqualJSON(existing.OutputSchema, contract.OutputSchema),
		)...)
	}
	snapshot, err := json.Marshal(FunctionSpecFromContract(contract))
	if err != nil {
		return err
	}
	return s.contractVersions.AppendVersion(ctx, &model.FunctionContractVersion{
		GameID:       contract.GameID,
		Env:          contract.Env,
		FunctionID:   contract.FunctionID,
		Version:      contract.Version,
		Source:       contract.Source,
		SourceDigest: contract.SourceDigest,
		ChangeType:   changeType,
		Breaking:     schemadiff.HasBreaking(findings),
		Actor:        actorFromContext(ctx),
		Snapshot:     model.JSON(snapshot),
		Diff:         toJSON(diff),
	})
}

// appendRemovedContractVersion 记录 removed 历史：快照保留删除前最后一版，
// 让「函数消失」本身也成为可查询事件。
func (s *ContractService) appendRemovedContractVersion(ctx context.Context, contract *model.FunctionContract) error {
	if s.contractVersions == nil || contract == nil {
		return nil
	}
	snapshot, err := json.Marshal(FunctionSpecFromContract(contract))
	if err != nil {
		return err
	}
	return s.contractVersions.AppendVersion(ctx, &model.FunctionContractVersion{
		GameID:       contract.GameID,
		Env:          contract.Env,
		FunctionID:   contract.FunctionID,
		Version:      contract.Version,
		Source:       contract.Source,
		SourceDigest: contract.SourceDigest,
		ChangeType:   ContractVersionChangeRemoved,
		Actor:        actorFromContext(ctx),
		Snapshot:     model.JSON(snapshot),
	})
}

func jsonBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func jsonInt(v int) string {
	return strings.TrimSpace(jsonCompact(v))
}

func jsonCompact(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func canonicalEqualJSON(a, b model.JSON) bool {
	return jsonCompact(canonicalUnmarshal(a)) == jsonCompact(canonicalUnmarshal(b))
}

func canonicalUnmarshal(raw model.JSON) interface{} {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}
