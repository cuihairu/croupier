package ops

// 覆盖率巡检第十轮（wt-api）：api/ops 残余 26 块收口——
// probe.go 9 块（wecom/feishu 渠道、SMTP 已配置探活主链）、performance.go
// 5 块（绑定错误双翼含 jsonNumber 非数值解错、store 写失败、StartTime
// 可达分支）、logs.go 12 块（nil 守卫×2、写失败、绑定错误×2、PurgeBefore
// 与直清三表错误翼、nil 快照早退、轮转目录不可读降级）。
// sitesettings 有他会话 v10 在途文件（06:14 落盘）已回避；本文件只碰
// ops 域，与全部在途 ?? 文件目录不相交。

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// opsReadonlyDB 连接级只读（PRAGMA 作用于单连接，锁池大小 1 复用同一连接）。
func opsReadonlyDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA query_only = ON").Error)
}

// ---------- probe.go：wecom / feishu 渠道 + SMTP 已配置主链 ----------

func TestThirdPartyProbe_WecomFeishuChannels(t *testing.T) {
	r := setupProbeRouter(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	for _, tc := range []struct{ channel, key string }{
		{"wecom", settings.KeyNotifyWecomURL},
		{"feishu", settings.KeyNotifyFeishuURL},
	} {
		seedProbeSetting(t, tc.key, srv.URL)
		code, body := postProbe(r, tc.channel)
		require.Equal(t, http.StatusOK, code, tc.channel)
		assert.Contains(t, body, `"configured":true`, tc.channel)
		assert.Contains(t, body, `"ok":true`, tc.channel)
		assert.Contains(t, body, `"status":200,`, tc.channel)
	}
}

// TestThirdPartyProbe_SMTPConfigured SMTP 已配置时走 TCP+EHLO 探活主链。
// ProbeSMTP 的 Hello 阶段会阻塞等待 220 问候语（连接无读超时），假目标
// 必须是真 SMTP 形态的服务器——HTTP 假服务器一句话不发会把用例挂死
// （首轮实证：600s 包级超时）。此处回环裸监听，问候 + 逐行 250 应答。
func TestThirdPartyProbe_SMTPConfigured(t *testing.T) {
	r := setupProbeRouter(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(3 * time.Second))
				_, _ = fmt.Fprint(c, "220 fake ESMTP ready\r\n")
				sc := bufio.NewScanner(c)
				for sc.Scan() {
					_, _ = fmt.Fprint(c, "250 ok\r\n")
				}
			}(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })

	seedProbeSetting(t, settings.KeyNotifySMTPHost, "127.0.0.1")
	// int 键经字符串形态写入（GetInt 的字符串回退解析）
	seedProbeSetting(t, settings.KeyNotifySMTPPort, strconv.Itoa(ln.Addr().(*net.TCPAddr).Port))

	code, body := postProbe(r, "smtp")
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `"configured":true`)
	assert.Contains(t, body, `"channel":"smtp"`)
	assert.Contains(t, body, `"ok":true`, "220 问候 + EHLO/NOOP 的 250 应答链探活成功")
}

// ---------- performance.go：绑定/解错/写失败/StartTime ----------

func TestPerformancePut_BindAndValueErrors(t *testing.T) {
	initPerfSettings(t)
	r, _ := setupPerformanceRouter(t)

	cases := []struct {
		name string
		body string
	}{
		// 值非数值：jsonNumber.UnmarshalJSON 解错（float unmarshal 失败翼）
		{"string_value", `{"perf.maxCpuPct":"abc"}`},
		// 请求体整体非法 JSON（ShouldBindJSON 直接失败）
		{"truncated", `{"perf.maxCpuPct":`},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/ops/performance", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, tc.name)
	}
}

