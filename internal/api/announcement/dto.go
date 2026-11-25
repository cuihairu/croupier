package announcement

import "time"

// ---- Admin DTOs ----

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
	// GameIds lists the bound game identifiers (#45); empty = unbound =
	// visible server-wide
	GameIds   []string  `json:"gameIds,omitempty"`
	CreatedBy string    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
	// GameIds are the bound game identifiers (#45); empty/omitted = visible
	// server-wide
	GameIds []string `json:"gameIds"`
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
	// GameIds replaces all bindings when non-nil (an empty slice clears the
	// bindings -> visible server-wide); nil (omitted) leaves bindings
	// untouched
	GameIds *[]string `json:"gameIds"`
}

// ---- User-side DTOs ----

// ActiveItem is an announcement visible to the current user; when
// shouldPopup=true the frontend shows it as a popup until the user
// acknowledges it (dismiss).
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
