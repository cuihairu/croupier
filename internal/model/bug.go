package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/dbenum"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Bug is the product defect tracking model, independent from support
// tickets (see docs/research/bug-tracking-design.md). A ticket is a service
// request closed by customer satisfaction; a bug is a product fix tracked to
// a released version.
type Bug struct {
	gorm.Model
	Title    string `gorm:"size:255;index"`
	Content  string `gorm:"type:text"`
	Status   string `gorm:"size:32;index:idx_bug_status"`
	Severity string `gorm:"size:16;index"`
	Priority string `gorm:"size:16;index"`
	Assignee string `gorm:"size:64;index"`

	// Game context
	GameID   string `gorm:"size:64;index:idx_bug_game_env,priority:1"`
	Env      string `gorm:"size:64;index:idx_bug_game_env,priority:2"`
	ServerID string `gorm:"size:64"`
	Platform string `gorm:"size:32"`  // ios|android|pc|webgl|editor
	Device   string `gorm:"size:128"` // device model, e.g. iPhone 15 Pro
	OS       string `gorm:"size:64"`  // e.g. iOS 18.1

	// Reproduction
	Steps           string `gorm:"type:text"`
	Reproducibility string `gorm:"size:16"` // always|often|sometimes|once

	// Version tracking (Jira-style dual version)
	AffectsVersion string `gorm:"size:64"`
	FixVersion     string `gorm:"size:64;index"`

	// Origin: player-reported via GM, internal QA, or converted from a ticket.
	Source         string `gorm:"size:32;index"` // player|internal|ticket
	SourceTicketID uint   `gorm:"index"`
	PlayerID       string `gorm:"size:64;index"`

	// External links (GitHub issue/PR, wiki, monitor dashboards...)
	Links JSON `gorm:"type:json"`

	// CrashFingerprint is the aggregation key for auto-filed crash bugs
	// (bug-tracking P2): same normalized stack → same bug with a counter.
	// Indexed for the report hot path; empty for manually filed bugs.
	CrashFingerprint string `gorm:"size:32;index"`

	// Free-form payload (crash report ids, dashboard params...)
	Extra datatypes.JSONMap `gorm:"type:json"`

	CreatedBy string     `gorm:"size:64"`
	DueAt     *time.Time `gorm:"index"`

	// Incident-report category (incident-reports 批 1): 职能域类别 + 子类。
	// 0040 加列（HasColumn+AddColumn）；存量行 category_id=0 → 报表层归入未分类。
	CategoryID  uint   `gorm:"index"`
	Subcategory string `gorm:"size:64"`
}

func (Bug) TableName() string { return "bugs" }

// BugLink is one external reference attached to a bug.
type BugLink struct {
	URL   string `json:"url"`
	Kind  string `json:"kind"` // github_issue|github_pr|gitlab|jira|wiki|monitor|other
	Title string `json:"title,omitempty"`
}

// Controlled vocabulary for link kinds (frontend icon mapping depends on it).
const (
	BugLinkGithubIssue = "github_issue"
	BugLinkGithubPR    = "github_pr"
	BugLinkGitlab      = "gitlab"
	BugLinkJira        = "jira"
	BugLinkWiki        = "wiki"
	BugLinkMonitor     = "monitor"
	BugLinkOther       = "other"
)

// ValidBugLinkKinds is the closed set of link kinds.
var ValidBugLinkKinds = map[string]struct{}{
	BugLinkGithubIssue: {}, BugLinkGithubPR: {}, BugLinkGitlab: {},
	BugLinkJira: {}, BugLinkWiki: {}, BugLinkMonitor: {}, BugLinkOther: {},
}

// Bug status flow: triage → confirmed → fixing → verify → released,
// with terminal alternatives wontfix/rejected.
const (
	BugStatusTriage    = "triage"
	BugStatusConfirmed = "confirmed"
	BugStatusFixing    = "fixing"
	BugStatusVerify    = "verify"
	BugStatusReleased  = "released"
	BugStatusWontfix   = "wontfix"
	BugStatusRejected  = "rejected"
)

// BugStatusFlow lists statuses in board order.
var BugStatusFlow = []string{
	BugStatusTriage, BugStatusConfirmed, BugStatusFixing, BugStatusVerify, BugStatusReleased,
}

// ValidBugStatuses is the closed set of statuses (flow + terminals).
var ValidBugStatuses = map[string]struct{}{
	BugStatusTriage: {}, BugStatusConfirmed: {}, BugStatusFixing: {},
	BugStatusVerify: {}, BugStatusReleased: {}, BugStatusWontfix: {}, BugStatusRejected: {},
}

