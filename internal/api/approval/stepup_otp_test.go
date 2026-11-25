package approval

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/cuihairu/croupier/internal/platform/settings"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/security/otp"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// stepupOTPFixture: an in-memory sqlite database plus two admins, one with
// TOTP bound and one without, plus
// two high-risk contracts fn.high / fn.danger (the step-up gate relies on the
// contract tier judgement of function.ResolvePolicyRisk).
type stepupOTPFixture struct {
	svc      *Service
	store    *approvals.MemStore
	auditSvc *audit.AuditService
	// operatorSecrets registers the secret of each TOTP-bound operator;
	// absent means unbound.
	operatorSecrets map[string]string
}

func newStepUpFixture(t *testing.T) *stepupOTPFixture {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// The successful Approve path goes on to look up admin roles
	// (admin_roles/roles), so follow the same-package precedent and migrate
	// everything
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
	// Dispatcher must not be nil: the successful approve path continues with
	// FunctionInvoke, and nil panics outright;
	// an empty registry makes the continuation report no live agent — see how
	// the success case handles the continuation-failed terminal state.
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

// seedPending creates a pending approval record; passing an empty functionID
// gives the low-risk tier (no contract row → medium).
func (f *stepupOTPFixture) seedPending(t *testing.T, id, functionID string) {
	t.Helper()
	_, err := f.store.Create(&approvals.Approval{
		ID: id, State: "pending", FunctionID: functionID,
		GameID: "game-1", Env: "prod", Actor: "requester",
	})
	require.NoError(t, err)
}

// validCode generates the currently valid TOTP code for the given operator.
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

// R30-02 (ARCH-025): when the contract risk source fails, the step-up gate
// fails closed and treats the risk as high
// (requiring second verification); a read failure must never silently downgrade
// the tier to medium/low risk and relax it.
func TestApproveStepUpRisk_SourceFailureFailsClosed(t *testing.T) {
	// An empty in-memory database has no function_contracts table → the
	// contract risk query reports "no such table"
	// (a source failure, not ErrRecordNotFound).
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	svcCtx := &svc.ServiceContext{DB: db}

	risk := approveStepUpRisk(context.Background(), svcCtx, &approvals.Approval{
		FunctionID: "fn.high", GameID: "game-1", Env: "prod",
	})
	assert.Equal(t, policymgr.RiskHigh, risk, "风险来源故障不得放宽 step-up 门槛")

	// An undeclared risk (no contract row → medium) still does not trigger the
	// high-risk gate; this is not a source failure
	goodDB, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(goodDB))
	svcCtx2 := &svc.ServiceContext{DB: goodDB}
	risk = approveStepUpRisk(context.Background(), svcCtx2, &approvals.Approval{
		FunctionID: "fn.unseen", GameID: "game-1", Env: "prod",
	})
	assert.Equal(t, policymgr.RiskMedium, risk, "未声明风险应回落 medium")
}

func TestStepUp_HighRisk_CorrectOTP_Passes(t *testing.T) {
	f := newStepUpFixture(t)
	f.seedPending(t, "ap-otp-1", "fn.high")

	// A correct otp → the approved state lands and the step-up audit is written
	// before the continuation. The fixture has no online
	// agent, so the continuation ends up approved-but-continuation-failed (same
	// precedent as TestApprove_FreshPageSnapshotContinuationFailureRecordsReason
	// in this package),
	// without rolling back the approved state — this case only pins the
	// approval and audit behavior once the step-up gate is passed.
	_, _ = f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{
		ID: "ap-otp-1", OTP: f.validCode(t, "enrolled"),
	})
	stored, getErr := f.store.Get("ap-otp-1")
	require.NoError(t, getErr)
	assert.Equal(t, "approved", stored.State)

	// The approval audit row carries stepUp=totp and never exposes the otp value
	// itself (including case-variant key names)
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

	// The danger tier is likewise inside the gate: no otp → otp_required
	_, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{ID: "ap-otp-5"})
	assert.Equal(t, "otp_required", stepUpErrorCode(t, err))
}

func TestStepUp_LowRisk_NoOTP_Passes(t *testing.T) {
	f := newStepUpFixture(t)
	// No contract row → the medium tier: let it through with no otp
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

	// In the low-risk tier a supplied otp must be validated: a wrong one is
	// rejected (preventing a typo from passing silently)
	_, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{ID: "ap-low-2", OTP: "999999"})
	assert.Equal(t, "otp_invalid", stepUpErrorCode(t, err))
	f.requireStillPending(t, "ap-low-2")

	// The right one passes
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

	// The operator cannot be found (fail closed, treated as unbound): supplying
	// an otp always yields otp_invalid
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

