// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificatesListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取证书列表
func NewCertificatesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificatesListLogic {
	return &CertificatesListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificatesListLogic) CertificatesList(req *types.CertificatesListRequest) (*types.CertificatesListResponse, error) {
	opts := model.ListCertificatesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Status:   strings.TrimSpace(req.Status),
	}

	certs, total, err := l.svcCtx.CertificateModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(certs))
	for i := range certs {
		items = append(items, utils.BuildCertificateDTO(&certs[i]))
	}

	return &types.CertificatesListResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"total": total,
			"page":  opts.Page,
			"size":  opts.PageSize,
		},
	}, nil
}
