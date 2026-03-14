// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type CertificateDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取证书详情
func NewCertificateDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateDetailLogic {
	return &CertificateDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateDetailLogic) CertificateDetail(req *types.CertificateDetailRequest) (*types.CertificateDetailResponse, error) {
	id, err := utils.ParseUintID(req.ID, "证书ID")
	if err != nil {
		return nil, err
	}

	cert, err := l.svcCtx.CertificateModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.CertificateDetailResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildCertificateDTO(cert),
	}, nil
}
