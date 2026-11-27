package functioninvoke

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/gameops"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

// DurableIdempotency 是 x-durable-idempotency 能力声明（GAME-009 批准协议
// D3）的运行时形态：平台据此判断高风险业务函数的游戏侧自报去重面，并把
// query/cancel 提示落进凭据供 D4/D5 收敛链调用。空值 = 未声明，平台不发明
// 去重行为。
type DurableIdempotency struct {
	Query  string
	Cancel string
	Undo   string
}

func (d *DurableIdempotency) declared() bool {
	return d.Query != "" || d.Cancel != "" || d.Undo != ""
}

// resolveDurableIdempotency 读取函数 OpenAPI 操作扩展的幂等能力声明，口径与
// resolvePolicyRisk 同构（同一 RegistryStore 读取面）：
//   - 未声明扩展（含函数不存在 ErrOpenAPINotFound）→ (nil, nil)；
//   - 读取故障 → 向上传播 fail-closed（能力判定不得被静默当作「未声明」放行）。
func resolveDurableIdempotency(ctx context.Context, svcCtx *svc.ServiceContext, functionID string) (*DurableIdempotency, error) {
	if svcCtx == nil || svcCtx.RegistryStore == nil {
		return nil, nil
	}
	op, err := svcCtx.RegistryStore.GetOpenAPI(functionID)
	if err != nil {
		if errors.Is(err, registry.ErrOpenAPINotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取函数 OpenAPI durable 幂等声明失败（fail-closed）: %w", err)
	}
	if op == nil {
		return nil, nil
	}
	raw, exists := op.Extensions["x-durable-idempotency"]
	if !exists || raw == nil {
		return nil, nil
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("x-durable-idempotency 声明格式非法（须为对象）")
	}
	capability := &DurableIdempotency{}
	capability.Query, _ = obj["query"].(string)
	capability.Cancel, _ = obj["cancel"].(string)
	capability.Undo, _ = obj["undo"].(string)
	if !capability.declared() {
		return nil, nil
	}
	return capability, nil
}

// operationContext 是一次调用在 durable 幂等协议下的上下文：操作标识、
// 参数指纹与能力声明。nil = 本次调用不走 durable 幂等路径（函数未声明能力）。
type operationContext struct {
	operationID string
	fingerprint string
	capability  *DurableIdempotency
	credential  *model.BusinessOperationCredential
	store       *gameops.CredentialStore
	// mode 是本次调用模式；async 任务的凭据保持 pending（任务完成收敛是边界，
	// 见 review 文档 §5），sync 路径按信封/未知收敛。
	mode string
}

// beginDurableOperation 在 Govern 放行、派发之前登记业务操作凭据（D1/D2/D5）：
//
//   - 高危函数（x-risk high/danger）必须声明 x-durable-idempotency 能力，
//     否则按 D3 fail-closed 拒绝（能力是准入项，不是可选项）；
//   - operationId 由平台生成（D1 复合标识，UUIDv7 意图段）；调用方显式带入的
//     operationId 仅允许用于响应丢失后的同参数重放，参数指纹不一致即 409（R5）；
//   - broadcast 路由是语义错误：一次业务意图只能落一个游戏侧操作，拒绝执行；
//   - 重放命中既有凭据（同 operationId 同指纹）不新建行、不重复执行——执行与否
//     仍由 dispatch 承担，本层只保证凭据唯一。
func beginDurableOperation(ctx context.Context, svcCtx *svc.ServiceContext, req *Request, adminUsername string) (*operationContext, error) {
	capability, err := resolveDurableIdempotency(ctx, svcCtx, req.ID)
	if err != nil {
		return nil, err
	}
	if capability == nil {
		return nil, nil
	}

	// D3 准入：高危业务函数必须声明 durable 幂等能力。风险档位与治理链
	// （resolvePolicyRisk）同一口径；解析故障向上传播 fail-closed。
	riskLevel, err := resolvePolicyRisk(ctx, svcCtx, req.ID, req.GameID, req.Env)
	if err != nil {
		return nil, err
	}
	risk := string(riskLevel)
	if (risk == "high" || risk == "danger") && !capability.declared() {
		return nil, errorx.NewValidationErrorWithDetails(
			"高危函数必须声明 x-durable-idempotency 能力（GAME-009 D3）",
			map[string]string{"functionId": strings.TrimSpace(req.ID)},
		)
	}

	if strings.EqualFold(strings.TrimSpace(req.Route), "broadcast") {
		return nil, errorx.NewBadRequest("x-durable-idempotency 函数不支持 route=broadcast（一次业务意图只能执行一次）")
	}

	fingerprint := gameops.FingerprintParams(req.ID, InvokePayload(req))

	operationID := strings.TrimSpace(req.OperationID)
	if operationID != "" {
		if err := gameops.ValidateOperationID(operationID); err != nil {
			return nil, errorx.NewBadRequestWithCode("invalid_operation_id", "operationId 格式非法（GAME-009 D1）", map[string]any{"reason": err.Error()})
		}
	} else {
		domain := deriveOperationDomain(ctx, svcCtx, req.ID)
		generated, err := gameops.GenerateOperationID(req.GameID, req.Env, domain)
		if err != nil {
			return nil, fmt.Errorf("生成 operationId 失败: %w", err)
		}
		operationID = generated
	}

	if svcCtx == nil || svcCtx.DB == nil {
		return nil, errorx.NewInternalError("durable operation needs database")
	}
	store := gameops.NewCredentialStore(svcCtx.DB)
	credential, _, err := store.Register(ctx, req.GameID, req.Env, operationID, req.ID, fingerprint, adminUsername, capability.Query, capability.Cancel)
	if err != nil {
		if errors.Is(err, gameops.ErrFingerprintConflict) {
			return nil, errorx.NewConflictWithDetails(
				"operationId 已绑定不同参数（参数指纹冲突，R5 拒绝改写）",
				map[string]any{"operationId": operationID},
			)
		}
		return nil, fmt.Errorf("登记业务操作凭据失败: %w", err)
	}
	return &operationContext{
		operationID: operationID,
		fingerprint: fingerprint,
		capability:  capability,
		credential:  credential,
		store:       store,
		mode:        strings.TrimSpace(req.Mode),
	}, nil
}

// deriveOperationDomain 取 operationId 的业务域段（D1 第三段）：优先 x-resource
// 扩展；缺失时降级为函数标识（规范化为 [a-z0-9-]），保证标识格式合法且
// 唯一性由意图 UUID 段承担，域段归一化不影响唯一性。
func deriveOperationDomain(ctx context.Context, svcCtx *svc.ServiceContext, functionID string) string {
	if svcCtx != nil && svcCtx.RegistryStore != nil {
		if op, err := svcCtx.RegistryStore.GetOpenAPI(functionID); err == nil && op != nil {
			if resource, ok := op.Extensions["x-resource"].(string); ok && strings.TrimSpace(resource) != "" {
				return normalizeOperationIDSegment(resource)
			}
		}
	}
	return normalizeOperationIDSegment(functionID)
}

// normalizeOperationIDSegment 把小写字母/数字/连字符以外的字符替换为连字符，
// 首尾去连字符（D1 标识格式 [a-z0-9:-]）。
func normalizeOperationIDSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// finalizeDurableOperation 在派发返回后按 D3/D4 收敛凭据：
//
//   - 派发成功（同步/broadcast 返回响应体）：响应体即游戏信封，解析后迁移
//     凭据（终态保护在 store 内）；响应不是合法信封 → unknown（绝不谎称
//     成功或失败，R2）；
//   - 派发失败（传输层错误：超时/连接/服务不可用）：凭据落 unknown，等待 D4
//     查询收敛；
//   - async 任务：凭据保持 pending（任务完成收敛是边界，见 review 文档 §5）。
//
// 返回值为凭据当前 outcome 与操作标识，供 Response 透出。
func finalizeDurableOperation(ctx context.Context, op *operationContext, result *Response, invokeErr error) (outcome string, err error) {
	if op == nil {
		return "", nil
	}
	if invokeErr != nil {
		updated, markErr := op.store.MarkUnknownOnTransportFailure(ctx, op.credential.GameID, op.credential.Env, op.operationID, op.credential.FunctionID, op.fingerprint, op.credential.Actor)
		if markErr != nil {
			return model.BizOpOutcomeUnknown, markErr
		}
		return updated.Outcome, nil
	}
	if strings.EqualFold(op.mode, "async") {
		return model.BizOpOutcomePending, nil
	}
	if result == nil || len(result.Result) == 0 {
		// 无响应体：未知，等查询收敛。
		updated, markErr := op.store.MarkUnknownOnTransportFailure(ctx, op.credential.GameID, op.credential.Env, op.operationID, op.credential.FunctionID, op.fingerprint, op.credential.Actor)
		if markErr != nil {
			return model.BizOpOutcomeUnknown, markErr
		}
		return updated.Outcome, nil
	}
	envelope, parseErr := gameops.ParseEnvelope(op.operationID, op.fingerprint, result.Result)
	if parseErr != nil {
		// 收到响应但不是合法信封：按未知收敛，不发明终态。
		updated, markErr := op.store.MarkUnknownOnTransportFailure(ctx, op.credential.GameID, op.credential.Env, op.operationID, op.credential.FunctionID, op.fingerprint, op.credential.Actor)
		if markErr != nil {
			return model.BizOpOutcomeUnknown, markErr
		}
		return updated.Outcome, nil
	}
	updated, _, applyErr := op.store.ApplyEnvelope(ctx, op.credential.GameID, op.credential.Env, op.operationID, envelope)
	if applyErr != nil {
		return envelope.Outcome, applyErr
	}
	if updated != nil {
		return updated.Outcome, nil
	}
	return envelope.Outcome, nil
}
