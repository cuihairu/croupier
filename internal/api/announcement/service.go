// Package announcement provides admin announcements: Markdown announcements
// targeted at everyone or at a role; popup announcements are shown to the
// user after login until explicitly acknowledged (announcement_reads).
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

// ---- Admin ----

// List returns the announcement list (newest update first), each item
// carrying its bound game identifiers. When gameID is non-empty, only
// announcements that "apply to that game" are returned: announcements bound
// to no game (visible server-wide) or bound to that game (#45).
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
	// #45: gameIds non-nil (including an empty slice) -> replace all
	// bindings; omitted -> bindings untouched
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
	// Cascade cleanup of acknowledgement records and game bindings
	if err := s.db().WithContext(ctx).Where("announcement_id = ?", id).Delete(&model.AnnouncementRead{}).Error; err != nil {
		return err
	}
	return s.db().WithContext(ctx).Where("announcement_id = ?", id).Delete(&model.AnnouncementGame{}).Error
}

// ---- User side ----

// ActiveForUser returns the active announcements visible to the current user
// (newest first). shouldPopup = popup && not acknowledged && not expired;
// "pop up on every login until acknowledged" is implemented by the frontend
// calling this endpoint on each session initialization.
// gameID is the game currently selected in the top bar (X-Game-ID):
// unbound announcements are always visible; bound announcements are visible
// only when their bindings include that game, and with no game context
// (empty) only unbound announcements show (#45).
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

// Dismiss records the user's acknowledgement (idempotent).
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

// ---- Game bindings (#45) ----

// applicableToGame decides whether an announcement applies to a game:
// bound to no game = visible server-wide; when bindings exist it applies
// only if they include the game. With an empty gameID (no game context) only
// unbound announcements apply.
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

// normalizeGameIDs normalizes the binding input: trims, drops empty strings,
// deduplicates (order-preserving).
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

// replaceGameBindings fully replaces an announcement's game bindings
// (delete first, then insert; idempotent).
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

// gameIDsByAnnouncement loads the announcement -> bound game IDs mapping in
// bulk (one query to avoid N+1).
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
