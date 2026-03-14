// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type CertificateDomainInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取域名证书信息
func NewCertificateDomainInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateDomainInfoLogic {
	return &CertificateDomainInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateDomainInfoLogic) CertificateDomainInfo(req *types.CertificateDomainInfoRequest) (*types.CertificateDomainInfoResponse, error) {
	domain, err := utils.ValidateDomain(req.Domain)
	if err != nil {
		return nil, err
	}

	cert, err := l.svcCtx.CertificateModel.FindByDomain(l.ctx, domain)
	if err != nil {
		return nil, err
	}

	return &types.CertificateDomainInfoResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildCertificateDTO(cert),
	}, nil
}
