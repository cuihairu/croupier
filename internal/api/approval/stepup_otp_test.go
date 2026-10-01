package approval

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/security/otp"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// stepupOTPFixture：sqlite 内存库 + 已绑定/未绑定 TOTP 的两名管理员 +
// fn.high / fn.danger 两行高危契约（step-up 门槛靠 function.ResolvePolicyRisk
// 的契约档位判定）。
type stepupOTPFixture struct {
	svc      *Service
	store    *approvals.MemStore
	auditSvc *audit.AuditService
	// operatorSecrets 登记已绑定 TOTP 操作人的 secret；缺省即未绑定。
	operatorSecrets map[string]string
}

func newStepUpFixture(t *testing.T) *stepupOTPFixture {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// Approve 成功路径会续查管理员角色（admin_roles/roles），按同包先例全量迁移
	require.NoError(t, model.AutoMigrate(db))

	enrolledSecret, err := otp.GenerateSecret()
	require.NoError(t, err)
	admins := []model.Admin{
		{Username: "enrolled", PasswordHash: "x", OTPSecret: enrolledSecret, OTPEnabled: true},
		{Username: "plain", PasswordHash: "x"},
	}
	for i := range admins {
		require.NoError(t, db.Create(&admins[i]).Error)
	}
	contracts := []model.FunctionContract{
		{GameID: "game-1", Env: "prod", FunctionID: "fn.high", Version: "1.0.0", Risk: dbenum.RiskHigh},
		{GameID: "game-1", Env: "prod", FunctionID: "fn.danger", Version: "1.0.0", Risk: dbenum.RiskDanger},
	}
	for i := range contracts {
		require.NoError(t, db.Create(&contracts[i]).Error)
	}

	auditSvc := audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)
	store := approvals.NewMemStore()
	// Dispatcher 必须非 nil：批准成功路径会续跑 FunctionInvoke，nil 直接 panic；
	// 空 registry 使续跑报 no live agent —— 见成功用例对 continuation failed 终态的处理。
	regStore := reg.NewStore()
	return &stepupOTPFixture{
		svc: NewService(&svc.ServiceContext{
			DB:             db,
			RegistryStore:  regStore,
			Dispatcher:     dispatch.NewDispatcher(regStore),
			ApprovalsStore: store,
			AuditService:   auditSvc,
			AdminModel:     model.NewAdminModel(db),
		}),
		store:           store,
		auditSvc:        auditSvc,
		operatorSecrets: map[string]string{"enrolled": enrolledSecret},
	}
}

// seedPending 造一条待审批记录；functionID 传空即低危档（无契约行 → medium）。
func (f *stepupOTPFixture) seedPending(t *testing.T, id, functionID string) {
	t.Helper()
	_, err := f.store.Create(&approvals.Approval{
		ID: id, State: "pending", FunctionID: functionID,
		GameID: "game-1", Env: "prod", Actor: "requester",
	})
	require.NoError(t, err)
}

// validCode 生成指定操作人当前有效的 TOTP 验证码。
func (f *stepupOTPFixture) validCode(t *testing.T, operator string) string {
	t.Helper()
	secret, ok := f.operatorSecrets[operator]
	require.True(t, ok, "fixture 未登记 %s 的 secret", operator)
	code, ok := otp.CodeAt(secret, time.Now())
	require.True(t, ok)
	return code
}

func stepUpErrorCode(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)
	var ce *errorx.CodeError
	require.True(t, errors.As(err, &ce), "期望 errorx.CodeError，得到 %T", err)
	return ce.ErrorCode()
}

func (f *stepupOTPFixture) stepUpFailedOutcomes(t *testing.T) []interface{} {
	t.Helper()
	items, _, err := f.auditSvc.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventApprovalStepUpFailed},
	}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	outcomes := make([]interface{}, 0, len(items))
	for _, it := range items {
		outcomes = append(outcomes, it.Details["outcome"])
	}
	return outcomes
}

func (f *stepupOTPFixture) requireStillPending(t *testing.T, id string) {
	t.Helper()
	rec, err := f.store.Get(id)
	require.NoError(t, err)
	assert.Equal(t, "pending", rec.State, "step-up 校验不过时审批必须保持 pending")
}

func TestStepUp_HighRisk_CorrectOTP_Passes(t *testing.T) {
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-otp-1", "fn.high")

	// 正确 otp → 批准态落库、step-up 审计先于续跑写入。fixture 无在线
	// agent，续跑以 approved-but-continuation-failed 收场（同包先例
	// TestApprove_FreshPageSnapshotContinuationFailureRecordsReason），
	// 不回滚批准态——这里只锁 step-up 门通过后的批准与审计行为。
	_, _ = f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{
		ID: "ap-otp-1", OTP: f.validCode(t, "enrolled"),
	})
	stored, getErr := f.store.Get("ap-otp-1")
	require.NoError(t, getErr)
	assert.Equal(t, "approved", stored.State)

	// 批准审计行带 stepUp=totp，且绝不出现 otp 值（含大小写键名）
	items, total, err := f.auditSvc.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventApprovalApproved},
	}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, stepUpResultTOTP, items[0].Details["stepUp"])
	for _, it := range items {
		assert.NotContains(t, it.Details, "otp")
		assert.NotContains(t, it.Details, "OTP")
	}
}

