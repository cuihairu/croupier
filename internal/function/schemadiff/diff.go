// Package schemadiff 提供 JSON Schema 语义 diff，用于函数注册时的
// 契约兼容性检查。只做告警不做阻断：注册流程消费 Findings 决定是否
// 写入契约 Diagnostics / 返回注册警告。
package schemadiff

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Finding 单条 schema 差异。
type Finding struct {
	// Severity: "breaking" | "compatible"
	Severity string `json:"severity"`
	// Source: "input_schema" | "output_schema"
	Source string `json:"source"`
	// Path JSON Pointer（如 "/playerId"）
	Path string `json:"path"`
	// Reason 人类可读的变更说明
	Reason string `json:"reason"`
}

const (
	SeverityBreaking   = "breaking"
	SeverityCompatible = "compatible"
)

// HasBreaking 报告 findings 中是否存在破坏性变更。
func HasBreaking(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Severity == SeverityBreaking {
			return true
		}
	}
	return false
}

// DiffSchemas 对比旧/新 schema（JSON 原文），返回差异列表。
// 旧 schema 为空视为首次注册（无差异）；任一侧非法 JSON 返回 nil
// （非法 schema 由各自的校验器负责，diff 不重复报错）。
func DiffSchemas(source string, oldRaw, newRaw json.RawMessage) []Finding {
	if len(strings.TrimSpace(string(oldRaw))) == 0 {
		return nil
	}
	var oldNode, newNode interface{}
	if err := json.Unmarshal(oldRaw, &oldNode); err != nil {
		return nil
	}
	if err := json.Unmarshal(newRaw, &newNode); err != nil {
		return nil
	}
	findings := make([]Finding, 0, 4)
	diffNode(source, "$", oldNode, newNode, &findings)
	sortFindings(findings)
	return findings
}

func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity == SeverityBreaking
		}
		if findings[i].Source != findings[j].Source {
			return findings[i].Source < findings[j].Source
		}
		return findings[i].Path < findings[j].Path
	})
}

func diffNode(source, path string, oldNode, newNode interface{}, findings *[]Finding) {
	// 类型变更 = breaking
	if oldType, newType := jsonTypeName(oldNode), jsonTypeName(newNode); oldType != "" && newType != "" && oldType != newType {
		*findings = append(*findings, Finding{
			Severity: SeverityBreaking,
			Source:   source,
			Path:     path,
			Reason:   fmt.Sprintf("类型从 %s 变更为 %s", oldType, newType),
		})
		return
	}

	oldMap, oldIsMap := oldNode.(map[string]interface{})
	newMap, newIsMap := newNode.(map[string]interface{})
	if !oldIsMap || !newIsMap {
		return
	}

	// schema type 关键字变更（同为 object 时也可能收窄，如 integer→string 未体现为结构）
	if oldT, ok := oldMap["type"].(string); ok {
		if newT, ok2 := newMap["type"].(string); ok2 && oldT != "" && newT != "" && oldT != newT {
			*findings = append(*findings, Finding{
				Severity: SeverityBreaking,
				Source:   source,
				Path:     path,
				Reason:   fmt.Sprintf("schema type 从 %q 变更为 %q", oldT, newT),
			})
		}
	}

	diffProperties(source, path, oldMap, newMap, findings)
	diffRequired(source, path, oldMap, newMap, findings)
	diffEnum(source, path, oldMap, newMap, findings)
	diffDocKeywords(source, path, oldMap, newMap, findings)
}

// diffDocKeywords 描述/标题等文档性关键字的新增/删除/变更 = compatible。
func diffDocKeywords(source, path string, oldMap, newMap map[string]interface{}, findings *[]Finding) {
	for _, keyword := range []string{"description", "title"} {
		oldValue, oldOK := oldMap[keyword]
		newValue, newOK := newMap[keyword]
		if !oldOK && !newOK {
			continue
		}
		if oldOK && newOK && fmt.Sprint(oldValue) == fmt.Sprint(newValue) {
			continue
		}
		*findings = append(*findings, Finding{
			Severity: SeverityCompatible,
			Source:   source,
			Path:     path + "/" + keyword,
			Reason:   fmt.Sprintf("%s 变更", keyword),
		})
	}
}

