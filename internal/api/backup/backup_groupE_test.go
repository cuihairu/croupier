// Coverage target: the CurrentUsername hit branch of backup
// service.saveBackupsToExtensionInstallation (service.go:347-349).
//
// The operator of this function defaults to "system" and is rewritten only when a
// non-empty username is injected into ctx; existing cases all call it with
// context.Background(). This file injects a username and reads it back to verify
// the landed content.
package backup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveBackupsToExtensionInstallation_WithUsernameGroupE(t *testing.T) {
	env := setupBackupFlowEnv(t)
	installBackupAdvancedExtension(t, env, map[string]any{})

	ctx := context.WithValue(context.Background(), "username", "alice")
	require.NoError(t, env.service.saveBackupsToExtensionInstallation(ctx, []Backup{
		{Id: "bkp-groupE-1", Name: "one", Type: "database", Status: "completed"},
	}))

	items, ok, err := env.service.loadBackupsFromExtensionInstallation(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "bkp-groupE-1", items[0].Id)
	assert.Equal(t, "one", items[0].Name)
}
