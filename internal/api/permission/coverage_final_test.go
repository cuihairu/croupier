package permission

import (
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestService_List_ModelError covers the branch where the underlying
// PermissionModel.List query fails after the permission check passes.
func TestService_List_ModelError(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	require.NoError(t, db.Migrator().DropTable("permissions"))
	defer func() {
		require.NoError(t, model.AutoMigrate(db))
	}()

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}
