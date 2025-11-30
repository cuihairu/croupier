// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加证书
func NewCertificateAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateAddLogic {
	return &CertificateAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateAddLogic) CertificateAdd(req *types.CertificateAddRequest) (*types.CertificateAddResponse, error) {
	domain, err := utils.ValidateDomain(req.Domain)
	if err != nil {
		return nil, err
	}

	certPEM := strings.TrimSpace(req.Certificate)
	if certPEM == "" {
		return nil, errors.New("证书内容不能为空")
	}

	parsed, err := utils.ParseCertificatePEM(certPEM)
	if err != nil {
		return nil, err
	}

	certificate := &model.Certificate{
		Domain:         domain,
		CertificatePEM: certPEM,
		PrivateKeyPEM:  strings.TrimSpace(req.PrivateKey),
		Issuer:         utils.FormatIssuer(parsed),
		ExpiresAt:      parsed.NotAfter,
		Status:         model.CertificateStatus(parsed.NotAfter),
	}

	if err := l.svcCtx.CertificateModel.Create(l.ctx, certificate); err != nil {
		return nil, err
	}

	return &types.CertificateAddResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildCertificateDTO(certificate),
	}, nil
}
