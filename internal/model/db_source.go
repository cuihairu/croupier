package model

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// DBSource is one registered game database instance monitored from the ops
// center (docs/research/db-monitoring-design.md §5 P1). The DSN must be a
// read-only account: probes never write.
type DBSource struct {
	gorm.Model
	Name   string `gorm:"size:128"`
	Driver string `gorm:"size:32"` // mysql | postgres
	// Deployment kind drives capability display (what the probe can and
	// cannot see) and the deep-diagnosis external link.
	Kind      string `gorm:"size:32"` // self | aliyun | huawei
	DSN       string `gorm:"size:512"`
	GameID    string `gorm:"size:64;index:idx_db_source_scope,priority:1"`
	Env       string `gorm:"size:64;index:idx_db_source_scope,priority:2"`
	Enabled   bool   `gorm:"default:true"`
	Sort      int    `gorm:"default:0"`
	CreatedBy string `gorm:"size:64"`

	// Thresholds for alert integration (0 = use platform defaults).
	LockWaitWarn  int `gorm:"default:0"` // lock wait entries above → warning
	ConnWarnRatio int `gorm:"default:0"` // connections/max ratio percent
}

func (DBSource) TableName() string { return "db_sources" }

// DB source kinds (closed set; frontend labels + diagnosis links depend on it).
const (
	DBSourceKindSelf   = "self"
	DBSourceKindAliyun = "aliyun"
	DBSourceKindHuawei = "huawei"
)

// ValidDBSourceKinds is the closed set of deployment kinds.
var ValidDBSourceKinds = map[string]struct{}{
	DBSourceKindSelf: {}, DBSourceKindAliyun: {}, DBSourceKindHuawei: {},
}

// ValidateDBSource checks name/driver/kind/url invariants.
func ValidateDBSource(src *DBSource) error {
	src.Name = strings.TrimSpace(src.Name)
	src.DSN = strings.TrimSpace(src.DSN)
	src.Driver = strings.ToLower(strings.TrimSpace(src.Driver))
	if src.Name == "" {
		return errors.New("数据源名称不能为空")
	}
	if src.Driver != "mysql" && src.Driver != "postgres" {
		return errors.New("驱动仅支持 mysql / postgres")
	}
	if _, ok := ValidDBSourceKinds[src.Kind]; !ok {
		return errors.New("无效的部署类型: " + src.Kind)
	}
	if !strings.Contains(src.DSN, "@") && !strings.Contains(src.DSN, "=") {
		return errors.New("DSN 格式无效")
	}
	// Read-only guard at registration time is advisory (the account itself
	// must be granted read-only by the operator); refuse obvious admin DSNs.
	if strings.Contains(src.DSN, "root:") || strings.Contains(src.DSN, "superuser") {
		return errors.New("请使用只读监控账号，禁止 root/superuser")
	}
	return nil
}

// MaskedDSN hides credentials for API responses.
func (s *DBSource) MaskedDSN() string {
	dsn := s.DSN
	at := strings.LastIndex(dsn, "@")
	colon := strings.Index(dsn, ":")
	proto := strings.Index(dsn, "://")
	userStart := 0
	if proto >= 0 {
		userStart = proto + 3
	}
	if at > userStart && colon > userStart && colon < at {
		return dsn[:colon] + ":***" + dsn[at:]
	}
	if at > userStart {
		return dsn[:userStart] + "***" + dsn[at:]
	}
	return "***"
}

// DBSourceModel provides CRUD for registered database sources.
type DBSourceModel struct {
	db *gorm.DB
}

// NewDBSourceModel creates a helper.
func NewDBSourceModel(db *gorm.DB) *DBSourceModel {
	return &DBSourceModel{db: db}
}

// List returns every registered source including disabled ones.
func (m *DBSourceModel) List(ctx context.Context) ([]DBSource, error) {
	var items []DBSource
	err := m.db.WithContext(ctx).Order("sort ASC, updated_at DESC").Find(&items).Error
	return items, err
}

// FindOne returns a source by id.
func (m *DBSourceModel) FindOne(ctx context.Context, id uint) (*DBSource, error) {
	var src DBSource
	if err := m.db.WithContext(ctx).First(&src, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &src, nil
}

// Create inserts a source.
func (m *DBSourceModel) Create(ctx context.Context, src *DBSource) error {
	return m.db.WithContext(ctx).Create(src).Error
}

// Update applies a partial update map.
func (m *DBSourceModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&DBSource{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes a source.
func (m *DBSourceModel) Delete(ctx context.Context, id uint) error {
	res := m.db.WithContext(ctx).Delete(&DBSource{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
