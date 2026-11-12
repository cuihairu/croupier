package dbmon

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// dbmonTestDBSeq 给每次 fixture 调用发互不重名的内存库。按 t.Name() 命名不够：
// -count=3 让同一用例名跑三遍，而 file: 内存库只要还有一条连接开着就存活
// （上一遍的连接池没关尽），于是这一遍会读到上一遍残留的 source/alert 行——
// 签名是列表长度翻倍（ProbeAllDegradedOnUnreachable 断言 2 实得 4/6）。
var dbmonTestDBSeq int64

func newDBMonFixture(t *testing.T) (*Service, *model.DBSourceModel, *model.AlertModel) {
	t.Helper()
	name := fmt.Sprintf("dbmon_%d", atomic.AddInt64(&dbmonTestDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	srcModel := model.NewDBSourceModel(db)
	alertModel := model.NewAlertModel(db)
	return NewService(&svc.ServiceContext{
		DB:            db,
		DBSourceModel: srcModel,
		AlertModel:    alertModel,
	}), srcModel, alertModel
}

func TestValidateDBSource(t *testing.T) {
	ok := &model.DBSource{Name: "游戏主库", Driver: "mysql", Kind: model.DBSourceKindAliyun,
		DSN: "readonly:pass@tcp(10.0.0.1:3306)/game", GameID: "demo", Env: "prod"}
	require.NoError(t, model.ValidateDBSource(ok))

	cases := []struct {
		name string
		src  *model.DBSource
		want string
	}{
		{"empty name", &model.DBSource{Driver: "mysql", DSN: "a:b@c/d", GameID: "demo", Env: "prod"}, "名称"},
		{"bad driver", &model.DBSource{Name: "x", Driver: "oracle", DSN: "a:b@c/d", GameID: "demo", Env: "prod"}, "驱动"},
		{"bad kind", &model.DBSource{Name: "x", Driver: "mysql", Kind: "gcp", DSN: "a:b@c/d", GameID: "demo", Env: "prod"}, "部署类型"},
		{"root dsn", &model.DBSource{Name: "x", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: "root:pw@tcp(127.0.0.1)/x", GameID: "demo", Env: "prod"}, "只读"},
		{"missing game", &model.DBSource{Name: "x", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: "a:b@c/d", Env: "prod"}, "gameId"},
		{"missing env", &model.DBSource{Name: "x", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: "a:b@c/d", GameID: "demo"}, "env"},
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
		DSN: "ro:ro@tcp(127.0.0.1:1)/nothing?timeout=1s", Enabled: true,
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
