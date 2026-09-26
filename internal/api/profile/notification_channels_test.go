package profile

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
)

// stubSMS 是「已接入」的测试替身：Configured 由测试控制。
type stubSMS struct {
	configured bool
	provider   string
	sends      int
}

func (s *stubSMS) Send(context.Context, string, string, map[string]string) error {
	s.sends++
	return nil
}
func (s *stubSMS) Configured() bool { return s.configured }
func (s *stubSMS) Describe() approvals.SMSProviderInfo {
	return approvals.SMSProviderInfo{Provider: s.provider, Signature: "sig", Template: "tpl"}
}

// newChannelService 造一个带指定手机号/邮箱的账号与可选的短信注册表。
func newChannelService(t *testing.T, phone, email string, sms *approvals.SMSRegistry) (*Service, *model.AdminModel) {
	t.Helper()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	require.NoError(t, adminModel.Create(context.Background(), &model.Admin{
		Username: "chanuser",
		Nickname: "n",
		Phone:    phone,
		Email:    email,
		Status:   1,
	}, "CorrectPass123"))

	svc := NewService(adminModel, model.NewGameModel(db), model.NewRoleModel(db))
	if sms != nil {
		svc = svc.WithSMSRegistry(sms)
	}
	// 平台侧状态用固定值，避免依赖 settings 单例（跨包不可控）
	svc = svc.WithNotifyBaseStatus(func() NotificationBaseStatus {
		return NotificationBaseStatus{InAppEnabled: true, EmailEnabled: true}
	})
	return svc, adminModel
}

