// 覆盖目标：dispatchExternal 三渠道（钉钉/通用 webhook/邮件）接线、Dispatch nil 接收者、
// dispatchInApp 的 EncodeData 失败降级、joinRecipients 逗号拼接分支。
package notify_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	gsqlite "github.com/glebarez/sqlite"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	notifyservice "github.com/cuihairu/croupier/internal/service/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setNotifySetting(t *testing.T, db *gorm.DB, key, rawValue string) {
	t.Helper()
	m := model.NewPlatformSettingModel(db)
	require.NoError(t, m.Set(context.Background(), key, json.RawMessage(rawValue), "tester"))
	settings.Current().Reload(context.Background(), m)
}

// capturedServer 记录收到的请求（handler 在 server goroutine 内运行，需加锁）。
type capturedServer struct {
	mu     sync.Mutex
	bodies map[string][]string
	srv    *httptest.Server
}

func newCapturedServer(status int) *capturedServer {
	cs := &capturedServer{bodies: map[string][]string{}}
	cs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		cs.mu.Lock()
		cs.bodies[r.URL.Path] = append(cs.bodies[r.URL.Path], string(buf))
		cs.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"errcode":0}`))
	}))
	return cs
}

func (cs *capturedServer) body(path string) []string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.bodies[path]
}

func TestDispatch_NilReceiver(t *testing.T) {
	var svc *notifyservice.Service
	assert.NotPanics(t, func() {
		svc.Dispatch(context.Background(), notifyservice.Event{Title: "t"})
	})
}

func TestDispatch_DingTalkAndWebhook(t *testing.T) {
	svc, db := setupNotify(t)
	cs := newCapturedServer(http.StatusOK)
	defer cs.srv.Close()

	setNotifySetting(t, db, settings.KeyNotifyDingtalkURL, `"`+cs.srv.URL+`/ding"`)
	setNotifySetting(t, db, settings.KeyNotifyWebhookURL, `"`+cs.srv.URL+`/hook"`)
	setNotifySetting(t, db, settings.KeyNotifyWebhookSecret, `"whsec"`)

	svc.Dispatch(context.Background(), notifyservice.Event{
		Type:       "approval.created",
		Title:      "审批待处理",
		Message:    "玩家查询等待审批",
		Recipients: []string{"alice", "bob"},
		Priority:   "high",
	})

	// 钉钉群机器人：一次 markdown 投递，正文含标题。
	ding := cs.body("/ding")
	require.Len(t, ding, 1)
	assert.Contains(t, ding[0], "审批待处理")
	assert.Contains(t, ding[0], "markdown")

	// 通用 webhook：接收人逗号拼接（joinRecipients 全分支），带 audience。
	hook := cs.body("/hook")
	require.Len(t, hook, 1)
	assert.Contains(t, hook[0], `"recipient":"alice,bob"`)
	assert.Contains(t, hook[0], `"audience":"croupier"`)

	// 站内信不受外部渠道配置影响。
	var count int64
	require.NoError(t, db.Model(&model.Message{}).Count(&count).Error)
	assert.Equal(t, int64(2), count)
}

func TestDispatch_ExternalSendFailureLogsOnly(t *testing.T) {
	svc, db := setupNotify(t)
	// 服务端返回 500：钉钉 sender 判定失败，Dispatch 只记日志不 panic。
	cs := newCapturedServer(http.StatusInternalServerError)
	defer cs.srv.Close()
	setNotifySetting(t, db, settings.KeyNotifyDingtalkURL, `"`+cs.srv.URL+`/ding"`)

	assert.NotPanics(t, func() {
		svc.Dispatch(context.Background(), notifyservice.Event{Title: "t", Recipients: []string{"alice"}})
	})
	// 主流程（站内信）不受通知失败影响。
	var count int64
	require.NoError(t, db.Model(&model.Message{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestDispatch_EmailChannel(t *testing.T) {
	svc, db := setupNotify(t)
	// emailEnabled 但未配 host：跳过邮件发送，不报错。
	setNotifySetting(t, db, settings.KeyNotifyEmailEnabled, "true")
	assert.NotPanics(t, func() {
		svc.Dispatch(context.Background(), notifyservice.Event{Title: "t", Recipients: []string{"alice"}})
	})

	// 配置 host=127.0.0.1 保留端口：拨号被拒，失败仅记日志。
	setNotifySetting(t, db, settings.KeyNotifySMTPHost, `"127.0.0.1"`)
	setNotifySetting(t, db, settings.KeyNotifySMTPPort, "1")
	setNotifySetting(t, db, settings.KeyNotifySMTPFrom, `"gm@example.com"`)
	assert.NotPanics(t, func() {
		svc.Dispatch(context.Background(), notifyservice.Event{Title: "t", Recipients: []string{"alice@example.com"}})
	})
	var count int64
	require.NoError(t, db.Model(&model.Message{}).Count(&count).Error)
	assert.Equal(t, int64(2), count)
}

func TestDispatch_InAppDataEncodeFailure(t *testing.T) {
	svc, db := setupNotify(t)
	// Data 含不可序列化值：EncodeData 失败降级为 nil，消息仍写入。
	svc.Dispatch(context.Background(), notifyservice.Event{
		Title:      "t",
		Recipients: []string{"alice"},
		Data:       map[string]interface{}{"ch": make(chan int)},
	})
	var msg model.Message
	require.NoError(t, db.First(&msg).Error)
	assert.Equal(t, "alice", msg.To)
	assert.Empty(t, msg.Data)
}

func TestDispatch_OnlyLayeredNoMessageModel(t *testing.T) {
	// 有 layered 无 messageModel：仅外部渠道，站内信安全跳过。
	_, db := setupNotify(t)
	settings.ResetForTest()
	layered := settings.InitLayered(context.Background(), &settings.ConfigInput{}, model.NewPlatformSettingModel(db))
	svc := notifyservice.New(layered, nil)
	assert.NotPanics(t, func() {
		svc.Dispatch(context.Background(), notifyservice.Event{Title: "t", Recipients: []string{"a"}})
	})
}

// 企业微信/飞书渠道接线：配置 URL 后 dispatchExternal 投递两渠道。
func TestDispatchExternal_WecomAndFeishu(t *testing.T) {
	var wecomBody, feishuBody []byte
	wecomSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wecomBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer wecomSrv.Close()
	feishuSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		feishuBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer feishuSrv.Close()

	settings.ResetForTest()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/notify-wf.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	settingStore := model.NewPlatformSettingModel(db)
	layered := settings.InitLayered(context.Background(), &settings.ConfigInput{}, settingStore)
	setStr := func(key, val string) {
		require.NoError(t, settingStore.Set(context.Background(), key, json.RawMessage(`"`+val+`"`), "test"))
		layered.Reload(context.Background(), settingStore)
	}
	setStr("notification.wecomUrl", wecomSrv.URL)
	setStr("notification.feishuUrl", feishuSrv.URL)
	setStr("notification.feishuSecret", "s")

	svc := notifyservice.New(layered, nil)
	svc.Dispatch(context.Background(), notifyservice.Event{
		Type: "approval_required", Title: "T", Message: "M",
	})

	require.NotEmpty(t, wecomBody, "wecom 应收到投递")
	assert.Contains(t, string(wecomBody), `"msgtype":"markdown"`)
	assert.Contains(t, string(wecomBody), "T")

	require.NotEmpty(t, feishuBody, "feishu 应收到投递")
	assert.Contains(t, string(feishuBody), `"msg_type":"text"`)
}
