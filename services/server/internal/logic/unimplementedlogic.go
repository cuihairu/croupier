package logic

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type unimplementedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func newUnimplementedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *unimplementedLogic {
	return &unimplementedLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *unimplementedLogic) notImplemented(feature string) (*types.GenericOkResponse, error) {
	return nil, fmt.Errorf("%s endpoint is not implemented yet", feature)
}

type AuditLogic struct {
	*unimplementedLogic
}

func NewAuditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuditLogic {
	return &AuditLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *AuditLogic) Audit() (*types.GenericOkResponse, error) {
	return l.notImplemented("Audit")
}

type CertificateAddLogic struct {
	*unimplementedLogic
}

func NewCertificateAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateAddLogic {
	return &CertificateAddLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *CertificateAddLogic) CertificateAdd() (*types.GenericOkResponse, error) {
	return l.notImplemented("CertificateAdd")
}

type CertificateAlertAddLogic struct {
	*unimplementedLogic
}

func NewCertificateAlertAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateAlertAddLogic {
	return &CertificateAlertAddLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *CertificateAlertAddLogic) CertificateAlertAdd() (*types.GenericOkResponse, error) {
	return l.notImplemented("CertificateAlertAdd")
}

type CertificateAlertsListLogic struct {
	*unimplementedLogic
}

func NewCertificateAlertsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateAlertsListLogic {
	return &CertificateAlertsListLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *CertificateAlertsListLogic) CertificateAlertsList() (*types.GenericOkResponse, error) {
	return l.notImplemented("CertificateAlertsList")
}

type CertificateCheckAllLogic struct {
	*unimplementedLogic
}

func NewCertificateCheckAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateCheckAllLogic {
	return &CertificateCheckAllLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *CertificateCheckAllLogic) CertificateCheckAll() (*types.GenericOkResponse, error) {
	return l.notImplemented("CertificateCheckAll")
}

type CertificateCheckLogic struct {
	*unimplementedLogic
}

func NewCertificateCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateCheckLogic {
	return &CertificateCheckLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *CertificateCheckLogic) CertificateCheck() (*types.GenericOkResponse, error) {
	return l.notImplemented("CertificateCheck")
}

type CertificateDeleteLogic struct {
	*unimplementedLogic
}

func NewCertificateDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateDeleteLogic {
	return &CertificateDeleteLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *CertificateDeleteLogic) CertificateDelete() (*types.GenericOkResponse, error) {
	return l.notImplemented("CertificateDelete")
}

type CertificateDomainInfoLogic struct {
	*unimplementedLogic
}

func NewCertificateDomainInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateDomainInfoLogic {
	return &CertificateDomainInfoLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *CertificateDomainInfoLogic) CertificateDomainInfo() (*types.GenericOkResponse, error) {
	return l.notImplemented("CertificateDomainInfo")
}

type CertificateExpiringLogic struct {
	*unimplementedLogic
}

func NewCertificateExpiringLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateExpiringLogic {
	return &CertificateExpiringLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *CertificateExpiringLogic) CertificateExpiring() (*types.GenericOkResponse, error) {
	return l.notImplemented("CertificateExpiring")
}

type CertificatesListLogic struct {
	*unimplementedLogic
}

func NewCertificatesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificatesListLogic {
	return &CertificatesListLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *CertificatesListLogic) CertificatesList() (*types.GenericOkResponse, error) {
	return l.notImplemented("CertificatesList")
}

type CertificateStatsLogic struct {
	*unimplementedLogic
}

func NewCertificateStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateStatsLogic {
	return &CertificateStatsLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *CertificateStatsLogic) CertificateStats() (*types.GenericOkResponse, error) {
	return l.notImplemented("CertificateStats")
}

type InvokeLogic struct {
	*unimplementedLogic
}

func NewInvokeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InvokeLogic {
	return &InvokeLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *InvokeLogic) Invoke() (*types.GenericOkResponse, error) {
	return l.notImplemented("Invoke")
}

type JobCancelLogic struct {
	*unimplementedLogic
}

func NewJobCancelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobCancelLogic {
	return &JobCancelLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *JobCancelLogic) JobCancel() (*types.GenericOkResponse, error) {
	return l.notImplemented("JobCancel")
}

type JobResultLogic struct {
	*unimplementedLogic
}

func NewJobResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobResultLogic {
	return &JobResultLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *JobResultLogic) JobResult() (*types.GenericOkResponse, error) {
	return l.notImplemented("JobResult")
}

type JobStartLogic struct {
	*unimplementedLogic
}

func NewJobStartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobStartLogic {
	return &JobStartLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *JobStartLogic) JobStart() (*types.GenericOkResponse, error) {
	return l.notImplemented("JobStart")
}

type MessageReadLogic struct {
	*unimplementedLogic
}

func NewMessageReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessageReadLogic {
	return &MessageReadLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *MessageReadLogic) MessageRead() (*types.GenericOkResponse, error) {
	return l.notImplemented("MessageRead")
}

type MessageSendLogic struct {
	*unimplementedLogic
}

func NewMessageSendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessageSendLogic {
	return &MessageSendLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *MessageSendLogic) MessageSend() (*types.GenericOkResponse, error) {
	return l.notImplemented("MessageSend")
}

type MessagesListLogic struct {
	*unimplementedLogic
}

func NewMessagesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessagesListLogic {
	return &MessagesListLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *MessagesListLogic) MessagesList() (*types.GenericOkResponse, error) {
	return l.notImplemented("MessagesList")
}

type MessagesUnreadCountLogic struct {
	*unimplementedLogic
}

func NewMessagesUnreadCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessagesUnreadCountLogic {
	return &MessagesUnreadCountLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *MessagesUnreadCountLogic) MessagesUnreadCount() (*types.GenericOkResponse, error) {
	return l.notImplemented("MessagesUnreadCount")
}

type RoleCreateLogic struct {
	*unimplementedLogic
}

func NewRoleCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleCreateLogic {
	return &RoleCreateLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *RoleCreateLogic) RoleCreate() (*types.GenericOkResponse, error) {
	return l.notImplemented("RoleCreate")
}

type RoleDeleteLogic struct {
	*unimplementedLogic
}

func NewRoleDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleDeleteLogic {
	return &RoleDeleteLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *RoleDeleteLogic) RoleDelete() (*types.GenericOkResponse, error) {
	return l.notImplemented("RoleDelete")
}

type RolePermissionsUpdateLogic struct {
	*unimplementedLogic
}

func NewRolePermissionsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RolePermissionsUpdateLogic {
	return &RolePermissionsUpdateLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *RolePermissionsUpdateLogic) RolePermissionsUpdate() (*types.GenericOkResponse, error) {
	return l.notImplemented("RolePermissionsUpdate")
}

type RolesListLogic struct {
	*unimplementedLogic
}

func NewRolesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RolesListLogic {
	return &RolesListLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *RolesListLogic) RolesList() (*types.GenericOkResponse, error) {
	return l.notImplemented("RolesList")
}

type RoleUpdateLogic struct {
	*unimplementedLogic
}

func NewRoleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleUpdateLogic {
	return &RoleUpdateLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *RoleUpdateLogic) RoleUpdate() (*types.GenericOkResponse, error) {
	return l.notImplemented("RoleUpdate")
}

type RootLogic struct {
	*unimplementedLogic
}

func NewRootLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RootLogic {
	return &RootLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *RootLogic) Root() (*types.GenericOkResponse, error) {
	return l.notImplemented("Root")
}

type SignedUrlLogic struct {
	*unimplementedLogic
}

func NewSignedUrlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignedUrlLogic {
	return &SignedUrlLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *SignedUrlLogic) SignedUrl() (*types.GenericOkResponse, error) {
	return l.notImplemented("SignedUrl")
}

type StreamJobLogic struct {
	*unimplementedLogic
}

func NewStreamJobLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StreamJobLogic {
	return &StreamJobLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *StreamJobLogic) StreamJob() (*types.GenericOkResponse, error) {
	return l.notImplemented("StreamJob")
}

type StreamMessagesLogic struct {
	*unimplementedLogic
}

func NewStreamMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StreamMessagesLogic {
	return &StreamMessagesLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *StreamMessagesLogic) StreamMessages() (*types.GenericOkResponse, error) {
	return l.notImplemented("StreamMessages")
}

type UserCreateLogic struct {
	*unimplementedLogic
}

func NewUserCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserCreateLogic {
	return &UserCreateLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *UserCreateLogic) UserCreate() (*types.GenericOkResponse, error) {
	return l.notImplemented("UserCreate")
}

type UserDeleteLogic struct {
	*unimplementedLogic
}

func NewUserDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserDeleteLogic {
	return &UserDeleteLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *UserDeleteLogic) UserDelete() (*types.GenericOkResponse, error) {
	return l.notImplemented("UserDelete")
}

type UserGameEnvsLogic struct {
	*unimplementedLogic
}

func NewUserGameEnvsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserGameEnvsLogic {
	return &UserGameEnvsLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *UserGameEnvsLogic) UserGameEnvs() (*types.GenericOkResponse, error) {
	return l.notImplemented("UserGameEnvs")
}

type UserGameEnvsUpdateLogic struct {
	*unimplementedLogic
}

func NewUserGameEnvsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserGameEnvsUpdateLogic {
	return &UserGameEnvsUpdateLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *UserGameEnvsUpdateLogic) UserGameEnvsUpdate() (*types.GenericOkResponse, error) {
	return l.notImplemented("UserGameEnvsUpdate")
}

type UserGamesLogic struct {
	*unimplementedLogic
}

func NewUserGamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserGamesLogic {
	return &UserGamesLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *UserGamesLogic) UserGames() (*types.GenericOkResponse, error) {
	return l.notImplemented("UserGames")
}

type UserGamesUpdateLogic struct {
	*unimplementedLogic
}

func NewUserGamesUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserGamesUpdateLogic {
	return &UserGamesUpdateLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *UserGamesUpdateLogic) UserGamesUpdate() (*types.GenericOkResponse, error) {
	return l.notImplemented("UserGamesUpdate")
}

type UserPasswordResetLogic struct {
	*unimplementedLogic
}

func NewUserPasswordResetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserPasswordResetLogic {
	return &UserPasswordResetLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *UserPasswordResetLogic) UserPasswordReset() (*types.GenericOkResponse, error) {
	return l.notImplemented("UserPasswordReset")
}

type UsersListLogic struct {
	*unimplementedLogic
}

func NewUsersListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UsersListLogic {
	return &UsersListLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *UsersListLogic) UsersList() (*types.GenericOkResponse, error) {
	return l.notImplemented("UsersList")
}

type UserUpdateLogic struct {
	*unimplementedLogic
}

func NewUserUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserUpdateLogic {
	return &UserUpdateLogic{
		unimplementedLogic: newUnimplementedLogic(ctx, svcCtx),
	}
}

func (l *UserUpdateLogic) UserUpdate() (*types.GenericOkResponse, error) {
	return l.notImplemented("UserUpdate")
}
