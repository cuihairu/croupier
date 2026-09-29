// 覆盖率巡检第二十轮（wt-api）：notification.go defaultPostJSONWithHeaders
// 的出站安全守卫拦截翼（760-761）收口——sec.* 键启用时非白名单端口在
// CheckURL 即被拒（不发起真实连接）。注入键选 sec.allowPorts（端口判定
// 在 DNS 解析前，规避本机解析器劫持短主机名的坑，见第九轮台账）。
// settings 单例操纵仅限顺序用例（本包无 t.Parallel，已核实）。
package approvals

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDefaultPostJSONWithHeaders_SecguardPortBlocked(t *testing.T) {
	settings.ResetForTest()
	t.Cleanup(settings.ResetForTest)

	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/notif_secguard.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PlatformSetting{}))
	store := model.NewPlatformSettingModel(db)
	ctx := context.Background()
	settings.InitLayered(ctx, &settings.ConfigInput{}, store)

	require.NoError(t, store.Set(ctx, settings.KeySecAllowPorts, json.RawMessage(`"80"`), "tester"))
	settings.Current().Reload(ctx, store)

	err = defaultPostJSONWithHeaders(ctx, "http://203.0.113.9:9999/hook", []byte(`{}`), nil)
	require.Error(t, err, "非白名单端口的出站须被 secguard 拦截")
	assert.Contains(t, err.Error(), "端口", "拦截原因应为端口不在允许清单")
}