// Handler layer: an empty body must be tolerated (when Web has no otp the data
// is undefined and no body is sent,
// which is the shape seen in production), while a present body that does not
// parse as JSON still yields 400 per contract.
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

// withStepUpSwitch initializes the settings singleton and sets
// security.approvalStepUpOtp to the given value (the #61 master switch); it
// restores the default (clear the key + Reload) when the test ends so it does
// not pollute the other cases in this package — settings.Current() is a
// sync.Once singleton shared by every case in this package.
func withStepUpSwitch(t *testing.T, f *stepupOTPFixture, raw string) {
	t.Helper()
	settingModel := model.NewPlatformSettingModel(f.svc.svcCtx.DB)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, settingModel)
	l := settings.Current()
	require.NotNil(t, l)
	require.NoError(t, settingModel.Set(context.Background(),
		settings.KeySecurityApprovalStepUpOTP, json.RawMessage(raw), "tester"))
	l.Reload(context.Background(), settingModel)
	t.Cleanup(func() {
		_ = settingModel.Clear(context.Background(), settings.KeySecurityApprovalStepUpOTP)
		l.Reload(context.Background(), settingModel)
	})
}

// The #61 master switch is off: a high-risk approval without an otp is let
// through, and the approval audit records the disabled bypass tier
// (as opposed to not_required's "not needed at all" — after the fact one can
// audit which high-risk operations the switch let through).
// The fixture has no online agent, so the continuation ends up
// approved-but-continuation-failed (same precedent as
// TestStepUp_HighRisk_CorrectOTP_Passes in this package); this case only pins
// the approved state and the stepUp audit.
func TestStepUp_SwitchOff_HighRisk_NoOTP_PassesWithDisabledAudit(t *testing.T) {
	f := newStepUpFixture(t)
	withStepUpSwitch(t, f, `false`)
	f.seedPending(t, "ap-sw-1", "fn.high")

	_, _ = f.svc.Approve(approvalCtx("plain"), &ApprovalApproveRequest{ID: "ap-sw-1"})
	stored, getErr := f.store.Get("ap-sw-1")
	require.NoError(t, getErr)
	assert.Equal(t, "approved", stored.State)

	items, total, err := f.auditSvc.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventApprovalApproved},
	}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, stepUpResultDisabled, items[0].Details["stepUp"])
}

// Switch off does not mean exemption from validation: a supplied otp is still
// validated with medium/low tier semantics, and a wrong one is rejected.
func TestStepUp_SwitchOff_HighRisk_WrongOTP_Rejected(t *testing.T) {
	f := newStepUpFixture(t)
	withStepUpSwitch(t, f, `false`)
	f.seedPending(t, "ap-sw-2", "fn.high")

	_, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{
		ID: "ap-sw-2", OTP: "000000",
	})
	assert.Equal(t, "otp_invalid", stepUpErrorCode(t, err))
	f.requireStillPending(t, "ap-sw-2")

	// The right one passes and the audit records totp (deliberate second
	// verification is recorded faithfully even with the switch off); continuation
	// errors are tolerated as above, with the approved state taking precedence
	_, _ = f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{
		ID: "ap-sw-2", OTP: f.validCode(t, "enrolled"),
	})
	stored, getErr := f.store.Get("ap-sw-2")
	require.NoError(t, getErr)
	assert.Equal(t, "approved", stored.State)

	items, total, err := f.auditSvc.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventApprovalApproved},
	}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	assert.Equal(t, stepUpResultTOTP, items[0].Details["stepUp"])
}

// An explicit true for the switch has the same semantics as an unset one (the
// existing high-risk cases, which run with settings nil, already pin
// the fail-safe default; this adds an explicit true case as a regression guard).
func TestStepUp_SwitchOn_HighRisk_StillEnforced(t *testing.T) {
	f := newStepUpFixture(t)
	withStepUpSwitch(t, f, `true`)
	f.seedPending(t, "ap-sw-3", "fn.high")

	_, err := f.svc.Approve(approvalCtx("enrolled"), &ApprovalApproveRequest{ID: "ap-sw-3"})
	assert.Equal(t, "otp_required", stepUpErrorCode(t, err))
	f.requireStillPending(t, "ap-sw-3")
}
