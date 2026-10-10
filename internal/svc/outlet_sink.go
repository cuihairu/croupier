package svc

import (
	"context"
	"encoding/json"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"gorm.io/gorm"
)

// messageSink 把 outlet.StationNotice 落 messages 表（站内出口链首的
// 持久化面；outlet 包只依赖 MessageSink 接口，不反向依赖 model）。
type messageSink struct {
	db *gorm.DB
}

// Create implements outlet.MessageSink.
func (s messageSink) Create(ctx context.Context, n outlet.StationNotice) error {
	scopeJSON, err := json.Marshal(n.Scope)
	if err != nil {
		return err
	}
	return model.NewMessageModel(s.db).Create(ctx, &model.Message{
		To:      n.To,
		Type:    n.Type,
		Title:   n.Title,
		Content: n.Content,
		Level:   n.Level,
		Source:  n.Source,
		RefType: n.RefType,
		RefID:   n.RefID,
		Scope:   model.JSON(scopeJSON),
	})
}
