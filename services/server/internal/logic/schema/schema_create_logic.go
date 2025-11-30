// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package schema

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SchemaCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建模式
func NewSchemaCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SchemaCreateLogic {
	return &SchemaCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SchemaCreateLogic) SchemaCreate(req *types.SchemaCreateRequest) (*types.SchemaCreateResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("Schema 名称不能为空")
	}

	if err := validateSchemaDefinition(req.Schema); err != nil {
		return nil, err
	}

	doc := &schemaDocument{
		ID:        ensureUniqueSchemaID(l.svcCtx.Config, name),
		Name:      name,
		Schema:    req.Schema,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := saveSchema(l.svcCtx.Config, doc); err != nil {
		return nil, err
	}

	return &types.SchemaCreateResponse{
		Code:    0,
		Message: "OK",
		Data:    doc.toMap(),
	}, nil
}
