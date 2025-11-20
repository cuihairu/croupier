package logic

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type MetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MetricsLogic {
	return &MetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MetricsLogic) Metrics() (string, error) {
	snap := l.svcCtx.MetricsSnapshot()
	var b strings.Builder
	fmt.Fprintf(&b, "# HELP croupier_uptime_seconds Time since server started\n")
	fmt.Fprintf(&b, "# TYPE croupier_uptime_seconds gauge\n")
	fmt.Fprintf(&b, "croupier_uptime_seconds %.0f\n", snap.UptimeSeconds)

	fmt.Fprintf(&b, "# HELP croupier_invocations_total Total number of function invocations\n")
	fmt.Fprintf(&b, "# TYPE croupier_invocations_total counter\n")
	fmt.Fprintf(&b, "croupier_invocations_total %d\n", snap.Invocations)

	fmt.Fprintf(&b, "# HELP croupier_invocations_error_total Total number of failed invocations\n")
	fmt.Fprintf(&b, "# TYPE croupier_invocations_error_total counter\n")
	fmt.Fprintf(&b, "croupier_invocations_error_total %d\n", snap.InvocationsError)

	fmt.Fprintf(&b, "# HELP croupier_jobs_started_total Total number of jobs started\n")
	fmt.Fprintf(&b, "# TYPE croupier_jobs_started_total counter\n")
	fmt.Fprintf(&b, "croupier_jobs_started_total %d\n", snap.JobsStarted)

	fmt.Fprintf(&b, "# HELP croupier_jobs_error_total Total number of job errors\n")
	fmt.Fprintf(&b, "# TYPE croupier_jobs_error_total counter\n")
	fmt.Fprintf(&b, "croupier_jobs_error_total %d\n", snap.JobsError)

	fmt.Fprintf(&b, "# HELP croupier_rbac_denied_total Total number of RBAC denials\n")
	fmt.Fprintf(&b, "# TYPE croupier_rbac_denied_total counter\n")
	fmt.Fprintf(&b, "croupier_rbac_denied_total %d\n", snap.RbacDenied)

	fmt.Fprintf(&b, "# HELP croupier_audit_errors_total Total number of audit errors\n")
	fmt.Fprintf(&b, "# TYPE croupier_audit_errors_total counter\n")
	fmt.Fprintf(&b, "croupier_audit_errors_total %d\n", snap.AuditErrors)

	return b.String(), nil
}
