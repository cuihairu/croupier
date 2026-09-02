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

func (s *Service) List(ctx context.Context) (*AdminListResponse, error) {
	var rows []model.Announcement
	if err := s.db().WithContext(ctx).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]AdminAnnouncementItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, toAdminItem(&r))
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
	item := toAdminItem(row)
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
	if err := s.db().WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	item := toAdminItem(&row)
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
	// 级联清理确认记录
	return s.db().WithContext(ctx).Where("announcement_id = ?", id).Delete(&model.AnnouncementRead{}).Error
}

// ---- 用户侧 ----

// ActiveForUser 返回当前用户可见的生效公告（按创建时间倒序）。
// shouldPopup = popup && 未确认 && 未过期；「未确认前每次登录都弹」
// 由前端在每次会话初始化时调用本接口实现。
func (s *Service) ActiveForUser(ctx context.Context, username string, roles []string) (*ActiveListResponse, error) {
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
