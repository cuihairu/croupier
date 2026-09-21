package generator

import (
	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// payloadFormKey is the single form field key used for functions without an
// input schema. The submitted form value is {"<key>": "<JSON text>"} — the
// payload key is NOT unwrapped by the execution chain (see 已知边界).
const payloadFormKey = "payload"

// payloadOnlyFormPresentation implements the promise made by the normalizer
// diagnostic (input_schema_missing: "function form will be a single payload
// field"): a function without an input schema still gets an operable form —
// one JSON text field instead of an empty, unusable one.
func payloadOnlyFormPresentation() *spec.FormPresentationSpec {
	schema := spec.JSONSchema(`{"type":"object","properties":{"payload":{"type":"string"}},"required":["payload"]}`)
	fp := spec.DefaultFormPresentation(schema)
	required := true
	fp.Fields = []spec.FormFieldSpec{
		{
			Key:      payloadFormKey,
			Widget:   spec.FormWidgetJSON,
			Width:    12,
			Required: &required,
			Label: spec.LocalizedText{
				"zh-CN": "请求载荷",
				"en-US": "Payload",
			},
			Description: spec.LocalizedText{
				"zh-CN": "该函数未提供入参 schema：请输入完整请求 JSON，内容将作为 payload 字段随请求提交。",
				"en-US": "This function has no input schema: enter the full request JSON; its content is submitted as the payload field.",
			},
			Placeholder: spec.LocalizedText{
				"zh-CN": `{ "key": "value" }`,
				"en-US": `{ "key": "value" }`,
			},
		},
	}
	return fp
}
