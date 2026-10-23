package page

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// staleHealActor 是系统愈合循环写审计/提案/发布快照的 actor 标识。
const staleHealActor = "system:contract-heal"

const staleHealGeneratorVersion = "contract-selector-sync"

type healStaleOptions struct {
	AllowAutoPublish bool
}

// StartStaleHealLoop 周期处理「契约漂移 → 已发布页面 stale」。生产路由
// 只传零值选项：系统默认只生成稳定 key 的变更提案；只有调用方显式
// AllowAutoPublish=true 时，才基于当前 PublishedPageSpec 生成独立修订。
//
// 语义边界：
//   - 人工未发布草稿（标题/布局/按钮等）不读作愈合输入，也不被系统
//     同步或连带发布；
//   - selector 适配只允许无歧义子集，出现 renamed/added/type_changed、
//     governance/function-version 漂移等 manual 项一律要求人工处理；
//   - 自动发布在同一事务内重新校验发布基线并用 page_versions 的全局
//     下一版本落独立修订（绝不复用/推进 DraftRevision），并发发布即冲突；
//   - 默认路径只写 page_proposals（+版本快照），不写 page_specs 与
//     page_versions，已存在的同内容提案不再重建（尊重人工接受/拒绝）；
//   - 单轮全程 ctx 超时 60s；错误只记日志（与 proposal 重建同档降级）。
func (s *Service) StartStaleHealLoop(ctx context.Context, interval time.Duration, options healStaleOptions) {
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
			s.healStalePublishedPagesOnce(ctx, options)
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
func (s *Service) healStalePublishedPagesOnce(ctx context.Context, options healStaleOptions) {
	loopCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	loopCtx = context.WithValue(loopCtx, "username", staleHealActor)
	defer cancel()

	metaDB := s.svcCtx.DB
	scopes, err := s.healScopes(loopCtx, metaDB)
	if err != nil {
		slog.Warn("stale heal: list scopes failed", "error", err)
		return
	}

	proposed, published, skipped := 0, 0, 0
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
			revision, err := s.planStaleHealRevision(scopeCtx, sc.GameID, sc.Env, fresh)
			if err != nil {
				continue
			}
			if revision == nil {
				continue
			}
			// 自动发布只允许「无歧义 selector 修复 + 页面无其余发布级错误」
			// 且调用方显式开启；其余一律落提案。两条路径都不写 page_specs：
			// 人工未发布草稿既不参与规划，也绝不会被同步或连带发布。
			if options.AllowAutoPublish && revision.Safe && countErrors(revision.Remaining) == 0 {
				if _, perr := s.publishStaleHealRevision(scopeCtx, sc.GameID, sc.Env, revision); perr != nil {
					continue
				}
				published++
				continue
			}
			created, perr := s.upsertStaleHealProposal(scopeCtx, sc.GameID, sc.Env, revision)
			if perr != nil {
				continue
			}
			if created {
				proposed++
			} else {
				skipped++
			}
		}
	}
	if proposed > 0 || published > 0 || skipped > 0 {
		slog.Info("stale heal round done", "proposals", proposed, "autoPublished", published, "awaitingManual", skipped)
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

type staleHealRevision struct {
	BaseVersion   int
	Safe          bool
	Page          spec.PageSpec
	SpecJSON      string
	Contracts     []spec.BindingContractSnapshot
	ContractsJSON string
	Reports       []spec.BindingSelectorSyncReport
	Remaining     []spec.Diagnostic
	PublishSource pagePublishSource
}

// planStaleHealRevision 从当前激活 PublishedPageSpec 读取唯一愈合输入。
// 人工草稿不参与规划，因此其标题、布局或按钮修改不会进入自动修复结果。
func (s *Service) planStaleHealRevision(ctx context.Context, gameID, env string, p *model.PageSpec) (*staleHealRevision, error) {
	published, err := s.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, gameID, env, p.PageKey)
	if err != nil {
		return nil, err
	}
	// 快照里的旧契约摘要只用于判断「漂移前是否可信」，不能写回新快照：
	// 自动修复后的契约快照必须按当前函数契约重建，否则发布后仍判 stale。
	basePage, _ := parsePublishedPageForFreshness(*published)
	baseModel := model.PageSpec{PageKey: p.PageKey}
	if err := applyPageSpecToModel(&baseModel, basePage); err != nil {
		return nil, err
	}
	baseModel.GameID = gameID
	baseModel.Env = env
	baseModel.PublishedVersion = p.PublishedVersion
	baseModel.BaseProposalKey = published.BaseProposalKey
	baseModel.BaseProposalVersion = published.BaseProposalVersion

	plan, err := s.planSelectorSync(ctx, gameID, env, &baseModel, nil)
	if err != nil {
		return nil, err
	}
	if countChangedBindings(plan.reports) == 0 && countManualEntries(plan.reports) == 0 {
		return nil, nil
	}
	specJSON, err := marshalPageSpec(plan.synced)
	if err != nil {
		return nil, err
	}
	contracts, err := buildBindingContracts(plan.synced.Bindings, s.normalizedFunctions(ctx))
	if err != nil {
		return nil, err
	}
	contractsJSON, err := pageJSONMarshal(contracts)
	if err != nil {
		return nil, err
	}
	return &staleHealRevision{
		BaseVersion:   published.Version,
		Safe:          healSafeReports(plan.reports),
		Page:          plan.synced,
		SpecJSON:      specJSON,
		Contracts:     contracts,
		ContractsJSON: string(contractsJSON),
		Reports:       plan.reports,
		Remaining:     plan.remaining,
		PublishSource: pagePublishSource{
			BaseProposalKey:     published.BaseProposalKey,
			BaseProposalVersion: published.BaseProposalVersion,
			FunctionDigest:      published.FunctionDigest,
			SemanticsDigest:     published.SemanticsDigest,
			GeneratorVersion:    staleHealGeneratorVersion,
		},
	}, nil
}

