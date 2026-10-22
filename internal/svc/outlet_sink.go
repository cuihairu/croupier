package svc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"gorm.io/gorm"
)

// messageSink 把 outlet.StationNotice 落 messages 表（站内出口链首的
// 持久化面；outlet 包只依赖 MessageSink 接口，不反向依赖 model）。
type messageSink struct {
	db *gorm.DB
}

// Create implements outlet.MessageSink. 按 (recipient, ref_type, ref_id)
// upsert（incident-reports §6 重推幂等的站内侧）：同键已存在则刷新标题/
// 正文并按级别只升不降（info→warn→critical），不新建行。
func (s messageSink) Create(ctx context.Context, n outlet.StationNotice) error {
	scopeJSON, err := json.Marshal(n.Scope)
	if err != nil {
		return err
	}
	db := s.db.WithContext(ctx)
	var existing model.Message
	err = db.Where("recipient = ? AND ref_type = ? AND ref_id = ?", n.To, n.RefType, n.RefID).
		First(&existing).Error
	if err == nil {
		updates := map[string]interface{}{"title": n.Title, "content": n.Content}
		if noticeLevelRank(n.Level) > noticeLevelRank(existing.Level) {
			updates["level"] = n.Level
		}
		return db.Model(&model.Message{}).Where("id = ?", existing.ID).Updates(updates).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
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

// noticeLevelRank 通知级别序（只升不降的升级判定）。
func noticeLevelRank(level string) int {
	switch level {
	case model.MessageLevelCritical:
		return 2
	case model.MessageLevelWarn:
		return 1
	default:
		return 0
	}
}