// TestPerformanceUpdate_StoreWriteBlocked 校验全过、L3 写入被拒的 500 翼。
func TestPerformanceUpdate_StoreWriteBlocked(t *testing.T) {
	initPerfSettings(t)
	db := newPerfDB(t)
	opsReadonlyDB(t, db)
	svcCtx := &svc.ServiceContext{PlatformSettingModel: model.NewPlatformSettingModel(db)}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(NewService(svcCtx))
	r.PUT("/ops/performance", func(c *gin.Context) {
		c.Set("username", "tester")
		h.PerformancePut(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/ops/performance", strings.NewReader(`{"perf.maxCpuPct":50}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestBuildPerformanceSnapshot_StartTimeReachable svcCtx.StartTime 可达时
// 在线时长与其同源（而非包级 perfStartedAt 兜底）。
func TestBuildPerformanceSnapshot_StartTimeReachable(t *testing.T) {
	initPerfSettings(t)
	started := time.Now().Add(-90 * time.Second)
	snap := buildPerformanceSnapshot(&svc.ServiceContext{StartTime: started})
	assert.GreaterOrEqual(t, snap.Runtime.UptimeSeconds, int64(89))
	assert.LessOrEqual(t, snap.Runtime.UptimeSeconds, int64(92))
}

// ---------- logs.go：守卫 / 写失败 / 绑定 / 清理错误翼 / 快照降级 ----------

func TestLogsPut_NilStore_WriteBlocked_BindError(t *testing.T) {
	initLogsSettings(t)

	// svcCtx 存在但 settings store 未接线 → 守卫 500
	r, _ := setupLogsRouter(t, func(s *svc.ServiceContext) { s.PlatformSettingModel = nil })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/ops/logs", strings.NewReader(`{"log.retentionDays":14}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// store 在、连接拒写（校验全过、Set 被拒）
	r2, svcCtx2 := setupLogsRouter(t, nil)
	opsReadonlyDB(t, svcCtx2.DB)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/ops/logs", strings.NewReader(`{"log.retentionDays":14}`))
	req.Header.Set("Content-Type", "application/json")
	r2.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// 请求体整体非法 JSON
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/ops/logs", strings.NewReader(`{"log.retentionDays":`))
	req.Header.Set("Content-Type", "application/json")
	r2.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogsCleanupPost_BindError(t *testing.T) {
	initLogsSettings(t)
	r, _ := setupLogsRouter(t, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ops/logs/cleanup", strings.NewReader(`{"scope":`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestLogsCleanup_NilServiceContext scope/beforeHours 校验在守卫之前——
// 合法请求 + nil service 才触达「service context unavailable」。
func TestLogsCleanup_NilServiceContext(t *testing.T) {
	_, err := NewService(nil).LogsCleanup(context.Background(), LogsCleanupRequest{Scope: "all", BeforeHours: 24})
	require.ErrorContains(t, err, "service context unavailable")
}

// TestLogsCleanup_ErrorWings 清理链四条错误翼：fanout PurgeBefore 失败、
// 回退态 DB 缺失、直清三表逐一缺表。
func TestLogsCleanup_ErrorWings(t *testing.T) {
	initLogsSettings(t)

	post := func(r *gin.Engine, body string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/ops/logs/cleanup", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}

	t.Run("fanout_purge_error", func(t *testing.T) {
		// 无表裸库上的 Retention：PurgeBefore 直清缺表报错
		raw, err := gorm.Open(gsqlite.Open(t.TempDir()+"/empty_retention.db"), &gorm.Config{})
		require.NoError(t, err)
		r, _ := setupLogsRouter(t, func(s *svc.ServiceContext) {
			s.LogRetention = executionlog.NewRetention(raw, executionlog.RetentionConfig{})
		})
		assert.Equal(t, http.StatusInternalServerError, post(r, `{"scope":"all","beforeHours":24}`))
	})

	t.Run("fallback_db_nil", func(t *testing.T) {
		r, _ := setupLogsRouter(t, func(s *svc.ServiceContext) { s.DB = nil })
		assert.Equal(t, http.StatusInternalServerError, post(r, `{"scope":"all","beforeHours":24}`))
	})

	t.Run("direct_execution_table_missing", func(t *testing.T) {
		r, _ := setupLogsRouter(t, func(s *svc.ServiceContext) {
			require.NoError(t, s.DB.Migrator().DropTable("execution_logs"))
		})
		assert.Equal(t, http.StatusInternalServerError, post(r, `{"scope":"execution","beforeHours":24}`))
	})

	t.Run("direct_task_runs_missing", func(t *testing.T) {
		r, _ := setupLogsRouter(t, func(s *svc.ServiceContext) {
			require.NoError(t, s.DB.Migrator().DropTable("task_runs"))
		})
		assert.Equal(t, http.StatusInternalServerError, post(r, `{"scope":"task","beforeHours":24}`))
	})

	t.Run("direct_task_events_missing", func(t *testing.T) {
		// task_runs 完好、events 缺表：先成功删 runs 再在 events 报错
		r, _ := setupLogsRouter(t, func(s *svc.ServiceContext) {
			require.NoError(t, s.DB.Migrator().DropTable("task_events"))
		})
		assert.Equal(t, http.StatusInternalServerError, post(r, `{"scope":"task","beforeHours":24}`))
	})
}

// TestBuildLogsSnapshot_NilContext svcCtx 为 nil：server log 视图走
// stdout 早退、三表体量降级 Rows=-1，快照不 panic。
func TestBuildLogsSnapshot_NilContext(t *testing.T) {
	initLogsSettings(t)
	snap := buildLogsSnapshot(nil)
	assert.Equal(t, "stdout", snap.ServerLog.Output)
	assert.Empty(t, snap.ServerLog.File)
	require.Len(t, snap.Tables, 3)
	for _, tv := range snap.Tables {
		assert.Equal(t, int64(-1), tv.Rows)
		assert.Empty(t, tv.OldestAt)
	}
}

// TestBuildServerLogView_UnreadableDirectory 日志目录不存在：轮转计数
// 按读目录失败降级 0（当前文件也计不进），不阻塞视图。
func TestBuildServerLogView_UnreadableDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone", "croupier.log")
	svcCtx := &svc.ServiceContext{Config: config.Config{}}
	svcCtx.Config.Logging.Output = "file"
	svcCtx.Config.Logging.File = missing

	view := buildServerLogView(svcCtx)
	assert.Equal(t, "file", view.Output)
	assert.Equal(t, missing, view.File)
	assert.Equal(t, filepath.Dir(missing), view.Directory)
	assert.Zero(t, view.FileCount, "读目录失败降级为 0")
}
