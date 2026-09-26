package svc

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
)

// 演示审批种子（dev 模式）：让 approvals 页面在全新部署时即有
// approved / rejected / pending 三态记录可看，并留出可现场演示的两人复核流程。
//
// 记录形态与真实写入路径一致：
//   - pending 记录对齐 createFunctionApproval（func_<id>_<nanots>、Route=lb、
//     Payload 为调用参数 JSON）；
//   - reviewed 记录对齐 MemStore.Approve/Reject（Approver + ReviewedAt + Reason）。
//
// pending 的申请人刻意与演示账号（admin）错开，使 admin 登录后可直接复核
// （self_approval 规则拦截申请人自审）。MemStore 不落盘、每次启动重建，
// 种子天然幂等，无需去重逻辑。

const demoApprovalSeedGameID = "default"

// demoApprovalScopesMaxEnvs 单个游戏最多播种的环境数，约束记录总量。
const demoApprovalScopesMaxEnvs = 6

type demoApprovalFixture struct {
	functionID  string
	state       string // pending|approved|rejected
	actor       string
	approver    string            // reviewed 记录的复核人
	reviewAfter time.Duration     // reviewed 记录的审批耗时
	age         time.Duration     // 距 now 的创建时间
	mode        string            // sync|async
	reason      string            // rejected 记录的拒绝理由
	payload     string            // 调用参数 JSON
	metadata    map[string]string // 额外元数据（如转审来源 delegatedFrom）
}

// demoApprovalFixtures 单个 scope 的演示记录。UpdatedAt 排序下 pending 两条
// 置顶，三种状态在列表首页交织可见。审批人覆盖 admin / reviewer01 / reviewer02
// 三个不同账号（对应「运营申请—安全复核」「客服申请—管理员复核」等跨角色
// 组合），其中一条带 delegatedFrom 元数据演示转审（委托审批）场景。
var demoApprovalFixtures = []demoApprovalFixture{
	{
		functionID: "player.unban",
		state:      "pending",
		actor:      "operator",
		age:        35 * time.Minute,
		mode:       "sync",
		payload:    `{"durationHours":0,"operator":"operator","playerId":"100877","reason":"申诉核实通过，解除封禁"}`,
	},
	{
		functionID: "config.update",
		state:      "pending",
		actor:      "gm01",
		age:        2*time.Hour + 10*time.Minute,
		mode:       "sync",
		payload:    `{"key":"shop.discount.rate","newValue":0.8,"oldValue":1.0,"scope":"global"}`,
	},
	{
		functionID:  "config.update",
		state:       "rejected",
		actor:       "operator",
		approver:    "admin",
		reviewAfter: 12 * time.Minute,
		age:         8 * time.Hour,
		mode:        "sync",
		reason:      "影响面过大，请先在 stage 环境验证后再提交",
		payload:     `{"key":"pvp.season.length","newValue":14,"oldValue":21,"scope":"global"}`,
	},
	{
		functionID:  "player.ban",
		state:       "approved",
		actor:       "operator",
		approver:    "admin",
		reviewAfter: 25 * time.Minute,
		age:         26 * time.Hour,
		mode:        "sync",
		payload:     `{"durationHours":720,"operator":"operator","playerId":"100423","reason":"外挂脚本行为封禁"}`,
	},
	{
		functionID:  "mail.send",
		state:       "approved",
		actor:       "gm01",
		approver:    "admin",
		reviewAfter: 40 * time.Minute,
		age:         49 * time.Hour,
		mode:        "async",
		payload:     `{"attachments":[],"content":"恭喜完成 S3 赛季，奖励已随信发放。","receiverRange":"all","title":"S3 赛季结算奖励"}`,
	},
	{
		functionID:  "config.update",
		state:       "approved",
		actor:       "gm01",
		approver:    "reviewer01",
		reviewAfter: 9 * time.Minute,
		age:         5 * time.Hour,
		mode:        "sync",
		payload:     `{"key":"match.mmr.k","newValue":32,"oldValue":40,"scope":"global"}`,
		metadata:    map[string]string{"delegatedFrom": "reviewer02"},
	},
	{
		functionID:  "player.unban",
		state:       "rejected",
		actor:       "gm01",
		approver:    "reviewer01",
		reviewAfter: 18 * time.Minute,
		age:         30 * time.Hour,
		mode:        "sync",
		reason:      "封禁依据不足，请补充证据链后再提交",
		payload:     `{"durationHours":0,"operator":"gm01","playerId":"100511","reason":"误封申诉"}`,
	},
	{
		functionID:  "mail.send",
		state:       "approved",
		actor:       "operator",
		approver:    "reviewer02",
		reviewAfter: 33 * time.Minute,
		age:         52 * time.Hour,
		mode:        "async",
		payload:     `{"attachments":[],"content":"服务器将于周日 02:00-04:00 停机维护。","receiverRange":"all","title":"停机维护公告"}`,
	},
}

