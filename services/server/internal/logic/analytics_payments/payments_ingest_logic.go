// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_payments

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type PaymentsIngestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 采集支付数据
func NewPaymentsIngestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentsIngestLogic {
	return &PaymentsIngestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PaymentsIngestLogic) PaymentsIngest(req *types.PaymentsIngestRequest) (*types.PaymentsIngestResponse, error) {
	if l.svcCtx.PaymentsModel == nil {
		return nil, errors.New("payments model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}
	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errors.New("gameId 不能为空")
	}

	env := strings.TrimSpace(req.Env)

	rawEntries, err := decodeTransactionsPayload(req.Transactions)
	if err != nil {
		return nil, err
	}

	var accepted, rejected int
	for _, entry := range rawEntries {
		tx, buildErr := buildTransaction(entry, gameID, env)
		if buildErr != nil {
			rejected++
			continue
		}
		if err := l.svcCtx.PaymentsModel.CreateTransaction(l.ctx, tx); err != nil {
			rejected++
			continue
		}
		accepted++
	}

	return &types.PaymentsIngestResponse{
		Accepted: accepted,
		Rejected: rejected,
		BatchId:  uuid.NewString(),
	}, nil
}