// Severity grades the player impact; Priority orders the schedule. They are
// orthogonal (a minor bug can be urgent for a launch blocker demo).
const (
	BugSeverityBlocker  = "blocker"
	BugSeverityCritical = "critical"
	BugSeverityMajor    = "major"
	BugSeverityMinor    = "minor"
)

// ValidBugSeverities is the closed set of severities.
var ValidBugSeverities = map[string]struct{}{
	BugSeverityBlocker: {}, BugSeverityCritical: {}, BugSeverityMajor: {}, BugSeverityMinor: {},
}

// ValidBugReproducibility is the closed set of reproduction rates.
var ValidBugReproducibility = map[string]struct{}{
	"always": {}, "often": {}, "sometimes": {}, "once": {},
}

// BugModel provides CRUD for the defect tracker.
type BugModel struct {
	db *gorm.DB
}

// NewBugModel creates a helper.
func NewBugModel(db *gorm.DB) *BugModel {
	return &BugModel{db: db}
}

// BugQueryOptions controls listing.
type BugQueryOptions struct {
	PaginationOptions
	Query    string
	Status   string
	Severity string
	Priority string
	Assignee string
	GameID   string
	Env      string
	Platform string
	// FixVersion filters the release board (which bugs ship in version X).
	FixVersion string
	PlayerID   string
}

