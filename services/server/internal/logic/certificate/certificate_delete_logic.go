// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type CertificateDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除证书
func NewCertificateDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateDeleteLogic {
	return &CertificateDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateDeleteLogic) CertificateDelete(req *types.CertificateDeleteRequest) (*types.CertificateDeleteResponse, error) {
	id, err := utils.ParseUintID(req.ID, "证书ID")
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.CertificateModel.Delete(l.ctx, id); err != nil {
		return nil, err
	}

	return &types.CertificateDeleteResponse{
		Code:    0,
		Message: "OK",
	}, nil
}
