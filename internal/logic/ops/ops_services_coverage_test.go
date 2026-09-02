// 覆盖目标：logic/ops 包 OpsServices 全路径（权限通过 + server/agent 聚合）、
// OpsClientWrapper.ReportMetrics 的 proto.Marshal 失败分支。
package ops

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newOpsPermSvcCtx 构造带 admin 用户/角色/权限的 sqlite 上下文，使
// RequireAnyPermission 通过（参考 internal/logic/utils/permission_guard_test.go）。
func newOpsPermSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin := &model.Admin{Username: "ops-admin", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "pass1234"))
	role := &model.Role{Name: "ops-admin", Description: "ops"}
	require.NoError(t, roleModel.Create(context.Background(), role))
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))
	require.NoError(t, roleModel.ReplacePermissions(context.Background(), role.ID, []string{"ops:read"}))

	return &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: model.NewPermissionModel(db),
	}
}

func TestOpsServicesLogic_OpsServices_FullPath(t *testing.T) {
	svcCtx := newOpsPermSvcCtx(t)
	store := registry.NewStore()

	// 健康 agent：带 providers / labels / functions。
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-healthy",
		Addr:     "10.0.0.1:19090",
		GameID:   "demo",
		Env:      "prod",
		Version:  "v1.2.3",
		Region:   "cn-north",
		Zone:     "zone-a",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Labels:   map[string]string{"os": "linux"},
		Functions: map[string]registry.FunctionMeta{
			"fn.enabled":  {Enabled: true},
			"fn.disabled": {Enabled: false},
		},
		Providers: []registry.ProviderSession{
			{
				ProviderID:   "provider-1",
				Addr:         "10.0.0.1:8081",
				Version:      "v1",
				LastSeenUnix: time.Now().Unix(),
				FunctionIDs:  []string{"fn.enabled", "fn.disabled"},
			},
		},
	})

	// 过期 agent（ExpireAt 已过）。
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-expired",
		ExpireAt: time.Now().Add(-time.Hour),
		LastSeen: time.Now().Add(-2 * time.Hour),
		Labels:   nil,
	})

	// 空白 AgentID 跳过；零 LastSeen 回退 ExpireAt。
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "   ",
		ExpireAt: time.Now().Add(time.Minute),
	})
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-nolastseen",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Time{},
	})

	svcCtx.Config = config.Config{
		Server: config.ServerConfig{Host: "0.0.0.0", Port: 18780},
		Region: "cn-north",
		Zone:   "zone-a",
		Labels: map[string]string{"team": "platform"},
	}
	svcCtx.ServerVersion = "v9.9.9"
	svcCtx.StartTime = time.Now()
	svcCtx.RegistryStore = store

	ctx := context.WithValue(context.Background(), "username", "ops-admin")
	logic := NewOpsServicesLogic(ctx, svcCtx)
	resp, err := logic.OpsServices(&OpsServicesRequest{})
	require.NoError(t, err)

	// server + 3 个有效 agent（空白 ID 被跳过）。
	require.Len(t, resp.Services, 4)
	assert.Equal(t, 4, resp.Total)

	var server, healthy, expired, noLast *OpsServiceItem
	for i := range resp.Services {
		switch resp.Services[i].ID {
		case "server":
			server = &resp.Services[i]
		case "agent-healthy":
			healthy = &resp.Services[i]
		case "agent-expired":
			expired = &resp.Services[i]
		case "agent-nolastseen":
			noLast = &resp.Services[i]
		}
	}

	require.NotNil(t, server)
	assert.Equal(t, "localhost:18780", server.Address) // 0.0.0.0 → localhost
	assert.Equal(t, "v9.9.9", server.Version)
	assert.Equal(t, "platform", server.Labels["team"])
	assert.NotEmpty(t, server.LastSeen)

	require.NotNil(t, healthy)
	assert.Equal(t, "healthy", healthy.Status)
	assert.Equal(t, "agent", healthy.Type)
	assert.Equal(t, 1, healthy.FunctionsCount) // 仅统计 enabled
	require.NotNil(t, healthy.Metadata)
	assert.Equal(t, 1, healthy.Metadata.ProcessesCount)
	assert.Equal(t, "provider-1", healthy.Metadata.Processes[0].ServiceID)
	assert.Equal(t, 2, healthy.Metadata.Processes[0].Functions)

	require.NotNil(t, expired)
	assert.Equal(t, "expired", expired.Status)
	assert.NotNil(t, expired.Labels) // nil labels 归一为空 map

	require.NotNil(t, noLast)
	assert.NotEmpty(t, noLast.LastSeen) // 零 LastSeen 回退 ExpireAt-30s

	// 非 0.0.0.0 host 的地址格式。
	svcCtx.Config.Server.Host = "10.1.2.3"
	resp2, err := logic.OpsServices(&OpsServicesRequest{})
	require.NoError(t, err)
	assert.Equal(t, "10.1.2.3:18780", resp2.Services[0].Address)
}

func TestOpsServicesLogic_OpsServices_PermissionDenied(t *testing.T) {
	svcCtx := newOpsPermSvcCtx(t)
	// 角色只授予 ops:read，但这里用户不存在 → 401 类错误。
	logic := NewOpsServicesLogic(context.WithValue(context.Background(), "username", "ghost"), svcCtx)
	_, err := logic.OpsServices(&OpsServicesRequest{})
	require.Error(t, err)
}

func TestOpsClientWrapper_ReportMetrics_MarshalError(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &opsTestCaller{responses: map[uint32][]byte{}}}
	// proto3 string 字段含非法 UTF-8 时 proto.Marshal 报错。
	_, err := wrapper.ReportMetrics(context.Background(), &opsv1.MetricsReport{
		AgentId: "\xff\xfe",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal metrics report failed")
}

var _ = fmt.Sprintf
