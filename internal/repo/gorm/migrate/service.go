//go:build legacy_repo
// +build legacy_repo

package migrate

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// MigrationManager manages database migrations
type MigrationManager struct {
	db *gorm.DB
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *gorm.DB) *MigrationManager {
	return &MigrationManager{db: db}
}

// GetLatestVersion gets the latest migration version from database
func (m *MigrationManager) GetLatestVersion(ctx context.Context) (string, error) {
	var migration MigrationRecord
	err := m.db.WithContext(ctx).Order("created_at DESC").First(&migration).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "0", nil
		}
		return "", fmt.Errorf("failed to get latest version: %w", err)
	}
	return migration.Version, nil
}

// SaveMigration saves a migration record
func (m *MigrationManager) SaveMigration(ctx context.Context, migration *MigrationRecord) error {
	return m.db.WithContext(ctx).Create(migration).Error
}

// UpdateMigrationStatus updates migration status
func (m *MigrationManager) UpdateMigrationStatus(ctx context.Context, id uint, status MigrationStatus, duration time.Duration, err error) error {
	updates := map[string]interface{}{
		"status":        string(status),
		"executed_at":   time.Now().UTC(),
		"duration":      duration.Milliseconds(),
		"error_message": "",
	}

	if err != nil {
		updates["error_message"] = err.Error()
	}

	return m.db.WithContext(ctx).Model(&MigrationRecord{}).Where("id = ?", id).Updates(updates).Error
}

// GetMigrationsToRun gets migrations that need to be executed
func (m *MigrationManager) GetMigrationsToRun(ctx context.Context, migrations []Migration, direction MigrationDirection) ([]Migration, error) {
	latestVersion, err := m.GetLatestVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	var migrationsToRun []Migration
	for _, migration := range migrations {
		migrationVersion := migration.Version()

		needsRun := false
		if latestVersion == "" {
			needsRun = true
		} else {
			cmp := compareVersions(latestVersion, migrationVersion)
			if direction == DirectionUp && cmp > 0 {
				needsRun = true
			} else if direction == DirectionDown && cmp < 0 {
				needsRun = true
			}
		}

		if needsRun {
			migrationsToRun = append(migrationsToRun, migration)
		}
	}

	return migrationsToRun, nil
}

// ExecuteMigration executes a single migration
func (m *MigrationManager) ExecuteMigration(ctx context.Context, migration Migration, direction MigrationDirection) (*MigrationResult, error) {
	startTime := time.Now()

	var sql string
	var rollbackSQL string

	if direction == DirectionUp {
		sql = migration.Up(m.db)
		rollbackSQL = migration.Down(m.db)
	} else {
		sql = migration.Down(m.db)
		rollbackSQL = migration.Up(m.db)
	}

	migrationResult := &MigrationResult{
		MigrationName: migration.Name(),
		Direction:     direction,
		Status:        StatusPending,
		SQL:           sql,
		DryRun:        false,
	}

	// Log migration start
	migrationRecord := &MigrationRecord{
		Name:        migration.Name(),
		Version:     migration.Version(),
		Description: migration.Description(),
		SQL:         sql,
		Checksum:    calculateChecksum(sql),
		ExecutedAt:  startTime,
		ExecutedBy:  "system",
		Status:      StatusPending,
	}

	err := m.SaveMigration(ctx, migrationRecord)
	if err != nil {
		migrationResult.Status = StatusFailed
		migrationResult.Error = fmt.Sprintf("failed to save migration record: %v", err)
		return migrationResult, nil
	}

	// Execute migration
	var execErr error
	if strings.TrimSpace(sql) == "" {
		execErr = fmt.Errorf("migration %s has no SQL to execute", migration.Name())
	} else {
		execErr = m.db.WithContext(ctx).Exec(sql).Error
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Update migration record based on execution result
	if execErr != nil {
		migrationResult.Status = StatusFailed
		migrationResult.Error = execErr.Error()
		_ = m.UpdateMigrationStatus(ctx, migrationRecord.ID, StatusFailed, duration, execErr)
	} else {
		migrationResult.Status = StatusSuccess
		_ = m.UpdateMigrationStatus(ctx, migrationRecord.ID, StatusSuccess, duration, nil)
	}

	migrationResult.Duration = duration.String()

	return migrationResult, nil
}

// RunMigrations executes pending migrations
func (m *MigrationManager) RunMigrations(ctx context.Context, migrations []Migration, direction MigrationDirection) error {
	if len(migrations) == 0 {
		fmt.Println("No migrations to run")
		return nil
	}

	migrationsToRun, err := m.GetMigrationsToRun(ctx, migrations, direction)
	if err != nil {
		return fmt.Errorf("failed to determine migrations to run: %w", err)
	}

	if len(migrationsToRun) == 0 {
		fmt.Printf("All migrations are up to date (current: %s)\n", func() string {
			v, _ := m.GetLatestVersion(context.Background())
			return v
		}())
		return nil
	}

	fmt.Printf("Running %d migration(s)...\n", len(migrationsToRun))

	// Sort migrations by version
	sort.Slice(migrationsToRun, func(i, j int) bool {
		return compareVersions(migrationsToRun[i].Version(), migrationsToRun[j].Version()) < 0
	})

	var results []*MigrationResult
	var hasError bool

	for _, migration := range migrationsToRun {
		result, err := m.ExecuteMigration(ctx, migration, direction)
		if err != nil {
			hasError = true
			fmt.Printf("Migration %s failed: %v\n", migration.Name(), err)
		} else {
			fmt.Printf("Migration %s completed successfully\n", migration.Name())
		}
		results = append(results, result)

		// Add delay between migrations to avoid conflicts
		time.Sleep(100 * time.Millisecond)
	}

	if hasError {
		return fmt.Errorf("some migrations failed")
	}

	fmt.Printf("All migrations completed successfully\n")
	return nil
}

// MigrateUp runs all pending up migrations
func (m *MigrationManager) MigrateUp(ctx context.Context, migrations []Migration) error {
	return m.RunMigrations(ctx, migrations, DirectionUp)
}

// MigrateDown runs down migrations
func (m *MigrationManager) MigrateDown(ctx context.Context, migrations []Migration) error {
	return m.RunMigrations(ctx, migrations, DirectionDown)
}

// GetMigrationHistory returns migration execution history
func (m *MigrationManager) GetMigrationHistory(ctx context.Context, limit int) ([]MigrationRecord, error) {
	var migrations []MigrationRecord
	query := m.db.WithContext(ctx).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&migrations).Error
	return migrations, err
}