// List returns bugs matching the options.
func (m *BugModel) List(ctx context.Context, opts BugQueryOptions) ([]Bug, int64, error) {
	opts.Normalize()

	var (
		items []Bug
		total int64
	)
	query := m.db.WithContext(ctx).Model(&Bug{})
	if opts.Query != "" {
		like := "%" + opts.Query + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", like, like)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.Severity != "" {
		query = query.Where("severity = ?", opts.Severity)
	}
	if opts.Priority != "" {
		query = query.Where("priority = ?", opts.Priority)
	}
	if opts.Assignee != "" {
		query = query.Where("assignee = ?", opts.Assignee)
	}
	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		query = query.Where("env = ?", opts.Env)
	}
	if opts.Platform != "" {
		query = query.Where("platform = ?", opts.Platform)
	}
	if opts.FixVersion != "" {
		query = query.Where("fix_version = ?", opts.FixVersion)
	}
	if opts.PlayerID != "" {
		query = query.Where("player_id = ?", opts.PlayerID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.
		// CASE-based board ordering works across MySQL/Postgres/SQLite
		// (FIELD() is MySQL-only).
		Order("CASE status WHEN 'triage' THEN 1 WHEN 'confirmed' THEN 2 WHEN 'fixing' THEN 3 WHEN 'verify' THEN 4 WHEN 'released' THEN 5 ELSE 9 END, updated_at DESC").
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FindOne returns a bug by id.
func (m *BugModel) FindOne(ctx context.Context, id uint) (*Bug, error) {
	var bug Bug
	if err := m.db.WithContext(ctx).First(&bug, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &bug, nil
}

// Create inserts a bug.
func (m *BugModel) Create(ctx context.Context, bug *Bug) error {
	return m.db.WithContext(ctx).Create(bug).Error
}

// Update applies a partial update map.
func (m *BugModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&Bug{}).Where("id = ?", id).Updates(updates).Error
}

// FindOpenByCrashFingerprint returns the open (non-terminal) bug holding a
// crash fingerprint, if any. Terminal statuses no longer aggregate.
func (m *BugModel) FindOpenByCrashFingerprint(ctx context.Context, gameID, env, fingerprint string) (*Bug, error) {
	var bug Bug
	err := m.db.WithContext(ctx).
		Where("game_id = ? AND (env = ? OR ? = '') AND crash_fingerprint = ? AND status NOT IN ?",
			gameID, env, env, fingerprint,
			[]string{BugStatusReleased, BugStatusWontfix, BugStatusRejected}).
		Order("updated_at DESC").
		First(&bug).Error
	if err != nil {
		return nil, err
	}
	return &bug, nil
}

// Delete removes a bug.
func (m *BugModel) Delete(ctx context.Context, id uint) error {
	res := m.db.WithContext(ctx).Delete(&Bug{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ValidateBugLinks checks every link kind/url.
func ValidateBugLinks(links []BugLink) error {
	for _, l := range links {
		if strings.TrimSpace(l.URL) == "" {
			return errors.New("bug link url is required")
		}
		if _, ok := ValidBugLinkKinds[l.Kind]; !ok {
			return errors.New("invalid bug link kind: " + l.Kind)
		}
	}
	return nil
}

// BugTicketLink is the bug↔ticket many-to-many association (#25).
// Bug and ticket are different domains (product fix vs service request):
// the one-shot conversion keeps Bug.SourceTicketID as legacy semantics,
// while this table supports arbitrary cross-links managed from both sides.
type BugTicketLink struct {
	gorm.Model
	BugID     uint   `gorm:"uniqueIndex:ux_bug_ticket_links_bug_ticket,priority:1"`
	TicketID  uint   `gorm:"uniqueIndex:ux_bug_ticket_links_bug_ticket,priority:2"`
	CreatedBy string `gorm:"size:64"`
}

func (BugTicketLink) TableName() string { return "bug_ticket_links" }

// TicketBrief is the ticket summary rendered on the bug side of the
// association (links to /support/tickets/:id).
type TicketBrief struct {
	ID       uint   `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	GameID   string `json:"gameId"`
	Env      string `json:"env"`
}

// BugBrief is the bug summary rendered on the ticket side of the
// association (links to /dev/bugs).
type BugBrief struct {
	ID       uint   `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Severity string `json:"severity"`
	Priority string `json:"priority"`
}

// 关联校验失败哨兵：model 层保持 plain error，由 API service 层翻译为 4xx
// （model 包不依赖 errorx 的既有边界）。
var (
	ErrBugLinkInvalidID     = errors.New("bug id 和工单 id 均不能为空")
	ErrBugLinkBugMissing    = errors.New("bug 不存在")
	ErrBugLinkTicketMissing = errors.New("工单不存在")
)

// LinkBugTicket associates a bug with a support ticket (idempotent:
// re-linking an existing pair is a no-op). Both sides must exist.
func (m *BugModel) LinkBugTicket(ctx context.Context, bugID, ticketID uint, createdBy string) error {
	if bugID == 0 || ticketID == 0 {
		return ErrBugLinkInvalidID
	}
	var bug Bug
	if err := m.db.WithContext(ctx).First(&bug, bugID).Error; err != nil {
		return ErrBugLinkBugMissing
	}
	var ticket Ticket
	if err := m.db.WithContext(ctx).First(&ticket, ticketID).Error; err != nil {
		return ErrBugLinkTicketMissing
	}
	link := BugTicketLink{BugID: bugID, TicketID: ticketID, CreatedBy: createdBy}
	// 先查后建：唯一索引兜底并发重复，已存在直接成功（幂等语义）
	var existing BugTicketLink
	err := m.db.WithContext(ctx).
		Where("bug_id = ? AND ticket_id = ?", bugID, ticketID).
		First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return m.db.WithContext(ctx).Create(&link).Error
}

// UnlinkBugTicket removes the association; unlinking a non-existent pair
// is a no-op so client retries stay safe.
func (m *BugModel) UnlinkBugTicket(ctx context.Context, bugID, ticketID uint) error {
	return m.db.WithContext(ctx).
		Where("bug_id = ? AND ticket_id = ?", bugID, ticketID).
		Delete(&BugTicketLink{}).Error
}

// ListTicketsByBug returns the ticket summaries linked to a bug.
func (m *BugModel) ListTicketsByBug(ctx context.Context, bugID uint) ([]TicketBrief, error) {
	// status 在库里是整型枚举，经类型化字段扫描后再转可读字符串
	var rows []struct {
		ID       uint
		Title    string
		Status   dbenum.TicketStatus
		Priority string
		GameID   string
		Env      string
	}
	err := m.db.WithContext(ctx).
		Table("bug_ticket_links").
		Select("tickets.id, tickets.title, tickets.status, tickets.priority, tickets.game_id, tickets.env").
		Joins("JOIN tickets ON tickets.id = bug_ticket_links.ticket_id AND tickets.deleted_at IS NULL").
		Where("bug_ticket_links.bug_id = ? AND bug_ticket_links.deleted_at IS NULL", bugID).
		Order("bug_ticket_links.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]TicketBrief, 0, len(rows))
	for _, r := range rows {
		items = append(items, TicketBrief{
			ID:       r.ID,
			Title:    r.Title,
			Status:   r.Status.String(),
			Priority: r.Priority,
			GameID:   r.GameID,
			Env:      r.Env,
		})
	}
	return items, nil
}

// ListBugsByTicket returns the bug summaries linked to a ticket.
func (m *BugModel) ListBugsByTicket(ctx context.Context, ticketID uint) ([]BugBrief, error) {
	var rows []BugBrief
	err := m.db.WithContext(ctx).
		Table("bug_ticket_links").
		Select("bugs.id, bugs.title, bugs.status, bugs.severity, bugs.priority").
		Joins("JOIN bugs ON bugs.id = bug_ticket_links.bug_id AND bugs.deleted_at IS NULL").
		Where("bug_ticket_links.ticket_id = ? AND bug_ticket_links.deleted_at IS NULL", ticketID).
		Order("bug_ticket_links.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	// 空结果返回 [] 而非 null（前端表格与跳转列表依赖数组形态）
	items := make([]BugBrief, 0, len(rows))
	items = append(items, rows...)
	return items, nil
}
