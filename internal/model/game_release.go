package model

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// GameRelease tracks a game client artifact through its release lifecycle
// (see docs/research/release-management-design.md).
type GameRelease struct {
	gorm.Model
	GameID   string `gorm:"size:64;index:idx_release_scope,priority:1"`
	Env      string `gorm:"size:64;index:idx_release_scope,priority:2"`
	Channel  string `gorm:"size:64;index:idx_release_scope,priority:3"`
	Platform string `gorm:"size:32;index:idx_release_scope,priority:4"`
	Version  string `gorm:"size:64"`
	Status   string `gorm:"size:32;index"`

	// hotfix（热更资源包）| full（整包）| forced（强更配置）
	Type string `gorm:"size:16"`

	// Artifact stored in objstore; set by the artifact upload endpoint.
	ObjectKey string `gorm:"size:512"`
	Size      int64
	Checksum  string `gorm:"size:64"`   // sha256 hex
	Manifest  JSON   `gorm:"type:json"` // file -> hash map for delta downloads

	Notes datatypes.JSONMap `gorm:"type:json"` // LocalizedText

	// Gray rollout.
	GrayPercent int    `gorm:"default:0"` // 0-100
	Whitelist   JSON   `gorm:"type:json"` // device/server/player ids
	GraySeed    string `gorm:"size:32"`   // bucket seed (stable per release)

	CreatedBy string     `gorm:"size:64"`
	DueAt     *time.Time `gorm:"index"`
}

func (GameRelease) TableName() string { return "game_releases" }

// Release statuses (state machine; see design doc §3.2).
const (
	ReleaseStatusDraft      = "draft"
	ReleaseStatusUploading  = "uploading"
	ReleaseStatusTesting    = "testing"
	ReleaseStatusGray       = "gray"
	ReleaseStatusFull       = "full"
	ReleaseStatusArchived   = "archived"
	ReleaseStatusRolledBack = "rolled_back"
)

// Release types.
const (
	ReleaseTypeHotfix = "hotfix"
	ReleaseTypeFull   = "full"
	ReleaseTypeForced = "forced"
)

// ValidReleaseTransitions lists the legal forward moves. draft→uploading is
// implicit (the artifact upload endpoint drives it). gray→gray is legal and
// only updates the rollout percent (must be non-decreasing).
var ValidReleaseTransitions = map[string][]string{
	ReleaseStatusDraft:     {ReleaseStatusUploading, ReleaseStatusArchived},
	ReleaseStatusUploading: {ReleaseStatusTesting, ReleaseStatusArchived},
	ReleaseStatusTesting:   {ReleaseStatusGray, ReleaseStatusArchived},
	ReleaseStatusGray:      {ReleaseStatusGray, ReleaseStatusFull, ReleaseStatusRolledBack, ReleaseStatusArchived},
	ReleaseStatusFull:      {ReleaseStatusRolledBack},
}

// ValidReleasePlatforms is the closed set of platforms.
var ValidReleasePlatforms = map[string]struct{}{
	"ios": {}, "android": {}, "pc": {}, "webgl": {},
}

// ValidReleaseTypes is the closed set of types.
var ValidReleaseTypes = map[string]struct{}{
	ReleaseTypeHotfix: {}, ReleaseTypeFull: {}, ReleaseTypeForced: {},
}

// CanTransition reports whether from → to is legal.
func CanTransition(from, to string) bool {
	for _, next := range ValidReleaseTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// BucketHit implements the stable per-device gray bucketing: the same device
// maps to the same bucket for a given release seed, so raising the percent
// only adds devices (never flips existing ones out).
func (r *GameRelease) BucketHit(deviceID string) bool {
	if r.GrayPercent >= 100 {
		return true
	}
	if r.GrayPercent <= 0 {
		return false
	}
	sum := sha256.Sum256([]byte(deviceID + "|" + r.GraySeed))
	raw := sum[:4]
	value := (uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])) % 10000
	return int(value) < r.GrayPercent*100
}

// GameReleaseModel provides CRUD for releases.
type GameReleaseModel struct {
	db *gorm.DB
}

// NewGameReleaseModel creates a helper.
func NewGameReleaseModel(db *gorm.DB) *GameReleaseModel {
	return &GameReleaseModel{db: db}
}

// ReleaseQueryOptions controls listing.
type ReleaseQueryOptions struct {
	PaginationOptions
	GameID   string
	Env      string
	Channel  string
	Platform string
	Status   string
	Type     string
}

