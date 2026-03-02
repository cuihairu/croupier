package rate_limit

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

func parseRateLimitID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", errorx.NewBadRequest("限流规则ID不能为空")
	}
	return trimmed, nil
}

func normalizeRules(payload interface{}) (map[string]interface{}, error) {
	if payload == nil {
		return nil, nil
	}

	if asMap, ok := payload.(map[string]interface{}); ok {
		return asMap, nil
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errorx.NewBadRequest("解析规则失败")
	}
	var rules map[string]interface{}
	if err := json.Unmarshal(bytes, &rules); err != nil {
		return nil, errorx.NewBadRequest("规则必须为对象")
	}
	return rules, nil
}

func buildRateLimitResponse(limit *model.RateLimit) types.RateLimit {
	var rules interface{}
	if len(limit.Rules) > 0 {
		copyMap := make(map[string]interface{}, len(limit.Rules))
		for k, v := range limit.Rules {
			copyMap[k] = v
		}
		rules = copyMap
	}

	return types.RateLimit{
		Id:        limit.RateLimitID,
		Name:      limit.Name,
		Resource:  limit.Resource,
		Limit:     limit.Limit,
		Window:    limit.Window,
		Action:    limit.Action,
		Rules:     rules,
		Status:    limit.Status,
		UpdatedAt: utils.FormatTimestamp(limit.UpdatedAt),
	}
}

func generateRateLimitID(resource, name string) string {
	base := strings.ToLower(strings.TrimSpace(resource) + "-" + strings.TrimSpace(name))
	var builder strings.Builder
	prevHyphen := false
	for _, r := range base {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			prevHyphen = false
			continue
		}
		if !prevHyphen && builder.Len() > 0 {
			builder.WriteRune('-')
			prevHyphen = true
		}
	}

	id := strings.Trim(builder.String(), "-")
	if id == "" {
		return uuid.NewString()
	}
	return id
}

func encodeRules(rules map[string]interface{}) datatypes.JSONMap {
	if len(rules) == 0 {
		return datatypes.JSONMap{}
	}
	return datatypes.JSONMap(rules)
}

func summarizeRuleKeys(rules map[string]interface{}) []string {
	if len(rules) == 0 {
		return nil
	}
	keys := make([]string, 0, len(rules))
	for key := range rules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func classifyComplexity(keys int) string {
	switch {
	case keys >= 4:
		return "high"
	case keys >= 2:
		return "medium"
	default:
		return "low"
	}
}
