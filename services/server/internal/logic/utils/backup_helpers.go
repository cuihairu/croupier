package utils

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/google/uuid"
)

// BuildBackupDTO converts a Backup model to API shape.
func BuildBackupDTO(backup *model.Backup) types.Backup {
	if backup == nil {
		return types.Backup{}
	}

	id := strings.TrimSpace(backup.BackupID)
	if id == "" {
		id = fmt.Sprintf("%d", backup.ID)
	}

	return types.Backup{
		Id:        id,
		Name:      backup.Name,
		Size:      backup.Size,
		Type:      backup.Type,
		Status:    backup.Status,
		CreatedAt: FormatTimestamp(backup.CreatedAt),
	}
}

// BuildBackupList converts a slice of models.
func BuildBackupList(backups []model.Backup) []types.Backup {
	items := make([]types.Backup, 0, len(backups))
	for i := range backups {
		items = append(items, BuildBackupDTO(&backups[i]))
	}
	return items
}

// GenerateBackupID returns a stable ID prefixing UUID.
func GenerateBackupID() string {
	raw := uuid.New().String()
	return fmt.Sprintf("bkp_%s", strings.ReplaceAll(raw, "-", ""))
}

// GuessBackupFilename picks a usable filename for downloads.
func GuessBackupFilename(backup *model.Backup) string {
	if backup == nil {
		return ""
	}
	name := strings.TrimSpace(backup.Name)
	if name != "" {
		return name
	}
	if backup.Location != "" {
		if base := filepath.Base(backup.Location); base != "." && base != "/" {
			return base
		}
	}
	id := strings.TrimSpace(backup.BackupID)
	if id != "" {
		return fmt.Sprintf("%s.bak", id)
	}
	return fmt.Sprintf("backup-%d.bak", backup.ID)
}
