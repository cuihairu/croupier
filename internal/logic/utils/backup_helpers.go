package utils

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/google/uuid"
)

// Backup represents a backup item in API responses.
type Backup struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// BuildBackupDTO converts a Backup model to API shape.
func BuildBackupDTO(backup *model.Backup) Backup {
	if backup == nil {
		return Backup{}
	}

	id := strings.TrimSpace(backup.BackupID)
	if id == "" {
		id = fmt.Sprintf("%d", backup.ID)
	}

	return Backup{
		Id:        id,
		Name:      backup.Name,
		Size:      backup.Size,
		Type:      backup.Type,
		Status:    backup.Status,
		CreatedAt: helper.FormatTimestamp(backup.CreatedAt),
	}
}

// BuildBackupList converts a slice of models.
func BuildBackupList(backups []model.Backup) []Backup {
	items := make([]Backup, 0, len(backups))
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