// publishStaleHealRevision 基于已发布快照落独立 page_versions 修订并发布。
// 事务内重查 active 基线：一旦基线已前进即冲突返回，不覆盖并发发布。
func (s *Service) publishStaleHealRevision(ctx context.Context, gameID, env string, revision *staleHealRevision) (*PagePublishResponse, error) {
	if revision == nil {
		return nil, errorx.NewBadRequest("stale heal revision is required")
	}
	now := time.Now()
	response := &PagePublishResponse{}
	err := s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, publishedModel *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		base, err := publishedModel.FindLatestByScopeAndPageKey(txCtx, gameID, env, revision.Page.PageKey)
		if err != nil {
			return err
		}
		if base.Version != revision.BaseVersion {
			return errorx.NewConflictWithDetails("stale heal publish conflict: published base changed", map[string]any{
				"expected": revision.BaseVersion,
				"current":  base.Version,
			})
		}
		nextRevision, err := versionModel.GetNextVersion(txCtx, gameID, env, revision.Page.PageKey)
		if err != nil {
			return err
		}
		if err := publishedModel.DeactivatePage(txCtx, gameID, env, revision.Page.PageKey, now); err != nil {
			return err
		}
		if err := publishedModel.Create(txCtx, &model.PublishedPageSpec{
			GameID:                gameID,
			Env:                   env,
			PageKey:               revision.Page.PageKey,
			Version:               nextRevision,
			SpecJSON:              revision.SpecJSON,
			BindingContractsJSON:  revision.ContractsJSON,
			RendererSchemaVersion: rendererSchemaVersion,
			BaseProposalKey:       revision.PublishSource.BaseProposalKey,
			BaseProposalVersion:   revision.PublishSource.BaseProposalVersion,
			FunctionDigest:        revision.PublishSource.FunctionDigest,
			SemanticsDigest:       revision.PublishSource.SemanticsDigest,
			GeneratorVersion:      revision.PublishSource.GeneratorVersion,
			Active:                true,
			PublishedAt:           now,
			PublishedBy:           staleHealActor,
		}); err != nil {
			return err
		}
		if err := versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   revision.Page.PageKey,
			Version:   nextRevision,
			SpecJSON:  revision.SpecJSON,
			Status:    "published",
			Message:   "stale heal from published base " + strconv.Itoa(revision.BaseVersion),
			CreatedBy: staleHealActor,
			CreatedAt: now,
		}); err != nil {
			return err
		}
		// 只推进发布指针：人工未发布草稿的 spec/revision 原样保留。
		if err := pageModel.UpdatePublishedPointer(txCtx, gameID, env, revision.Page.PageKey, nextRevision, now); err != nil {
			return err
		}
		response.PageKey = revision.Page.PageKey
		response.Published = true
		response.PublishedVersion = nextRevision
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.auditPageEvent(ctx, audit.EventPagePublish, gameID, env, revision.Page.PageKey, map[string]interface{}{
		"action":                 "stale_heal",
		"base_published_version": revision.BaseVersion,
		"published_version":      response.PublishedVersion,
		"safe_bindings":          countChangedBindings(revision.Reports),
	})
	return response, nil
}

