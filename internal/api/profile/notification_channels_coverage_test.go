package profile

// 覆盖率巡检补测：notification-channels 的 HTTP 层与生产配置源。
//
// 既有 notification_channels_test.go 全部走 WithNotifyBaseStatus 注入路径，
// 生产读取函数 readBaseStatus（settings 单例）与 GetNotificationChannels
// handler 此前 0%。本文件补齐：
//   - handler 层（缺 username 401 / 正常 200 三通道）；
//   - readBaseStatus 的零配置默认值（测试包内 settings 单例为 nil，
//     Layered 各 getter 均有 nil 守卫，行为确定）；
//   - baseStatus 未注入时的回落分支；
//   - WithObjectStore 装配 + resolveAvatar 签名失败退化占位图分支。
//
// 已知不可达（不强行造路径）：ChangePassword 的 UpdatePassword/Update/
// BumpTokenVersion 错误分支需要模型层在「ValidatePassword 与 FindByUsername
// 均成功后」才失败——同一活库上无法在不侵入 mock 模型的情况下构造，
// 包内无模型 mock 设施，按既有登记口径留白。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/platform/objstore"
)

// errStore 是 SignedURL 恒失败的 Store 替身：模拟签名服务不可用。
type errStore struct{}

func (errStore) Put(context.Context, string, objstore.ReadSeeker, int64, string) error {
	return errors.New("not implemented")
}
func (errStore) SignedURL(context.Context, string, string, time.Duration) (string, error) {
	return "", errors.New("sign failed")
}
func (errStore) Delete(context.Context, string) error { return errors.New("not implemented") }
func (errStore) List(context.Context, string, string, string, int) (objstore.ListResult, error) {
	return objstore.ListResult{}, errors.New("not implemented")
}
func (errStore) CreatePrefix(context.Context, string) error { return errors.New("not implemented") }
func (errStore) RenamePrefix(context.Context, string, string) error {
	return errors.New("not implemented")
}

var _ objstore.Store = errStore{}

// handler 缺 username 时 401（此前该 handler 0%）。
func TestHandler_GetNotificationChannels_RequiresUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&Service{})
	ctx, rec := newProfileTestContext("GET", "/api/v1/profile/notification-channels", "")
	handler.GetNotificationChannels(ctx)
	assertProfileHTTPStatus(t, rec, http.StatusUnauthorized)
	assertProfileErrorCode(t, rec, "unauthorized")
}

// handler 正常路径：200 + 三通道齐全（in_app/email/sms）。
func TestHandler_GetNotificationChannels_ReturnsAllChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newChannelService(t, "13800000000", "a@b.c", nil)
	handler := NewHandler(svc)
	ctx, rec := newProfileTestContext("GET", "/api/v1/profile/notification-channels", "")
	ctx.Set("username", "chanuser")
	handler.GetNotificationChannels(ctx)

	assertProfileHTTPStatus(t, rec, http.StatusOK)
	var body struct {
		Channels []NotificationChannelState `json:"channels"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Channels, 3)
	keys := map[string]bool{}
	for _, ch := range body.Channels {
		keys[ch.Key] = true
	}
	assert.True(t, keys["in_app"] && keys["email"] && keys["sms"], "channels = %v", body.Channels)
}

// readBaseStatus 是生产配置源：settings 单例为 nil（本包测试未初始化）时
// 走各 getter 的零值守卫，得到零配置默认值——站内信开、邮件关、SMTP 未配。
func TestReadBaseStatus_NilSingletonYieldsZeroConfigDefaults(t *testing.T) {
	st := readBaseStatus()
	assert.True(t, st.InAppEnabled, "站内信零配置即通")
	assert.False(t, st.EmailEnabled)
	assert.False(t, st.SMTPConfigured)
}

// 未注入 notifyBase 时回落到生产读取（baseStatus 的 fallback 分支）：
// 零配置下邮件如实报不可用，站内信可用。
func TestGetNotificationChannels_FallsBackToProductionBaseStatus(t *testing.T) {
	svc, _ := newChannelService(t, "", "", nil)
	// 覆盖 newChannelService 注入的 seam，回归生产路径。
	svc.notifyBase = nil

	resp := svc.GetNotificationChannels(context.Background(), "chanuser")
	inApp := channelByKey(t, resp, "in_app")
	assert.True(t, inApp.Available)
	assert.True(t, inApp.UserEnabled)
	email := channelByKey(t, resp, "email")
	assert.False(t, email.Available, "零配置 SMTP 不得声称可用")
	assert.Equal(t, "smtp_not_configured", email.Reason)
}

// WithObjectStore 装配后生效：签名失败时头像退化为空串（前端占位图），
// profile 读取本身不报错。
func TestWithObjectStore_SignedURLFailureDegradesAvatar(t *testing.T) {
	service, _, admin := newAvatarService(t)
	service.WithObjectStore(errStore{})

	require.NotNil(t, service.objectStore, "WithObjectStore 应装配 store")
	resp, err := service.GetProfile(context.Background(), admin.Username)
	require.NoError(t, err)
	assert.Empty(t, resp.Avatar, "签名失败应退化为占位图而非报错")
}