// createMigrationTable creates the migrations table if it doesn't exist
func (m *MigrationManager) createMigrationTable(ctx context.Context) error {
	sql := `
		CREATE TABLE IF NOT EXISTS migration_records (
			id SERIAL PRIMARY KEY,
			name VARCHAR(128) NOT NULL UNIQUE,
			version VARCHAR(32) NOT NULL,
			description TEXT,
			sql LONGTEXT,
			checksum VARCHAR(64),
			executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			executed_by VARCHAR(64),
			status VARCHAR(16) DEFAULT 'pending',
			duration_ms INTEGER DEFAULT 0,
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_migration_records_version ON migration_records(version);
		CREATE INDEX IF NOT EXISTS idx_migration_records_created_at ON migration_records(created_at);
		CREATE INDEX IF NOT EXISTS idx_migration_records_status ON migration_records(status);
	`

	return m.db.WithContext(ctx).Exec(sql).Error
}

// Init ensures migration table exists
func (m *MigrationManager) Init(ctx context.Context) error {
	// Check if migrations table exists
	var tableExists bool
	err := m.db.WithContext(ctx).Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'migration_records')").Scan(&tableExists).Error
	if err != nil {
		return fmt.Errorf("failed to check migration table: %w", err)
	}

	if !tableExists {
		fmt.Println("Creating migration records table...")
		if err := m.createMigrationTable(ctx); err != nil {
			return fmt.Errorf("failed to create migration table: %w", err)
		}
		fmt.Println("Migration records table created successfully")
	}

	return nil
}

// compareVersions compares two version strings
func compareVersions(v1, v2 string) int {
	// Simple version comparison - assumes format like "202312011200"
	if len(v1) != len(v2) || len(v1) < 8 {
		return strings.Compare(v1, v2)
	}

	v1Num := parseIntVersion(v1)
	v2Num := parseIntVersion(v2)

	if v1Num < v2Num {
		return -1
	} else if v1Num > v2Num {
		return 1
	}
	return 0
}

// parseIntVersion converts version string to integer for comparison
func parseIntVersion(version string) int {
	if len(version) < 8 {
		return 0
	}

	num := 0
	for i := 0; i < 8; i++ {
		digit := int(version[i] - '0')
		if digit < 0 || digit > 9 {
			continue
		}
		num = num*10 + digit
	}

	return num
}

// calculateChecksum calculates MD5 checksum of SQL
func calculateChecksum(sql string) string {
	// Simple checksum calculation - in production, use a proper crypto package
	var sum int32
	for _, char := range sql {
		sum += int32(char)
	}
	return fmt.Sprintf("%x", sum)
}
