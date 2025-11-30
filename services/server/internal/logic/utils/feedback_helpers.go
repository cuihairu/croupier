package utils

import (
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

// BuildFeedback converts model.Feedback into API response shape.
func BuildFeedback(record *model.Feedback) types.Feedback {
	if record == nil {
		return types.Feedback{}
	}
	return types.Feedback{
		Id:        int64(record.ID),
		PlayerId:  record.PlayerID,
		Contact:   record.Contact,
		Content:   record.Content,
		Category:  record.Category,
		Priority:  record.Priority,
		Status:    record.Status,
		Rating:    record.Rating,
		Attach:    record.Attach,
		GameId:    record.GameID,
		Env:       record.Env,
		Reply:     record.Reply,
		CreatedAt: FormatTimestamp(record.CreatedAt),
		UpdatedAt: FormatTimestamp(record.UpdatedAt),
	}
}

// NormalizeFeedbackRating clamps rating to [0,5].
func NormalizeFeedbackRating(rating int) int {
	if rating < 0 {
		return 0
	}
	if rating > 5 {
		return 5
	}
	return rating
}
