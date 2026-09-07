package backupexec

import (
	"context"
	"os"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBackup_DumpSHAFailureAfterChmod(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires non-root to enforce file permissions")
	}
	m, db := newBackupTestModel(t)
	createBackupRow(t, m, "bk-chmod")

	e := New("mysql", "root:root@tcp(127.0.0.1:3306)/none", m, newFakeObjStore(nil), "backups/")
	e.execCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		for _, a := range args {
			const prefix = "--result-file="
			if len(a) > len(prefix) && a[:len(prefix)] == prefix {
				require.NoError(t, os.Chmod(a[len(prefix):], 0o0000))
			}
		}
		return nil, nil
	}

	err := e.RunBackup(context.Background(), "bk-chmod", "daily", "full")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")

	var got model.Backup
	require.NoError(t, db.Where("backup_id = ?", "bk-chmod").First(&got).Error)
	assert.Equal(t, "failed", got.Status)
	assert.NotEmpty(t, got.ErrorMessage)
}
