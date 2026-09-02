package announcement

import "time"

// ---- 管理 DTO ----

type AdminAnnouncementItem struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title"`
	ContentMd string     `json:"contentMd"`
	Audience  string     `json:"audience"`
	Role      string     `json:"role,omitempty"`
	Popup     bool       `json:"popup"`
	Active    bool       `json:"active"`
	StartAt   *time.Time `json:"startAt,omitempty"`
	EndAt     *time.Time `json:"endAt,omitempty"`
	CreatedBy string     `json:"createdBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type AdminListResponse struct {
	Items []AdminAnnouncementItem `json:"items"`
	Total int64                   `json:"total"`
}

type CreateRequest struct {
	Title     string     `json:"title" binding:"required"`
	ContentMd string     `json:"contentMd" binding:"required"`
	Audience  string     `json:"audience"` // all | role
	Role      string     `json:"role"`
	Popup     bool       `json:"popup"`
	Active    *bool      `json:"active"`
	StartAt   *time.Time `json:"startAt"`
	EndAt     *time.Time `json:"endAt"`
}

type UpdateRequest struct {
	Title     *string    `json:"title"`
	ContentMd *string    `json:"contentMd"`
	Audience  *string    `json:"audience"`
	Role      *string    `json:"role"`
	Popup     *bool      `json:"popup"`
	Active    *bool      `json:"active"`
	StartAt   *time.Time `json:"startAt"`
	EndAt     *time.Time `json:"endAt"`
}

// ---- 用户侧 DTO ----

// ActiveItem 是当前用户可见的公告；shouldPopup=true 时前端弹窗展示，
// 直至用户确认（dismiss）。
type ActiveItem struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	ContentMd   string    `json:"contentMd"`
	Audience    string    `json:"audience"`
	Popup       bool      `json:"popup"`
	ShouldPopup bool      `json:"shouldPopup"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ActiveListResponse struct {
	Items []ActiveItem `json:"items"`
}

type DismissResponse struct {
	Dismissed bool `json:"dismissed"`
}
