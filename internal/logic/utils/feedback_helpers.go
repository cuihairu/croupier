package utils

import (
	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
)

// Feedback represents a feedback item in API responses.
type Feedback struct {
	Id        int64  `json:"id"`
	PlayerId  string `json:"playerId"`
	Contact   string `json:"contact"`
	Content   string `json:"content"`
	Category  string `json:"category"`
	Priority  string `json:"priority"`
	Status    string `json:"status"`
	Rating    int    `json:"rating"`
	Attach    string `json:"attach"`
	GameId    string `json:"gameId"`
	Env       string `json:"env"`
	Reply     string `json:"reply"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// BuildFeedback converts model.Feedback into API response shape.
func BuildFeedback(record *model.Feedback) Feedback {
	if record == nil {
		return Feedback{}
	}
	return Feedback{
		Id:        int64(record.ID),
		PlayerId:  record.PlayerID,
		Contact:   record.Contact,
		Content:   record.Content,
		Category:  record.Category,
		Priority:  record.Priority,
		Status:    record.Status.String(),
		Rating:    record.Rating,
		Attach:    record.Attach,
		GameId:    record.GameID,
		Env:       record.Env,
		Reply:     record.Reply,
		CreatedAt: helper.FormatTimestamp(record.CreatedAt),
		UpdatedAt: helper.FormatTimestamp(record.UpdatedAt),
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
