package cicd_test

// AUD-046 回归：TestConnection 的 SSRF 收敛与稳定错误码。
//
// 根因：探测原语此前对客户端回传原始拨号错误（refused / no route /
// timeout 可区分内网主机存活形态，等于免费内网扫描器），且不校验
// scheme、不做解析级私网拦截。修复：scheme 只认 http/https（400
// cicd_probe_invalid_target）；secguard 站点守卫开启时解析级拒绝私网/
// 回环/链路本地地址（sec.allowIPs 可放行，400 cicd_probe_target_blocked）；
// 客户端可见信息一律稳定文案，原文只进日志。

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

// scheme 只认 http/https：历史行/直写库绕过写入校验（ValidateCicdIntegration
// 只在 Create/Update 路径生效）时由探测兜底。
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

// secguard 站点守卫开启：解析级拦截回环/私网地址（sec.allowIPs 可放行内网
// CICD），拦截以 400 cicd_probe_target_blocked 呈现且不回显目标地址，
// 真实拨号不再发生。
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

	// sec.allowIPs 放行后守卫放行，落到真实拨号（本机端口无监听 → 不可达），
	// 客户端只见稳定文案，原始 refused 差异只进日志。
	require.NoError(t, store.Set(context.Background(), settings.KeySecAllowIPs, json.RawMessage(`"127.0.0.1"`), "tester"))
	settings.Current().Reload(context.Background(), store)
	resp, err := svcCicd.TestConnection(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.OK)
	assert.Contains(t, resp.Message, "不可达")
	assert.NotContains(t, resp.Message, "refused")
}

// 守卫关闭（默认）：私网目标沿用拨号级语义不额外收紧（自托管 CICD 常驻
// 内网），但失败文案仍是稳定形态，不再回传原始拨号错误——内网拓扑线索
// （refused 可区分主机存活）不外泄。
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
