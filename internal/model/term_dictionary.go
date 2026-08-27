package model

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TermDictionary stores normalized game terminology entries used for aliasing and display hints.
//
// Display 是本地化显示文本的权威存储（JSON 列，key 必须是 BCP47 locale
// 如 "zh-CN"/"en-US"，对齐 spec.LocalizedText 契约）。旧双列
// display_zh/display_en 仅在迁移期存在：MigrateTermDictionaryDisplay 会
// 把旧值回填进 Display 并删除旧列，之后一切读写都走 Display。
type TermDictionary struct {
	ID        uint              `gorm:"primaryKey"`
	Domain    string            `gorm:"size:32;not null;index:idx_term_domain_key,priority:1;uniqueIndex:uidx_term_domain_alias,priority:1"`
	TermKey   string            `gorm:"size:64;not null;index:idx_term_domain_key,priority:2"`
	Alias     string            `gorm:"size:128;not null;uniqueIndex:uidx_term_domain_alias,priority:2"`
	Display   map[string]string `gorm:"serializer:json"`
	SortOrder int               `gorm:"default:100"`
	CreatedAt time.Time         `gorm:"autoCreateTime"`
	UpdatedAt time.Time         `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt    `gorm:"index"`
}

func (TermDictionary) TableName() string {
	return "term_dictionary"
}

// NormalizeTermDisplay 规整本地化显示文本：
//   - key 归一为 BCP47 标准形式（zh/zh_cn → zh-CN，en → en-US）
//   - 丢弃空值与非法 key，输出稳定有序
func NormalizeTermDisplay(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		norm := NormalizeLocaleKey(k)
		v = strings.TrimSpace(v)
		if norm == "" || v == "" {
			continue
		}
		out[norm] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// marshalTermDisplay 序列化为 JSON 列值；nil map 返回 nil。
func marshalTermDisplay(m map[string]string) (any, error) {
	if len(m) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// UnmarshalTermDisplayText 解析 JSON 列文本为 map；空串/解析失败返回 nil。
func UnmarshalTermDisplayText(s string) map[string]string {
	if s == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// NormalizeLocaleKey 把宽松的 locale 写法归一为 BCP47；
// 无法识别时返回空串（调用方应丢弃该 key）。
func NormalizeLocaleKey(k string) string {
	k = strings.TrimSpace(strings.ReplaceAll(k, "_", "-"))
	if k == "" {
		return ""
	}
	parts := strings.Split(k, "-")
	switch strings.ToLower(parts[0]) {
	case "zh":
		if len(parts) > 1 {
			region := strings.ToUpper(parts[len(parts)-1])
			switch region {
			case "CN", "HANS":
				return "zh-CN"
			case "TW", "HK", "HANT":
				return "zh-TW"
			}
		}
		return "zh-CN"
	case "en":
		return "en-US"
	default:
		// 其余语言：language-REGION 形式规范大小写。
		lang := strings.ToLower(parts[0])
		if len(parts) > 1 {
			return lang + "-" + strings.ToUpper(parts[len(parts)-1])
		}
		return lang
	}
}