// seedDemoApprovals 开发模式下向内存审批库写入演示数据（每个 scope
// 4 approved + 2 rejected + 2 pending，审批人覆盖 admin/reviewer01/reviewer02）。
// 放在 seedBootstrapGames 之后调用，以便从 game_envs 绑定读取真实可用的 scope。
func seedDemoApprovals(ctx *ServiceContext) {
	if ctx == nil || !isDevelopmentConfig(ctx.Config) || ctx.ApprovalsStore == nil || ctx.GameModel == nil {
		return
	}
	bindings, err := ctx.GameModel.ListAllEnvBindings(context.Background())
	if err != nil {
		slog.Default().Warn("demo approval seed skipped: list env bindings failed", "error", err)
		return
	}
	scopes := demoApprovalScopes(bindings)
	created := seedDemoApprovalsInto(ctx.ApprovalsStore, scopes, time.Now())
	if created > 0 {
		slog.Default().Info("seed demo approvals", "records", created, "scopes", len(scopes))
	}
}

// demoApprovalScopes 返回要播种的 scope：优先取 game_envs 绑定中演示游戏
// （default）的环境；没有绑定或没有该游戏时，回退到 (game_id, env) 排序
// 第一的游戏——与 resolveFirstAuthorizedGame 的「首个授权 scope」一致，
// 保证种子在管理员登录后的默认 scope 下可见；完全没有绑定时兜底
// default × fallbackDefaultEnvs。
func demoApprovalScopes(bindings []model.GameEnvBinding) []GameScope {
	scoped := make([]model.GameEnvBinding, 0, len(bindings))
	for _, b := range bindings {
		if strings.TrimSpace(b.GameID) == demoApprovalSeedGameID {
			scoped = append(scoped, b)
		}
	}
	if len(scoped) == 0 && len(bindings) > 0 {
		sorted := append([]model.GameEnvBinding(nil), bindings...)
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].GameID != sorted[j].GameID {
				return sorted[i].GameID < sorted[j].GameID
			}
			return sorted[i].Env < sorted[j].Env
		})
		first := sorted[0].GameID
		for _, b := range sorted {
			if b.GameID == first {
				scoped = append(scoped, b)
			}
		}
	}

	scopes := make([]GameScope, 0, len(scoped))
	seen := make(map[string]struct{}, len(scoped))
	for _, b := range scoped {
		gameID := strings.TrimSpace(b.GameID)
		env := strings.TrimSpace(b.Env)
		if gameID == "" || env == "" {
			continue
		}
		if _, ok := seen[env]; ok {
			continue
		}
		seen[env] = struct{}{}
		scopes = append(scopes, GameScope{GameID: gameID, Env: env})
		if len(scopes) >= demoApprovalScopesMaxEnvs {
			break
		}
	}
	if len(scopes) > 0 {
		return scopes
	}
	for _, env := range fallbackDefaultEnvs {
		scopes = append(scopes, GameScope{GameID: demoApprovalSeedGameID, Env: env.Env})
	}
	return scopes
}

// seedDemoApprovalsInto 按给定 scope 写入演示记录，返回成功写入条数。
// scopeIdx 微移创建时间使 ID（含纳秒时间戳）跨 scope 不碰撞。
func seedDemoApprovalsInto(store approvals.Store, scopes []GameScope, now time.Time) int {
	if store == nil || len(scopes) == 0 {
		return 0
	}
	created := 0
	for scopeIdx, scope := range scopes {
		for _, f := range demoApprovalFixtures {
			createdAt := now.Add(-f.age).Add(time.Duration(scopeIdx) * time.Microsecond)
			metadata := map[string]string{"source": "console"}
			for k, v := range f.metadata {
				metadata[k] = v
			}
			record := &approvals.Approval{
				ID:             fmt.Sprintf("func_%s_%d", f.functionID, createdAt.UnixNano()),
				State:          f.state,
				FunctionID:     f.functionID,
				GameID:         scope.GameID,
				Env:            scope.Env,
				Actor:          f.actor,
				Mode:           f.mode,
				Route:          "lb",
				IdempotencyKey: fmt.Sprintf("seed-%s-%s-%s-%s", scope.Env, f.functionID, f.state, f.approver),
				Payload:        []byte(f.payload),
				Metadata:       metadata,
				Reason:         f.reason,
				CreatedAt:      createdAt,
				UpdatedAt:      createdAt,
			}
			if f.state == "approved" || f.state == "rejected" {
				reviewedAt := createdAt.Add(f.reviewAfter)
				record.Approver = f.approver
				record.ReviewedAt = &reviewedAt
				record.UpdatedAt = reviewedAt
			}
			if _, err := store.Create(record); err != nil {
				slog.Default().Warn("skip demo approval seed record",
					"id", record.ID, "error", err)
				continue
			}
			created++
		}
	}
	return created
}
