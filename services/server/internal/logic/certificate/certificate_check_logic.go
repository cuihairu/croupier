// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type CertificateCheckLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 检查证书状态
func NewCertificateCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateCheckLogic {
	return &CertificateCheckLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateCheckLogic) CertificateCheck(req *types.CertificateCheckRequest) (*types.CertificateCheckResponse, error) {
	id, err := utils.ParseUintID(req.ID, "证书ID")
	if err != nil {
		return nil, err
	}

	cert, err := l.svcCtx.CertificateModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	parsed, err := utils.ParseCertificatePEM(cert.CertificatePEM)
	if err != nil {
		return nil, err
	}

	cert.ExpiresAt = parsed.NotAfter
	cert.Issuer = utils.FormatIssuer(parsed)
	utils.UpdateCertificateStatus(cert)

	if err := l.svcCtx.CertificateModel.Update(l.ctx, cert.ID, map[string]interface{}{
		"expires_at":      cert.ExpiresAt,
		"issuer":          cert.Issuer,
		"status":          cert.Status,
		"last_checked_at": cert.LastCheckedAt,
		"error_message":   cert.ErrorMessage,
	}); err != nil {
		return nil, err
	}

	return &types.CertificateCheckResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildCertificateDTO(cert),
	}, nil
}
