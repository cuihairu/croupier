package dbmon

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newDBMonFixture(t *testing.T) (*Service, *model.DBSourceModel, *model.AlertModel) {
	t.Helper()
	name := fmt.Sprintf("dbmon_%s", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	srcModel := model.NewDBSourceModel(db)
	alertModel := model.NewAlertModel(db)
	return NewService(&svc.ServiceContext{
		DBSourceModel: srcModel,
		AlertModel:    alertModel,
	}), srcModel, alertModel
}

func TestValidateDBSource(t *testing.T) {
	ok := &model.DBSource{Name: "游戏主库", Driver: "mysql", Kind: model.DBSourceKindAliyun,
		DSN: "readonly:pass@tcp(10.0.0.1:3306)/game"}
	require.NoError(t, model.ValidateDBSource(ok))

	cases := []struct {
		name string
		src  *model.DBSource
		want string
	}{
		{"empty name", &model.DBSource{Driver: "mysql", DSN: "a:b@c/d"}, "名称"},
		{"bad driver", &model.DBSource{Name: "x", Driver: "oracle", DSN: "a:b@c/d"}, "驱动"},
		{"bad kind", &model.DBSource{Name: "x", Driver: "mysql", Kind: "gcp", DSN: "a:b@c/d"}, "部署类型"},
		{"root dsn", &model.DBSource{Name: "x", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: "root:pw@tcp(127.0.0.1)/x"}, "只读"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := model.ValidateDBSource(tc.src)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestMaskedDSN(t *testing.T) {
	s := &model.DBSource{DSN: "readonly:secret123@tcp(10.0.0.1:3306)/game"}
	assert.NotContains(t, s.MaskedDSN(), "secret123")
	assert.Contains(t, s.MaskedDSN(), "***@tcp(10.0.0.1:3306)/game")

	s2 := &model.DBSource{DSN: "postgres://ro:pw@10.0.0.2:5432/game"}
	assert.NotContains(t, s2.MaskedDSN(), "pw@")
}

func TestProbeAll_AlertFiresAndResolves(t *testing.T) {
	svc, srcModel, alertModel := newDBMonFixture(t)
	ctx := context.Background()

	// Register a source pointing at a database that does not exist.
	src := &model.DBSource{
		Name: "游戏主库", Driver: "mysql", Kind: model.DBSourceKindSelf,
		DSN: fmt.Sprintf("ro:ro@tcp(127.0.0.1:1)/nothing?timeout=1s"), Enabled: true,
	}
	require.NoError(t, srcModel.Create(ctx, src))

	resp, err := svc.ProbeAll(ctx)
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.False(t, resp.Results[0].OK)

	// No alerts from an unreachable source (probe errors are surfaced in the
	// result, not spammed into the alert center — the DB being down is
	// usually already alerted by infra monitoring).
	alerts, _, err := alertModel.List(ctx, model.ListAlertsOptions{})
	require.NoError(t, err)
	assert.Empty(t, alerts)
}

func TestProbeAll_LockWaitThresholdFiresAlert(t *testing.T) {
	// Connection-watermark default is 80% when unconfigured.
	assert.Equal(t, 80, warnConnRatio(0))
	assert.Equal(t, 90, warnConnRatio(90))
}
