package cicd_test

// AUD-046 regression: TestConnection's SSRF convergence and stable error codes.
//
// Root cause: the probe primitive previously returned the raw dial error to the client
// (refused / no route / timeout can distinguish the liveness shape of an intranet host,
// effectively a free intranet scanner), did not validate the scheme, and performed no
// resolution-level private-network blocking. The fix: only http/https schemes are
// accepted (400 cicd_probe_invalid_target); when the secguard site guard is on,
// resolution-level rejection covers private/loopback/link-local addresses
// (sec.allowIPs can let an intranet CICD through, 400 cicd_probe_target_blocked); what
// the client sees is always stable wording, with the original text going only to the
// log.

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/api/cicd"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aud046CodeOf(t *testing.T, err error) *errorx.CodeError {
	t.Helper()
	var cerr *errorx.CodeError
	require.ErrorAs(t, err, &cerr)
	return cerr
}

// Only http/https schemes are accepted: when legacy rows or direct DB writes bypass
// the write-time validation (ValidateCicdIntegration only runs on the Create/Update
// paths), the probe acts as the fallback guard.
func TestCicdProbe_Aud046_SchemeRestricted(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	svcCicd := cicd.NewService(ctxSvc)
	ctx := context.Background()
	for i, endpoint := range []string{
		"ftp://ci.example.com",
		"file:///etc/passwd",
		"gopher://legacy.example",
	} {
		row := &model.CicdIntegration{
			GameID: "demo", Env: "prod", Kind: "generic",
			Name: "aud046-scheme-" + strconv.Itoa(i), Endpoint: endpoint, Enabled: true,
		}
		require.NoError(t, db.Create(row).Error)
		_, err := svcCicd.TestConnection(ctx, row.ID)
		require.Error(t, err, endpoint)
		cerr := aud046CodeOf(t, err)
		assert.Equal(t, 400, cerr.Code, endpoint)
		assert.Equal(t, "cicd_probe_invalid_target", cerr.StableCode, endpoint)
	}
}

// secguard site guard enabled: resolution-level blocking of loopback/private addresses
// (sec.allowIPs can let an intranet CICD through); the block surfaces as 400
// cicd_probe_target_blocked without echoing the target address, and no real dial
// happens.
func TestCicdProbe_Aud046_SecguardBlocksPrivateTarget(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	require.NoError(t, db.AutoMigrate(&model.PlatformSetting{}))
	store := model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	t.Cleanup(settings.ResetForTest)
	require.NoError(t, store.Set(context.Background(), settings.KeySecSSRFProtection, json.RawMessage(`true`), "tester"))
	settings.Current().Reload(context.Background(), store)

	svcCicd := cicd.NewService(ctxSvc)
	ctx := context.Background()
	row := &model.CicdIntegration{
		GameID: "demo", Env: "prod", Kind: "generic",
		Name: "aud046-guard", Endpoint: "http://127.0.0.1:1", Enabled: true,
	}
	require.NoError(t, db.Create(row).Error)

	_, err := svcCicd.TestConnection(ctx, row.ID)
	require.Error(t, err)
	cerr := aud046CodeOf(t, err)
	assert.Equal(t, 400, cerr.Code)
	assert.Equal(t, "cicd_probe_target_blocked", cerr.StableCode)
	assert.NotContains(t, cerr.Message, "127.0.0.1", "拦截文案不回显内网地址")

	// Once sec.allowIPs lets it through, the guard lets it through and the call reaches
	// a real dial (no listener on the local port -> unreachable); the client only sees
	// the stable wording, with the raw refused difference going only to the log.
	require.NoError(t, store.Set(context.Background(), settings.KeySecAllowIPs, json.RawMessage(`"127.0.0.1"`), "tester"))
	settings.Current().Reload(context.Background(), store)
	resp, err := svcCicd.TestConnection(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.OK)
	assert.Contains(t, resp.Message, "不可达")
	assert.NotContains(t, resp.Message, "refused")
}

// Guard disabled (the default): a private target keeps dial-level semantics with no
// extra tightening (self-hosted CICD lives on the intranet), but the failure wording
// still has a stable form and no longer passes back the raw dial error - intranet
// topology hints (refused can distinguish host liveness) do not leak.
func TestCicdProbe_Aud046_UnreachableStableMessage(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	svcCicd := cicd.NewService(ctxSvc)
	ctx := context.Background()
	row := &model.CicdIntegration{
		GameID: "demo", Env: "prod", Kind: "generic",
		Name: "aud046-unreachable", Endpoint: "http://127.0.0.1:1", Enabled: true,
	}
	require.NoError(t, db.Create(row).Error)

	resp, err := svcCicd.TestConnection(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.OK)
	assert.Contains(t, resp.Message, "不可达")
	assert.NotContains(t, resp.Message, "refused")
	assert.NotContains(t, resp.Message, "connect")
}
