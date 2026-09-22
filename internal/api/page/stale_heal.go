package page

import (
	"context"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

// staleHealActor 是系统愈合循环写审计/草稿的 actor 标识。
const staleHealActor = "system:contract-heal"

// StartStaleHealLoop 周期自动愈合「契约漂移 → 已发布页面 stale」：
// 扫描所有存在发布快照的 (game,env) scope，对评估出 bindingFreshness 的页面
// 以系统身份执行 selector 同步；能完全自动适配的（无 manual 遗留）由
// syncSelectorsCore 的自动发布接续恢复（快照与渲染形态一起前进），
// 修不了的保持 stale 并在编辑器告警等待人工。
//
// 语义边界：
//   - selector 适配不改页面内容形状（只跟随函数契约），自动发布视同
//     机器跟随，不需要人工审批；出现 governance/required 未满足等
//     manual 项时绝不自动发布；
//   - 幂等护栏（无变化不 bump revision）保证循环可高频运行；
//   - 单轮全程 ctx 超时 60s；错误只记日志（与 proposal 重建同档降级）。
func (s *Service) StartStaleHealLoop(ctx context.Context, interval time.Duration) {
	if s == nil || s.svcCtx == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.healStalePublishedPagesOnce(ctx)
		}
	}
}

type healScope struct {
	GameID string `gorm:"column:game_id"`
	Env    string `gorm:"column:env"`
}

// healScopes 枚举待扫描的 (game,env) scope 清单，按部署形态分流（U1）：
//   - 单库模式（Router 未装配）：meta 库即游戏库，直查 page_specs 的
//     distinct scope——只扫有发布快照的 scope，行为与历史版本一致；
//   - 分库模式（database-per-game）：page_specs 在 game 库、meta 库无此
//     表，旧实现在 meta 直查每轮 no-such-table 报错返回、愈合永不执行。
//     改从 game_envs 绑定表枚举 scope（meta 侧唯一事实源，与迁移 fanout
//     同源），页级查询经 scopeDBContext 落到对应 game 库。绑定存在但
//     game 库尚未懒建时，Resolve 会在首轮访问建库——有发布页的 scope
//     必然建过库；纯绑定无页的 scope 空转一轮页级空查询，无副作用。
func (s *Service) healScopes(ctx context.Context, metaDB *gorm.DB) ([]healScope, error) {
	if s.svcCtx.Router == nil {
		var scopes []healScope
		err := metaDB.WithContext(ctx).
			Table("page_specs").
			Where("deleted_at IS NULL AND published_version > 0").
			Distinct("game_id", "env").
			Scan(&scopes).Error
		return scopes, err
	}
	bindings, err := model.NewGameModel(metaDB).ListAllEnvBindings(ctx)
	if err != nil {
		return nil, err
	}
	scopes := make([]healScope, 0, len(bindings))
	for _, b := range bindings {
		scopes = append(scopes, healScope{GameID: b.GameID, Env: b.Env})
	}
	return scopes, nil
}

// healStalePublishedPagesOnce 执行一轮扫描（导出仅包内，测试直调）。
func (s *Service) healStalePublishedPagesOnce(ctx context.Context) {
	loopCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	metaDB := s.svcCtx.DB
	scopes, err := s.healScopes(loopCtx, metaDB)
	if err != nil {
		slog.Warn("stale heal: list scopes failed", "error", err)
		return
	}

	healed, skipped := 0, 0
	for _, sc := range scopes {
		scopeCtx, err := s.scopeDBContext(loopCtx, sc.GameID, sc.Env)
		if err != nil {
			slog.Warn("stale heal: resolve scope db failed", "gameId", sc.GameID, "env", sc.Env, "error", err)
			continue
		}
		pages, err := s.svcCtx.PageSpecModel.ListByScope(scopeCtx, sc.GameID, sc.Env)
		if err != nil {
			slog.Warn("stale heal: list pages failed", "gameId", sc.GameID, "env", sc.Env, "error", err)
			continue
		}
		for i := range pages {
			p := pages[i]
			if p.PublishedVersion == 0 {
				continue
			}
			if len(s.bindingFreshnessForPublishedDraft(scopeCtx, &p)) == 0 {
				continue
			}
			// 重新精确加载（ListByScope 的对象可能缺 latest 字段），
			// core 内部带乐观锁：并发保存会 Conflict 跳过，下轮再来。
			fresh, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(scopeCtx, sc.GameID, sc.Env, p.PageKey)
			if err != nil {
				continue
			}
			resp, err := s.syncSelectorsCore(scopeCtx, sc.GameID, sc.Env, staleHealActor, fresh, nil, false, fresh.DraftRevision)
			if err == nil && resp.Applied && resp.DraftRevision > 0 && healSafeReports(resp.SyncedBindings) && fresh.PublishedVersion > 0 {
				// 系统身份直发（无请求 ctx 可核权限，动作语义 = 机器跟随契约）
				rev := resp.DraftRevision
				if _, perr := s.publishCore(scopeCtx, sc.GameID, sc.Env, staleHealActor, &PagePublishRequest{PageKey: fresh.PageKey, DraftRevision: &rev}); perr == nil {
					resp.AutoPublished = true
				}
			}
			if err != nil {
				// Conflict/校验失败：保留 stale 提示，不算系统错误风暴
				continue
			}
			if resp.AutoPublished {
				healed++
			} else {
				skipped++
			}
		}
	}
	if healed > 0 || skipped > 0 {
		slog.Info("stale heal round done", "autoPublished", healed, "awaitingManual", skipped)
	}
}

// scopeDBContext 为后台循环重建请求等价上下文：注入 GameScope（normalized
// Functions / requireScope 依赖）与 game DB（multiGame 模式经 Router 懒加载；
// 单库模式仅注入 scope）。
func (s *Service) scopeDBContext(ctx context.Context, gameID, env string) (context.Context, error) {
	scopeCtx := svc.WithGameScope(ctx, svc.GameScope{GameID: gameID, Env: env})
	if s.svcCtx.Router == nil {
		return scopeCtx, nil
	}
	scopeCtx, gdb, err := s.svcCtx.Router.Resolve(scopeCtx, gameID, env)
	if err != nil {
		return nil, err
	}
	return dbctx.WithDB(scopeCtx, gdb), nil
}

// healSafeReports 是系统愈合循环的自动发布严门（比用户手动同步更保守）。
// rename 看似机械跟随，实为语义判断（实测单候选 rename 推断可把 uid 映射到
// couponCode——prev 唯一候选启发式无法区分「改名」与「删旧增新」），因此：
//   - 输入侧仅放行 kept / removed（无新语义）；
//   - 输出侧放行 kept / removed / shape_updated（形状跟随，无字段语义）；
//   - renamed / added / type_changed / manual_required 一律只同步草稿，
//     发布留给人（编辑器一键同步带 publish 权限即可自动接续发布）。
func healSafeReports(reports []spec.BindingSelectorSyncReport) bool {
	for _, r := range reports {
		if len(r.Manual) > 0 {
			return false
		}
		for _, e := range r.Input {
			if e.Action != spec.SelectorSyncKept && e.Action != spec.SelectorSyncRemoved {
				return false
			}
		}
		for _, e := range r.Output {
			switch e.Action {
			case spec.SelectorSyncKept, spec.SelectorSyncRemoved, spec.SelectorSyncShapeUpdated:
			default:
				return false
			}
		}
	}
	return true
}
