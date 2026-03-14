// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type CertificateAlertAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加证书告警
func NewCertificateAlertAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateAlertAddLogic {
	return &CertificateAlertAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateAlertAddLogic) CertificateAlertAdd(req *types.CertificateAlertAddRequest) (*types.CertificateAlertAddResponse, error) {
	domain, err := utils.ValidateDomain(req.Domain)
	if err != nil {
		return nil, err
	}

	if _, err := l.svcCtx.CertificateModel.FindByDomain(l.ctx, domain); err != nil {
		return nil, err
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 30
	}

	alert := &model.CertificateAlert{
		Domain:        domain,
		ThresholdDays: threshold,
		Active:        true,
	}

	if err := l.svcCtx.CertificateModel.AddAlert(l.ctx, alert); err != nil {
		return nil, err
	}

	return &types.CertificateAlertAddResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id":            alert.ID,
			"domain":        alert.Domain,
			"thresholdDays": alert.ThresholdDays,
		},
	}, nil
}
