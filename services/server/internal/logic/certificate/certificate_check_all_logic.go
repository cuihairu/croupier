// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type CertificateCheckAllLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 检查所有证书
func NewCertificateCheckAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateCheckAllLogic {
	return &CertificateCheckAllLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateCheckAllLogic) CertificateCheckAll(req *types.CertificateCheckAllRequest) (*types.CertificateCheckAllResponse, error) {
	certs, err := l.svcCtx.CertificateModel.ListAll(l.ctx)
	if err != nil {
		return nil, err
	}

	var success int
	var failed int
	for i := range certs {
		cert := &certs[i]
		parsed, parseErr := utils.ParseCertificatePEM(cert.CertificatePEM)
		if parseErr != nil {
			cert.ErrorMessage = parseErr.Error()
			cert.Status = "invalid"
			failed++
		} else {
			cert.ExpiresAt = parsed.NotAfter
			cert.Issuer = utils.FormatIssuer(parsed)
			utils.UpdateCertificateStatus(cert)
			success++
		}
		_ = l.svcCtx.CertificateModel.Update(l.ctx, cert.ID, map[string]interface{}{
			"expires_at":      cert.ExpiresAt,
			"issuer":          cert.Issuer,
			"status":          cert.Status,
			"last_checked_at": cert.LastCheckedAt,
			"error_message":   cert.ErrorMessage,
		})
	}

	return &types.CertificateCheckAllResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"checked": success,
			"failed":  failed,
			"total":   len(certs),
		},
	}, nil
}
