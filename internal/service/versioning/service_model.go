package versioning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
)

func mustPageSpecFromModel(page *model.PageSpec) spec.PageSpec {
	pageSpec, err := pageSpecFromModel(page)
	if err != nil {
		return spec.PageSpec{}
	}
	return pageSpec
}

const rendererSchemaVersion = "page-spec:1"

func pageSpecFromModel(page *model.PageSpec) (spec.PageSpec, error) {
	if page == nil || strings.TrimSpace(page.SpecJSON) == "" {
		return spec.PageSpec{}, errorx.NewNotFound("page draft not found")
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(page.SpecJSON), &pageSpec); err != nil {
		return spec.PageSpec{}, fmt.Errorf("decode page spec: %w", err)
	}
	return normalizePageSpec(pageSpec), nil
}

func applyPageSpecToModel(page *model.PageSpec, pageSpec spec.PageSpec) error {
	pageSpec = normalizePageSpec(pageSpec)
	page.PageKey = pageSpec.PageKey
	page.Type = string(pageSpec.Type)
	page.ResourceKey = pageSpec.ResourceKey
	page.CategoryKey = pageSpec.Category.Key
	page.CategoryOrder = pageSpec.Category.Order
	page.Order = pageSpec.Order
	page.Icon = pageSpec.Icon
	// SetTitle 恒返回 nil（model 层实现为 `b, _ := json.Marshal(
	// map[string]string)`，无出错路径），原 err 检查为死分支，已删。下方
	// marshalPageSpec 的 err 真实可达（手工构造的非法 JSONSchema，见
	// TestV9_ApplyPageSpecToModel_Error），保留。
	_ = page.SetTitle(pageSpec.Title)
	specJSON, err := marshalPageSpec(pageSpec)
	if err != nil {
		return err
	}
	page.SpecJSON = specJSON
	return nil
}

func normalizePageSpec(page spec.PageSpec) spec.PageSpec {
	page.PageKey = strings.TrimSpace(page.PageKey)
	page.ResourceKey = strings.TrimSpace(page.ResourceKey)
	page.Icon = strings.TrimSpace(page.Icon)
	page.Title = normalizeLocalizedText(page.Title)
	page.Description = normalizeLocalizedText(page.Description)
	page.Category.Key = strings.TrimSpace(page.Category.Key)
	for i := range page.Bindings {
		page.Bindings[i].ID = strings.TrimSpace(page.Bindings[i].ID)
		page.Bindings[i].FunctionID = strings.TrimSpace(page.Bindings[i].FunctionID)
	}
	return page
}

func marshalPageSpec(page spec.PageSpec) (string, error) {
	page = normalizePageSpec(page)
	raw, err := json.Marshal(page)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func normalizeLocalizedText(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch strings.ToLower(strings.ReplaceAll(key, "_", "-")) {
		case "zh", "zh-cn":
			out["zh-CN"] = value
		case "en", "en-us":
			out["en-US"] = value
		default:
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func digestRaw(raw model.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	return freshness.CanonicalDigest(raw)
}

func localizedTextToJSONMap(values spec.LocalizedText) datatypes.JSONMap {
	if len(values) == 0 {
		return datatypes.JSONMap{}
	}
	out := datatypes.JSONMap{}
	for key, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			out[key] = text
		}
	}
	return out
}

func jsonValue(v interface{}) model.JSON {
	raw, err := json.Marshal(v)
	if err != nil {
		return model.JSON([]byte("null"))
	}
	return model.JSON(raw)
}

func computeDigest(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func jsonString(value string) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func jsonNumber(value uint) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
