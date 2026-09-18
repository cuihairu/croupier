package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/cuihairu/croupier/internal/model"
)

// DefaultMenusFilename 是默认菜单种子文件名，位于 bootstrap 数据目录
// （bootstrapData.baseDir，与 admins.json/roles.json/default-policies.yaml
// 同目录）。文件存在且非空 → 种子启用；缺失/空数组 → 禁用（零配置开关，
// 部署方删文件即关闭默认菜单）。
const DefaultMenusFilename = "default-menus.json"

// seedMenuKeyPattern 与 api/menu.Create 的菜单标识校验同款；svc 层不能
// 反向 import api/menu，此处复制定义，变更需两处同步。
var seedMenuKeyPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

// SeedMenuItem 是 default-menus.json 的单条种子（仅支持顶级菜单——骨架
// 分组，嵌套结构由用户在菜单管理中按需扩展）。
type SeedMenuItem struct {
	MenuKey   string            `json:"menuKey"`
	Labels    map[string]string `json:"labels"`
	Icon      string            `json:"icon,omitempty"`
	SortOrder int               `json:"sortOrder,omitempty"`
	// Permission / IsVisible 可选；IsVisible 缺省 true（与手工创建语义一致）。
	Permission string `json:"permission,omitempty"`
	IsVisible  *bool  `json:"isVisible,omitempty"`
}

// LoadSeedMenus 读取并校验种子文件。文件不存在返回 (nil, nil) 表示禁用；
// 任一条目 menuKey 非法或 labels 全空 → 返回错误（整体禁用，不留半套骨架）。
func LoadSeedMenus(dir string) ([]SeedMenuItem, error) {
	path := filepath.Join(dir, DefaultMenusFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var seeds []SeedMenuItem
	if err := json.Unmarshal(data, &seeds); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(seeds) == 0 {
		return nil, nil
	}
	for i, seed := range seeds {
		if !seedMenuKeyPattern.MatchString(seed.MenuKey) {
			return nil, fmt.Errorf("%s[%d]: invalid menuKey %q", path, i, seed.MenuKey)
		}
		if !hasAnyLabel(seed.Labels) {
			return nil, fmt.Errorf("%s[%d]: menuKey %q needs at least one non-empty label", path, i, seed.MenuKey)
		}
	}
	return seeds, nil
}

func hasAnyLabel(labels map[string]string) bool {
	for _, v := range labels {
		if v != "" {
			return true
		}
	}
	return false
}

// MenuSeeder 在某 (gameId, env) scope 首次访问菜单读路径且该 scope 无任何
// 菜单时，导入 default-menus.json 骨架（惰性种子）。
//
// 语义边界（有意设计，勿改）：
//   - 只在 scope 菜单表为空时导入，永不覆盖/更新用户数据；
//   - 进程内每 scope 只尝试一次（成功、失败、冲突均标记），重启后重新评估；
//   - 用户删光全部菜单后重启会重新种一次——彻底禁用请删种子文件；
//   - 种子是系统行为（与 AdminManager 默认 admins 同语义），不写用户审计。
type MenuSeeder struct {
	model *model.MenuItemModel
	seeds []SeedMenuItem

	mu    sync.Mutex
	tried map[string]struct{}
}

// NewMenuSeeder 构造 seeder；seeds 为 nil/空时 EnsureSeeded 恒为 no-op。
func NewMenuSeeder(menuModel *model.MenuItemModel, seeds []SeedMenuItem) *MenuSeeder {
	return &MenuSeeder{
		model: menuModel,
		seeds: seeds,
		tried: make(map[string]struct{}),
	}
}

// Enabled 报告种子是否启用（文件存在且非空）。
func (s *MenuSeeder) Enabled() bool {
	return s != nil && len(s.seeds) > 0
}

// EnsureSeeded 幂等触发惰性种子。永不返回错误：读路径（菜单列表/控制台
// 导航）不因种子失败降级，失败仅 Warn 一次（tried 标记防反复重试刷屏）。
func (s *MenuSeeder) EnsureSeeded(ctx context.Context, gameID, env string) {
	if !s.Enabled() || gameID == "" || env == "" {
		return
	}
	scope := gameID + "\x00" + env

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, done := s.tried[scope]; done {
		return
	}
	// 无论后续成败都在退出前标记（失败不重试：文件错误重启才重评估）。
	defer func() { s.tried[scope] = struct{}{} }()

	count, err := s.model.CountByScope(ctx, gameID, env)
	if err != nil {
		slog.Default().Warn("menu seed: count scope failed", "gameId", gameID, "env", env, "error", err)
		return
	}
	if count > 0 {
		// scope 已有菜单（用户数据或上次种子）：永不导入。
		return
	}

	created := 0
	for _, seed := range s.seeds {
		item := &model.MenuItem{
			GameID:     gameID,
			Env:        env,
			MenuKey:    seed.MenuKey,
			Icon:       seed.Icon,
			SortOrder:  seed.SortOrder,
			Permission: seed.Permission,
			IsVisible:  seed.IsVisibleOrDefault(),
		}
		if err := item.SetLabels(seed.Labels); err != nil {
			slog.Default().Warn("menu seed: marshal labels failed", "menuKey", seed.MenuKey, "error", err)
			continue
		}
		if err := s.model.Create(ctx, item); err != nil {
			// 并发首访撞唯一索引 / 残留半套种子：跳过该条继续，唯一索引
			// 保证不会重复。
			slog.Default().Warn("menu seed: create skipped", "gameId", gameID, "env", env, "menuKey", seed.MenuKey, "error", err)
			continue
		}
		created++
	}
	if created > 0 {
		slog.Default().Info("menu seed: default menus imported", "gameId", gameID, "env", env, "created", created)
	}
}

// IsVisibleOrDefault 返回该条种子的可见性（缺省 true，与手工创建语义一致）。
func (seed SeedMenuItem) IsVisibleOrDefault() bool {
	return seed.IsVisible == nil || *seed.IsVisible
}