// staleHealProposalKey 生成 deterministic key：同一已发布基线 + 同一份
// selector 同步结果只对应一个提案，循环重复扫到同一漂移时不会堆出新提案。
// 漂移内容再变（契约改动产生不同 synced 规格）才产生新 key。
func staleHealProposalKey(pageKey string, baseVersion int, specJSON string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("stale-heal:%s:%d:%s", pageKey, baseVersion, specJSON)))
	return "stale-heal:" + hex.EncodeToString(sum[:])
}

// upsertStaleHealProposal 是默认路径的唯一产出：一个 pending 变更提案。
// 它不读写人工草稿（page_specs），也不碰 page_versions；已存在的同 key
// 提案（含人工已接受/拒绝的）一律不重建，避免自动修复把人的决定翻回
// pending。返回值表示本轮是否新建。
func (s *Service) upsertStaleHealProposal(ctx context.Context, gameID, env string, revision *staleHealRevision) (bool, error) {
	if revision == nil {
		return false, errorx.NewBadRequest("stale heal revision is required")
	}
	specJSON, err := pageJSONMarshal(revision.Page)
	if err != nil {
		return false, err
	}
	proposalKey := staleHealProposalKey(revision.Page.PageKey, revision.BaseVersion, string(specJSON))
	proposalModel := model.NewPageProposalModel(s.svcCtx.DB)
	if existing, findErr := proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey); findErr == nil && existing != nil {
		return false, nil
	} else if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return false, findErr
	}
	reportsJSON, err := pageJSONMarshal(revision.Reports)
	if err != nil {
		return false, err
	}
	proposal := &model.PageProposal{
		GameID:           gameID,
		Env:              env,
		ProposalKey:      proposalKey,
		PageKey:          revision.Page.PageKey,
		PageType:         string(revision.Page.Type),
		ResourceKey:      strings.TrimSpace(revision.Page.ResourceKey),
		Quality:          string(spec.GeneratedPageQualityReady),
		GeneratorVersion: staleHealGeneratorVersion,
		CategoryKey:      revision.Page.Category.Key,
		PageSpec:         model.JSON(specJSON),
		Diagnostics:      model.JSON(reportsJSON),
		Status:           dbenum.ProposalStatusPending,
		UpdatedBy:        staleHealActor,
	}
	title, err := pageJSONMarshal(normalizeLocaleKeys(revision.Page.Title))
	if err != nil {
		return false, err
	}
	var titleMap datatypes.JSONMap
	if err := json.Unmarshal(title, &titleMap); err != nil {
		return false, err
	}
	proposal.Title = titleMap
	if countErrors(revision.Remaining) > 0 {
		proposal.Quality = string(spec.GeneratedPageQualityNeedsReview)
	}
	if err := proposalModel.UpsertProposal(ctx, proposal); err != nil {
		return false, err
	}
	if err := model.NewPageProposalVersionModel(s.svcCtx.DB).CreateVersion(ctx, &model.PageProposalVersion{
		ProposalID:      proposal.ID,
		Version:         1,
		Proposal:        model.JSON(json.RawMessage(specJSON)),
		FunctionDigest:  revision.PublishSource.FunctionDigest,
		SemanticsDigest: revision.PublishSource.SemanticsDigest,
		ChangeReason:    "stale heal from published base " + strconv.Itoa(revision.BaseVersion),
		CreatedBy:       staleHealActor,
	}); err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		return false, err
	}
	return true, nil
}