func diffProperties(source, path string, oldMap, newMap map[string]interface{}, findings *[]Finding) {
	oldProps, _ := oldMap["properties"].(map[string]interface{})
	newProps, _ := newMap["properties"].(map[string]interface{})
	if oldProps == nil && newProps == nil {
		return
	}
	// 删除已有 property = breaking
	for key, oldChild := range oldProps {
		newChild, exists := newProps[key]
		if !exists {
			*findings = append(*findings, Finding{
				Severity: SeverityBreaking,
				Source:   source,
				Path:     path + "/" + key,
				Reason:   "已声明的字段被删除",
			})
			continue
		}
		diffNode(source, path+"/"+key, oldChild, newChild, findings)
	}
	// 新增可选 property = compatible
	newKeys := make([]string, 0, len(newProps))
	for key := range newProps {
		newKeys = append(newKeys, key)
	}
	sort.Strings(newKeys)
	for _, key := range newKeys {
		if _, exists := oldProps[key]; exists {
			continue
		}
		*findings = append(*findings, Finding{
			Severity: SeverityCompatible,
			Source:   source,
			Path:     path + "/" + key,
			Reason:   "新增可选字段",
		})
	}
}

func diffRequired(source, path string, oldMap, newMap map[string]interface{}, findings *[]Finding) {
	oldRequired := requiredSet(oldMap)
	newRequired := requiredSet(newMap)
	if oldRequired == nil && newRequired == nil {
		return
	}
	newKeys := make([]string, 0, len(newRequired))
	for key := range newRequired {
		newKeys = append(newKeys, key)
	}
	sort.Strings(newKeys)
	for _, key := range newKeys {
		if !oldRequired[key] {
			*findings = append(*findings, Finding{
				Severity: SeverityBreaking,
				Source:   source,
				Path:     path + "/required/" + key,
				Reason:   fmt.Sprintf("字段 %q 新增为必填", key),
			})
		}
	}
}

// diffEnum 枚举方向性判定（审查修正）：破坏方向取决于数据流向——
//   - input_schema：收窄 = breaking（旧调用方发被删的值会被服务端拒绝），
//     扩张 = compatible（旧调用方不受影响）；
//   - output_schema：扩张 = breaking（消费方会见到新值），
//     收窄 = compatible（消费方只会见到更少的值）。
func diffEnum(source, path string, oldMap, newMap map[string]interface{}, findings *[]Finding) {
	oldEnum, okOld := oldMap["enum"].([]interface{})
	newEnum, okNew := newMap["enum"].([]interface{})
	if !okOld || !okNew {
		return
	}
	isInput := source == "input_schema"
	oldValues := make(map[string]bool, len(oldEnum))
	for _, item := range oldEnum {
		oldValues[fmt.Sprint(item)] = true
	}
	newValues := make(map[string]bool, len(newEnum))
	changed := make([]string, 0)
	for _, item := range newEnum {
		key := fmt.Sprint(item)
		newValues[key] = true
		if !oldValues[key] {
			changed = append(changed, key)
		}
	}
	for _, item := range oldEnum {
		key := fmt.Sprint(item)
		if !newValues[key] {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	for _, key := range changed {
		added := newValues[key]
		breaking := added != isInput // input：删旧值破坏；output：增新值破坏
		severity := SeverityCompatible
		reason := fmt.Sprintf("枚举新增取值 %q", key)
		if !added {
			reason = fmt.Sprintf("枚举删除取值 %q", key)
		}
		if breaking {
			severity = SeverityBreaking
			if added {
				reason = fmt.Sprintf("枚举新增取值 %q（消费方会见到新值）", key)
			} else {
				reason = fmt.Sprintf("枚举删除取值 %q（旧调用方发被删值将被拒绝）", key)
			}
		}
		*findings = append(*findings, Finding{
			Severity: severity,
			Source:   source,
			Path:     path + "/enum/" + key,
			Reason:   reason,
		})
	}
}

func requiredSet(node map[string]interface{}) map[string]bool {
	raw, ok := node["required"].([]interface{})
	if !ok {
		return nil
	}
	set := make(map[string]bool, len(raw))
	for _, item := range raw {
		if key, ok := item.(string); ok {
			set[key] = true
		}
	}
	return set
}

// jsonTypeName 返回 JSON 值的基础类型名（结构对比用）。
func jsonTypeName(node interface{}) string {
	switch node.(type) {
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case float64, int, int64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return ""
	}
}