func TestStepUp_HighRisk_MissingOTP_Rejected(t *testing.T) {
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-otp-2", "fn.high")

	res, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{ID: "ap-otp-2"})
	assert.Nil(t, res)
	assert.Equal(t, "otp_required", stepUpErrorCode(t, err))
	f.requireStillPending(t, "ap-otp-2")
	assert.Equal(t, []interface{}{"otp_required"}, f.stepUpFailedOutcomes(t))
}

func TestStepUp_HighRisk_WrongOTP_Rejected(t *testing.T) {
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-otp-3", "fn.high")

	res, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{
		ID: "ap-otp-3", OTP: "000000",
	})
	assert.Nil(t, res)
	assert.Equal(t, "otp_invalid", stepUpErrorCode(t, err))
	f.requireStillPending(t, "ap-otp-3")
	assert.Equal(t, []interface{}{"otp_invalid"}, f.stepUpFailedOutcomes(t))
}

func TestStepUp_HighRisk_NotEnrolled_Rejected(t *testing.T) {
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-otp-4", "fn.high")

	res, err := f.svc.Approve(approvalCtx("plain"), &ApprovalApproveRequest{
		ID: "ap-otp-4", OTP: "123456",
	})
	assert.Nil(t, res)
	assert.Equal(t, "otp_not_enrolled", stepUpErrorCode(t, err))
	f.requireStillPending(t, "ap-otp-4")
	assert.Equal(t, []interface{}{"not_enrolled"}, f.stepUpFailedOutcomes(t))
}

func TestStepUp_HighRisk_DangerTier_AlsoGated(t *testing.T) {
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-otp-5", "fn.danger")

	// danger 档同样在门槛内：未带 otp → otp_required
	_, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{ID: "ap-otp-5"})
	assert.Equal(t, "otp_required", stepUpErrorCode(t, err))
}

func TestStepUp_LowRisk_NoOTP_Passes(t *testing.T) {
	f := newStepUpFixture(t)
	// 无契约行 → medium 档：不带 otp 直接放行
	f.seedPending(t, "ap-low-1", "")

	res, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{ID: "ap-low-1"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "approved", res.State)

	items, total, err := f.auditSvc.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventApprovalApproved},
	}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, stepUpResultNone, items[0].Details["stepUp"])
}

func TestStepUp_LowRisk_ProvidedOTP_IsVerified(t *testing.T) {
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-low-2", "")

	// 低危档带了 otp 就要校验：错的拒绝（防误填静默通过）
	_, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{ID: "ap-low-2", OTP: "999999"})
	assert.Equal(t, "otp_invalid", stepUpErrorCode(t, err))
	f.requireStillPending(t, "ap-low-2")

	// 对的放行
	res, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{
		ID: "ap-low-2", OTP: f.validCode(t, "enrolled"),
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "approved", res.State)
}

func TestStepUp_LowRisk_UnknownOperatorProvidedOTP_Rejected(t *testing.T) {
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-low-3", "")

	// 操作人查不到（fail-closed 按未绑定处理）：带 otp 必然 otp_invalid
	_, err := f.svc.Approve(approvalCtx("ghost"), &ApprovalApproveRequest{ID: "ap-low-3", OTP: "123456"})
	assert.Equal(t, "otp_invalid", stepUpErrorCode(t, err))
}

func TestStepUp_OverlongOTP_NoPanic_Rejected(t *testing.T) {
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-low-4", "")

	long := make([]byte, 4096)
	for i := range long {
		long[i] = '7'
	}
	_, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{ID: "ap-low-4", OTP: string(long)})
	assert.Equal(t, "otp_invalid", stepUpErrorCode(t, err))
}

// handler 层：空 body 必须容忍（Web 无 otp 时 data 为 undefined 不发 body，
// 回归现网形态），有 body 且解不出 JSON 仍按契约 400。
func TestHandler_Approve_EmptyBody_NotBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-empty-1", "")
	handler := NewHandler(f.svc)
	router := gin.New()
	router.POST("/approvals/:id/approve", handler.Approve)

	req := httptest.NewRequest("POST", "/approvals/ap-empty-1/approve", nil)
	req = req.WithContext(context.WithValue(req.Context(), "username", "enrolled"))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Approve_WithJSONBody_BindOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-body-1", "")
	handler := NewHandler(f.svc)
	router := gin.New()
	router.POST("/approvals/:id/approve", handler.Approve)

	body := `{"otp":"` + f.validCode(t, "enrolled") + `"}`
	req := httptest.NewRequest("POST", "/approvals/ap-body-1/approve", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), "username", "enrolled"))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Approve_InvalidJSONBody_BadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-bad-1", "")
	handler := NewHandler(f.svc)
	router := gin.New()
	router.POST("/approvals/:id/approve", handler.Approve)

	req := httptest.NewRequest("POST", "/approvals/ap-bad-1/approve", bytes.NewBufferString("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), "username", "enrolled"))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}
