// Package announcement 提供后台公告：面向全体/角色的 Markdown 公告，
// popup 公告在用户登录后弹窗展示直至显式确认（announcement_reads）。
package announcement

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) db() *gorm.DB { return s.svcCtx.DB }

// ---- 管理 ----

// List 返回公告列表（按更新时间倒序），每项带绑定的游戏标识。
// gameID 非空时只返回「对该游戏适用」的公告：未绑定任何游戏（全服可见）
// 或绑定了该游戏（#45）。
func (s *Service) List(ctx context.Context, gameID string) (*AdminListResponse, error) {
	var rows []model.Announcement
	if err := s.db().WithContext(ctx).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}
	bindings, err := s.gameIDsByAnnouncement(ctx, ids)
	if err != nil {
		return nil, err
	}
	gameID = strings.TrimSpace(gameID)
	items := make([]AdminAnnouncementItem, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		gameIDs := bindings[r.ID]
		if gameID != "" && !applicableToGame(gameIDs, gameID) {
			continue
		}
		item := toAdminItem(r)
		item.GameIds = gameIDs
		items = append(items, item)
	}
	return &AdminListResponse{Items: items, Total: int64(len(items))}, nil
}

func (s *Service) Create(ctx context.Context, req *CreateRequest) (*AdminAnnouncementItem, error) {
	if err := validateAudience(req.Audience, req.Role); err != nil {
		return nil, err
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	gameIDs := normalizeGameIDs(req.GameIds)
	row := &model.Announcement{
		Title:     strings.TrimSpace(req.Title),
		ContentMd: req.ContentMd,
		Audience:  normalizeAudience(req.Audience),
		Role:      strings.TrimSpace(req.Role),
		Popup:     req.Popup,
		Active:    active,
		StartAt:   req.StartAt,
		EndAt:     req.EndAt,
		CreatedBy: currentUser(ctx),
	}
	if err := s.db().WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	if err := s.replaceGameBindings(ctx, row.ID, gameIDs); err != nil {
		return nil, err
	}
	item := toAdminItem(row)
	item.GameIds = gameIDs
	return &item, nil
}

func (s *Service) Update(ctx context.Context, id uint, req *UpdateRequest) (*AdminAnnouncementItem, error) {
	var row model.Announcement
	if err := s.db().WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("公告不存在")
		}
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = strings.TrimSpace(*req.Title)
	}
	if req.ContentMd != nil {
		updates["content_md"] = *req.ContentMd
	}
	if req.Audience != nil {
		role := derefOr(req.Role, row.Role)
		if err := validateAudience(*req.Audience, role); err != nil {
			return nil, err
		}
		updates["audience"] = normalizeAudience(*req.Audience)
	}
	if req.Role != nil {
		aud := derefOr(req.Audience, row.Audience)
		if err := validateAudience(aud, *req.Role); err != nil {
			return nil, err
		}
		updates["role"] = strings.TrimSpace(*req.Role)
	}
	if req.Popup != nil {
		updates["popup"] = *req.Popup
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if req.StartAt != nil {
		updates["start_at"] = req.StartAt
	}
	if req.EndAt != nil {
		updates["end_at"] = req.EndAt
	}
	if len(updates) > 0 {
		if err := s.db().WithContext(ctx).Model(&model.Announcement{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	// #45：gameIds 非 nil（含空数组）→ 全量替换绑定；缺省 → 绑定不动
	if req.GameIds != nil {
		if err := s.replaceGameBindings(ctx, id, normalizeGameIDs(*req.GameIds)); err != nil {
			return nil, err
		}
	}
	if err := s.db().WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	bindings, err := s.gameIDsByAnnouncement(ctx, []uint{row.ID})
	if err != nil {
		return nil, err
	}
	item := toAdminItem(&row)
	item.GameIds = bindings[row.ID]
	return &item, nil
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	res := s.db().WithContext(ctx).Delete(&model.Announcement{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errorx.NewNotFound("公告不存在")
	}
	// 级联清理确认记录与游戏绑定
	if err := s.db().WithContext(ctx).Where("announcement_id = ?", id).Delete(&model.AnnouncementRead{}).Error; err != nil {
		return err
	}
	return s.db().WithContext(ctx).Where("announcement_id = ?", id).Delete(&model.AnnouncementGame{}).Error
}

// ---- 用户侧 ----

// ActiveForUser 返回当前用户可见的生效公告（按创建时间倒序）。
// shouldPopup = popup && 未确认 && 未过期；「未确认前每次登录都弹」
// 由前端在每次会话初始化时调用本接口实现。
// gameID 为当前顶栏选择的游戏（X-Game-ID）：未绑定公告始终可见；
// 绑定公告仅当绑定含该游戏时可见，无游戏上下文（空）时只看未绑定（#45）。
func (s *Service) ActiveForUser(ctx context.Context, username string, roles []string, gameID string) (*ActiveListResponse, error) {
	var rows []model.Announcement
	if err := s.db().WithContext(ctx).
		Where("active = ?", true).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	roleSet := map[string]bool{}
	for _, r := range roles {
		roleSet[r] = true
	}
	readSet, err := s.readSetOf(ctx, username)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}
	bindings, err := s.gameIDsByAnnouncement(ctx, ids)
	if err != nil {
		return nil, err
	}
	gameID = strings.TrimSpace(gameID)
	items := []ActiveItem{}
	for i := range rows {
		r := &rows[i]
		if !visibleTo(r, roleSet) {
			continue
		}
		if r.StartAt != nil && now.Before(*r.StartAt) {
			continue
		}
		if r.EndAt != nil && now.After(*r.EndAt) {
			continue
		}
		if !applicableToGame(bindings[r.ID], gameID) {
			continue
		}
		items = append(items, ActiveItem{
			ID:          int64(r.ID),
			Title:       r.Title,
			ContentMd:   r.ContentMd,
			Audience:    r.Audience,
			Popup:       r.Popup,
			ShouldPopup: r.Popup && !readSet[r.ID],
			CreatedAt:   r.CreatedAt,
		})
	}
	return &ActiveListResponse{Items: items}, nil
}

// Dismiss 记录用户确认（幂等）。
func (s *Service) Dismiss(ctx context.Context, username string, id uint) (*DismissResponse, error) {
	var row model.Announcement
	if err := s.db().WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("公告不存在")
		}
		return nil, err
	}
	read := model.AnnouncementRead{AnnouncementID: id, Username: username, ReadAt: time.Now()}
	if err := s.db().WithContext(ctx).
		Where("announcement_id = ? AND username = ?", id, username).
		FirstOrCreate(&read).Error; err != nil {
		return nil, err
	}
	return &DismissResponse{Dismissed: true}, nil
}

func (s *Service) readSetOf(ctx context.Context, username string) (map[uint]bool, error) {
	var reads []model.AnnouncementRead
	if err := s.db().WithContext(ctx).
		Where("username = ?", username).Find(&reads).Error; err != nil {
		return nil, err
	}
	out := map[uint]bool{}
	for _, r := range reads {
		out[r.AnnouncementID] = true
	}
	return out, nil
}

func visibleTo(a *model.Announcement, roleSet map[string]bool) bool {
	if a.Audience == "role" {
		return roleSet[strings.TrimSpace(a.Role)]
	}
	return true // all
}

// ---- 游戏绑定（#45）----

// applicableToGame 判断公告对某游戏是否适用：未绑定任何游戏 = 全服可见；
// 有绑定时仅当绑定含该游戏。gameID 为空（无游戏上下文）时只看未绑定。
func applicableToGame(gameIDs []string, gameID string) bool {
	if len(gameIDs) == 0 {
		return true
	}
	if gameID == "" {
		return false
	}
	for _, g := range gameIDs {
		if g == gameID {
			return true
		}
	}
	return false
}

// normalizeGameIDs 归一绑定入参：trim、丢空串、去重（保序）。
func normalizeGameIDs(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, g := range in {
		g = strings.TrimSpace(g)
		if g == "" || seen[g] {
			continue
		}
		seen[g] = true
		out = append(out, g)
	}
	return out
}

// replaceGameBindings 全量替换公告的游戏绑定（先删后插，幂等）。
func (s *Service) replaceGameBindings(ctx context.Context, announcementID uint, gameIDs []string) error {
	if err := s.db().WithContext(ctx).
		Where("announcement_id = ?", announcementID).
		Delete(&model.AnnouncementGame{}).Error; err != nil {
		return err
	}
	if len(gameIDs) == 0 {
		return nil
	}
	rows := make([]model.AnnouncementGame, 0, len(gameIDs))
	for _, g := range gameIDs {
		rows = append(rows, model.AnnouncementGame{AnnouncementID: announcementID, GameID: g})
	}
	return s.db().WithContext(ctx).Create(&rows).Error
}

// gameIDsByAnnouncement 批量加载公告→绑定游戏标识映射（一次查询防 N+1）。
func (s *Service) gameIDsByAnnouncement(ctx context.Context, ids []uint) (map[uint][]string, error) {
	out := map[uint][]string{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []model.AnnouncementGame
	if err := s.db().WithContext(ctx).
		Where("announcement_id IN ?", ids).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.AnnouncementID] = append(out[r.AnnouncementID], r.GameID)
	}
	return out, nil
}

// ---- helpers ----

func validateAudience(audience, role string) error {
	switch normalizeAudience(audience) {
	case "all":
		return nil
	case "role":
		if strings.TrimSpace(role) == "" {
			return errorx.NewBadRequest("audience=role 时必须指定 role")
		}
		return nil
	default:
		return errorx.NewBadRequest("audience 必须是 all 或 role")
	}
}

func normalizeAudience(a string) string {
	a = strings.ToLower(strings.TrimSpace(a))
	if a == "" {
		return "all"
	}
	return a
}

func currentUser(ctx context.Context) string {
	if name, err := utils.CurrentUsername(ctx); err == nil && name != "" {
		return name
	}
	return "system"
}

func derefOr(v *string, def string) string {
	if v != nil {
		return *v
	}
	return def
}

func toAdminItem(r *model.Announcement) AdminAnnouncementItem {
	return AdminAnnouncementItem{
		ID:        int64(r.ID),
		Title:     r.Title,
		ContentMd: r.ContentMd,
		Audience:  r.Audience,
		Role:      r.Role,
		Popup:     r.Popup,
		Active:    r.Active,
		StartAt:   r.StartAt,
		EndAt:     r.EndAt,
		CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