// List returns releases matching the filters, newest version activity first.
func (m *GameReleaseModel) List(ctx context.Context, opts ReleaseQueryOptions) ([]GameRelease, int64, error) {
	opts.PaginationOptions.Normalize()
	var (
		items []GameRelease
		total int64
	)
	query := m.db.WithContext(ctx).Model(&GameRelease{})
	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		query = query.Where("env = ?", opts.Env)
	}
	if opts.Channel != "" {
		query = query.Where("channel = ?", opts.Channel)
	}
	if opts.Platform != "" {
		query = query.Where("platform = ?", opts.Platform)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.Type != "" {
		query = query.Where("type = ?", opts.Type)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.
		Order("updated_at DESC").
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FindOne returns a release by id.
func (m *GameReleaseModel) FindOne(ctx context.Context, id uint) (*GameRelease, error) {
	var rel GameRelease
	if err := m.db.WithContext(ctx).First(&rel, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &rel, nil
}

// Create inserts a release.
func (m *GameReleaseModel) Create(ctx context.Context, rel *GameRelease) error {
	return m.db.WithContext(ctx).Create(rel).Error
}

// Update applies a partial update map.
func (m *GameReleaseModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&GameRelease{}).Where("id = ?", id).Updates(updates).Error
}

// Transition moves a release between states inside a transaction, enforcing
// the single-full invariant: promoting to full demotes any existing full in
// the same scope to archived.
func (m *GameReleaseModel) Transition(ctx context.Context, id uint, to string, grayPercent *int) (*GameRelease, error) {
	var result *GameRelease
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rel GameRelease
		if err := tx.First(&rel, id).Error; err != nil {
			return err
		}
		if !CanTransition(rel.Status, to) {
			return fmt.Errorf("invalid transition %s -> %s", rel.Status, to)
		}
		if to == ReleaseStatusTesting && rel.ObjectKey == "" {
			return errors.New("artifact must be uploaded before testing")
		}
		updates := map[string]interface{}{"status": to}
		if to == ReleaseStatusGray {
			pct := 0
			if grayPercent != nil {
				pct = *grayPercent
			}
			if pct < 0 || pct > 100 {
				return errors.New("gray percent must be within 0-100")
			}
			if pct < rel.GrayPercent {
				return errors.New("gray percent can only increase; use rollback to reduce exposure")
			}
			updates["gray_percent"] = pct
		}
		if to == ReleaseStatusFull {
			updates["gray_percent"] = 100
			// Demote the current full release in the same scope.
			if err := tx.Model(&GameRelease{}).
				Where("game_id = ? AND env = ? AND channel = ? AND platform = ? AND status = ? AND id <> ?",
					rel.GameID, rel.Env, rel.Channel, rel.Platform, ReleaseStatusFull, rel.ID).
				Update("status", ReleaseStatusArchived).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&GameRelease{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		result = &rel
		result.Status = to
		if pct, ok := updates["gray_percent"].(int); ok {
			result.GrayPercent = pct
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CheckUpdateQuery carries the client-side check parameters.
type CheckUpdateQuery struct {
	GameID   string
	Env      string
	Channel  string
	Platform string
	DeviceID string
}

// FindCandidates returns all active releases (testing/gray/full) for the
// scope ordered by version activity, newest first. The caller applies
// per-device gray/whitelist filtering.
func (m *GameReleaseModel) FindCandidates(ctx context.Context, q CheckUpdateQuery) ([]GameRelease, error) {
	var items []GameRelease
	err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND channel = ? AND platform = ? AND status IN ?",
			q.GameID, q.Env, q.Channel, q.Platform,
			[]string{ReleaseStatusTesting, ReleaseStatusGray, ReleaseStatusFull}).
		Order("updated_at DESC").
		Find(&items).Error
	return items, err
}

// FindByVersion returns the release record for an exact version in the
// scope regardless of status (delta-diff needs the client's current version
// manifest even after it was archived by a newer full).
func (m *GameReleaseModel) FindByVersion(ctx context.Context, gameID, env, channel, platform, version string) (*GameRelease, error) {
	var rel GameRelease
	err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND channel = ? AND platform = ? AND version = ?",
			gameID, env, channel, platform, version).
		Order("updated_at DESC").
		First(&rel).Error
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

// RandomSeedHex generates a fresh gray bucket seed.
func RandomSeedHex() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// NormalizeChannel lowercases and trims a channel id.
func NormalizeChannel(channel string) string {
	return strings.ToLower(strings.TrimSpace(channel))
}
