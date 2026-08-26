package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newStore(t *testing.T) *model.PlatformSettingModel {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:settings_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return model.NewPlatformSettingModel(db)
}

func TestLayered_ThreeLayers(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{SiteName: "yaml-name", DefaultLocale: "zh-CN"}, store)

	// L2 from config, no L3.
	v, src, ok := l.GetString(context.Background(), KeySiteName)
	require.True(t, ok)
	assert.Equal(t, "yaml-name", v)
	assert.Equal(t, "config", src)

	// Unknown key → default false.
	_, _, ok = l.GetString(context.Background(), KeyFooterICP)
	assert.False(t, ok)

	// L3 overrides L2; source flips to database.
	require.NoError(t, store.Set(context.Background(), KeySiteName, json.RawMessage(`"db-name"`), "tester"))
	l.Reload(context.Background(), store)
	v, src, _ = l.GetString(context.Background(), KeySiteName)
	assert.Equal(t, "db-name", v)
	assert.Equal(t, "database", src)

	// Clear → back to config file.
	require.NoError(t, store.Clear(context.Background(), KeySiteName))
	l.Reload(context.Background(), store)
	v, src, _ = l.GetString(context.Background(), KeySiteName)
	assert.Equal(t, "yaml-name", v)
	assert.Equal(t, "config", src)
}

func TestLayered_FailOpenBeforeL3Load(t *testing.T) {
	resetForTest()
	// Fresh singleton not yet loaded with L3 (store present but empty).
	store := newStore(t)
	l := InitLayered(context.Background(), nil, store)
	_, src, ok := l.GetString(context.Background(), KeySiteName)
	assert.False(t, ok)
	assert.Equal(t, "default", src)
}

func TestWhitelist(t *testing.T) {
	assert.True(t, IsValidKey(KeySiteName))
	assert.True(t, IsValidKey(KeyFooterLinks))
	assert.False(t, IsValidKey("database.dsn"), "bootstrap keys must never be L3")
	assert.False(t, IsValidKey(""))
}

func TestSiteSnapshot_Sources(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{SiteName: "yaml-name"}, store)
	require.NoError(t, store.Set(context.Background(), KeyFooterICP, json.RawMessage(`"京ICP备123号"`), "tester"))
	l.Reload(context.Background(), store)

	snap := l.SiteSnapshot()
	assert.Equal(t, "yaml-name", snap.SiteName)
	assert.Equal(t, "config", snap.Sources[KeySiteName])
	assert.Equal(t, "京ICP备123号", snap.FooterICP)
	assert.Equal(t, "database", snap.Sources[KeyFooterICP])
}