func channelByKey(t *testing.T, resp *NotificationChannelsResponse, key string) NotificationChannelState {
	t.Helper()
	for _, c := range resp.Channels {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("响应中缺少通道 %s", key)
	return NotificationChannelState{}
}

// ---------------------------------------------------------------- BUG-016
// 核心不变量：**可用性只由服务商接入状态决定**。

func TestNotificationChannels_SMSUnavailableWithoutProvider(t *testing.T) {
	svc, _ := newChannelService(t, "13800000000", "a@b.c", nil)
	resp := svc.GetNotificationChannels(context.Background(), "chanuser")

	sms := channelByKey(t, resp, "sms")
	assert.False(t, sms.Available, "未接入短信服务商时不得声称可用")
	assert.Equal(t, "sms provider not configured", sms.Reason)
	// 关键：用户填了手机号**也不能**变成「已开启」
	assert.True(t, sms.HasTarget, "手机号确实已填写——但这不该让通道变成可用")
	assert.Nil(t, sms.Info, "未接入时不应返回服务商信息")
}

func TestNotificationChannels_SMSAvailableOnceProviderConfigured(t *testing.T) {
	reg := approvals.NewSMSRegistry()
	reg.RegisterSMSProvider(&stubSMS{configured: true, provider: "aliyun"})
	svc, _ := newChannelService(t, "13800000000", "a@b.c", reg)

	sms := channelByKey(t, svc.GetNotificationChannels(context.Background(), "chanuser"), "sms")
	assert.True(t, sms.Available)
	assert.Empty(t, sms.Reason, "可用时不应带不可用原因")
	assert.NotNil(t, sms.Info)
	assert.Equal(t, "aliyun", sms.Info.Provider)
}

func TestNotificationChannels_SMSProviderRegisteredButNotConfigured(t *testing.T) {
	// 注册了 provider 但凭据不齐 → 仍必须报不可用（Configured 是第二道闸）
	reg := approvals.NewSMSRegistry()
	reg.RegisterSMSProvider(&stubSMS{configured: false, provider: "aliyun"})
	svc, _ := newChannelService(t, "13800000000", "", reg)

	sms := channelByKey(t, svc.GetNotificationChannels(context.Background(), "chanuser"), "sms")
	assert.False(t, sms.Available, "凭据不齐时不得声称可用")
}

func TestNotificationChannels_EmailUnavailableWithoutSMTP(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	require.NoError(t, adminModel.Create(context.Background(), &model.Admin{
		Username: "mailuser", Email: "a@b.c", Status: 1,
	}, "CorrectPass123"))
	svc := NewService(adminModel, model.NewGameModel(db), model.NewRoleModel(db)).
		WithNotifyBaseStatus(func() NotificationBaseStatus { return NotificationBaseStatus{} })

	email := channelByKey(t, svc.GetNotificationChannels(context.Background(), "mailuser"), "email")
	assert.False(t, email.Available, "SMTP 未配置时 EmailSender 是 no-op，不得声称可用")
	assert.Equal(t, "smtp_not_configured", email.Reason)
	assert.True(t, email.HasTarget)
}

func TestNotificationChannels_EmailAvailableWithSMTP(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	require.NoError(t, adminModel.Create(context.Background(), &model.Admin{
		Username: "mailuser2", Email: "a@b.c", Status: 1,
	}, "CorrectPass123"))
	svc := NewService(adminModel, model.NewGameModel(db), model.NewRoleModel(db)).
		WithNotifyBaseStatus(func() NotificationBaseStatus {
			return NotificationBaseStatus{SMTPConfigured: true, EmailEnabled: true}
		})

	email := channelByKey(t, svc.GetNotificationChannels(context.Background(), "mailuser2"), "email")
	assert.True(t, email.Available)
	assert.Empty(t, email.Reason)
	assert.True(t, email.UserEnabled)
}

func TestNotificationChannels_InAppAlwaysAvailableByDefault(t *testing.T) {
	svc, _ := newChannelService(t, "", "", nil)
	inApp := channelByKey(t, svc.GetNotificationChannels(context.Background(), "chanuser"), "in_app")
	assert.True(t, inApp.Available, "站内信零配置即通")
	assert.True(t, inApp.UserEnabled, "默认开启")
	assert.False(t, inApp.RequiresTarget, "站内信不需要手机号/邮箱")
}

func TestNotificationChannels_InAppDisabledByAdmin(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	require.NoError(t, adminModel.Create(context.Background(), &model.Admin{
		Username: "inappuser", Status: 1,
	}, "CorrectPass123"))
	svc := NewService(adminModel, model.NewGameModel(db), model.NewRoleModel(db)).
		WithNotifyBaseStatus(func() NotificationBaseStatus {
			return NotificationBaseStatus{InAppEnabled: false}
		})
	inApp := channelByKey(t, svc.GetNotificationChannels(context.Background(), "inappuser"), "in_app")
	assert.False(t, inApp.UserEnabled, "管理员关闭后应如实反映")
	assert.Equal(t, "in_app_disabled", inApp.Reason)
}

func TestNotificationChannels_UnknownUserYieldsEmptyList(t *testing.T) {
	svc, _ := newChannelService(t, "", "", nil)
	resp := svc.GetNotificationChannels(context.Background(), "nobody")
	assert.Empty(t, resp.Channels, "账号不存在时返回空列表而非报错")
}

// ---------------------------------------------------------- SMSRegistry 自身

func TestSMSRegistry_DefaultIsNotConfigured(t *testing.T) {
	reg := approvals.NewSMSRegistry()
	st := reg.Status()
	assert.False(t, st.Available)
	assert.NotEmpty(t, st.Reason)
}

func TestSMSRegistry_SendWithoutProviderReturnsNotConfigured(t *testing.T) {
	reg := approvals.NewSMSRegistry()
	err := reg.Send(context.Background(), "13800000000", "tpl", nil)
	assert.ErrorIs(t, err, approvals.ErrSMSNotConfigured,
		"未接入时发送必须明确失败，而不是静默成功（静默成功正是假状态的根源）")
}

func TestSMSRegistry_SendRejectsEmptyRecipient(t *testing.T) {
	reg := approvals.NewSMSRegistry()
	reg.RegisterSMSProvider(&stubSMS{configured: true})
	assert.Error(t, reg.Send(context.Background(), "   ", "tpl", nil))
}

func TestSMSRegistry_RegisterNilFallsBackToUnconfigured(t *testing.T) {
	reg := approvals.NewSMSRegistry()
	reg.RegisterSMSProvider(&stubSMS{configured: true})
	require.True(t, reg.Status().Available)
	reg.RegisterSMSProvider(nil)
	assert.False(t, reg.Status().Available, "置 nil 应回落为未接入")
}
