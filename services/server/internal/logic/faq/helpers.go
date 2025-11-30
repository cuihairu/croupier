package faq

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

func buildFAQResponse(faq *model.FAQ) types.FAQ {
	return types.FAQ{
		Id:        int64(faq.ID),
		Question:  faq.Question,
		Answer:    faq.Answer,
		Category:  faq.Category,
		Tags:      decodeTags(faq.Tags),
		Visible:   faq.Visible,
		Sort:      faq.Sort,
		Views:     faq.Views,
		CreatedAt: utils.FormatTimestamp(faq.CreatedAt),
		UpdatedAt: utils.FormatTimestamp(faq.UpdatedAt),
	}
}

func decodeTags(data datatypes.JSON) []string {
	if len(data) == 0 {
		return nil
	}
	var tags []string
	if err := json.Unmarshal(data, &tags); err != nil {
		return nil
	}
	return tags
}

func encodeTags(tags []string) datatypes.JSON {
	if len(tags) == 0 {
		return datatypes.JSON{}
	}
	norm := normalizeTags(tags)
	bytes, _ := json.Marshal(norm)
	return datatypes.JSON(bytes)
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		lower := strings.ToLower(tag)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func sanitizeFAQInput(question, answer, category string) (string, string, string, error) {
	q := strings.TrimSpace(question)
	if q == "" {
		return "", "", "", fmt.Errorf("问题不能为空")
	}
	a := strings.TrimSpace(answer)
	if a == "" {
		return "", "", "", fmt.Errorf("答案不能为空")
	}
	c := strings.TrimSpace(category)
	if c == "" {
		return "", "", "", fmt.Errorf("分类不能为空")
	}
	return q, a, c, nil
}
