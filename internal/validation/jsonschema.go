package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const schemaResourceName = "schema.json"

// ValidateJSON validates a JSON payload `data` against a JSON Schema contained
// in `schema`. Local $defs/$ref are supported by the jsonschema compiler.
//
// Unknown object fields are accepted unless the schema explicitly declares
// `additionalProperties: false` (standard JSON Schema semantics, SUI-016):
// every server entry that validates an invoke payload shares this behavior.
func ValidateJSON(schema map[string]any, data []byte) error {
	if schema == nil {
		schema = map[string]any{}
	}
	return ValidateJSONSchema(schema, data)
}

// ValidateJSONRaw validates a JSON payload against a raw JSON Schema object.
func ValidateJSONRaw(schema json.RawMessage, data []byte) error {
	if len(bytes.TrimSpace(schema)) == 0 {
		return ValidateJSONSchema(map[string]any{}, data)
	}
	var parsed any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		return fmt.Errorf("invalid JSON Schema: %w", err)
	}
	return ValidateJSONSchema(parsed, data)
}

// ValidateJSONSchema validates a payload against a JSON Schema value and
// renders the verdict as one error.
func ValidateJSONSchema(schema any, data []byte) error {
	verdict, err := ValidateJSONSchemaVerdict(schema, data)
	if err != nil {
		return err
	}
	if verdict.HasViolations() {
		return fmt.Errorf("payload does not match schema: %s", strings.Join(verdict.AllMessages(), "; "))
	}
	return nil
}

// SchemaVerdict is the structured outcome of one payload-vs-schema check
// (SUI-016). Every server entry that validates an invoke payload maps this
// verdict onto its own HTTP error shape, so the same schema and payload
// produce the same verdict on the console execution path and on the direct
// function invoke path.
type SchemaVerdict struct {
	// MissingRequired lists the field paths of required-but-absent properties.
	MissingRequired []string
	// OtherMessages lists the rendered non-required violations, sorted.
	OtherMessages []string
	// Details maps each field path to "required" or its rendered message.
	Details map[string]string
}

// HasViolations reports whether the payload violates the schema.
func (v *SchemaVerdict) HasViolations() bool {
	return v != nil && (len(v.MissingRequired) > 0 || len(v.OtherMessages) > 0)
}

// AllMessages returns the required-missing and other violations as one sorted
// list for rendering into an error message; nil when there are none.
func (v *SchemaVerdict) AllMessages() []string {
	if v == nil {
		return nil
	}
	if len(v.MissingRequired) == 0 && len(v.OtherMessages) == 0 {
		return nil
	}
	out := make([]string, 0, len(v.MissingRequired)+len(v.OtherMessages))
	for _, field := range v.MissingRequired {
		out = append(out, field+": required")
	}
	out = append(out, v.OtherMessages...)
	sort.Strings(out)
	return out
}

// ValidateJSONSchemaVerdict separates the three outcomes an entry must
// distinguish (SUI-016):
//
//   - a non-nil returned error is a schema or payload DEFECT: the schema does
//     not compile, or the payload is not valid JSON. Entries reject it
//     fail-closed instead of silently treating a declared contract as absent.
//   - a verdict with violations: the payload does not satisfy the contract.
//   - an empty verdict: the payload satisfies the contract.
func ValidateJSONSchemaVerdict(schema any, data []byte) (*SchemaVerdict, error) {
	payload, err := decodeJSONPayload(data)
	if err != nil {
		return nil, err
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	// AddResource 恒成功（v6 实现：固定合法 URL "schema.json" 的 absolute 解析、
	// 非 meta URL、新 compiler 无重复资源），且它不解析 doc 本身——schema 内容
	// 的合法性错误全部由下方 Compile 报告，故无需检查其返回值。
	_ = compiler.AddResource(schemaResourceName, schema)
	compiled, err := compiler.Compile(schemaResourceName)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON Schema: %w", err)
	}
	if err := compiled.Validate(payload); err != nil {
		validationError, ok := err.(*jsonschema.ValidationError)
		if !ok {
			return nil, fmt.Errorf("payload does not match schema: %w", err)
		}
		return collectViolations(validationError), nil
	}
	return &SchemaVerdict{}, nil
}

// collectViolations flattens the compiler's error tree into field-level paths
// so both server entries can name the offending fields identically.
func collectViolations(validationError *jsonschema.ValidationError) *SchemaVerdict {
	verdict := &SchemaVerdict{Details: map[string]string{}}
	collectErrors(validationError, "", verdict)
	sort.Strings(verdict.MissingRequired)
	sort.Strings(verdict.OtherMessages)
	return verdict
}

func collectErrors(
	validationError *jsonschema.ValidationError,
	path string,
	verdict *SchemaVerdict,
) {
	if len(validationError.InstanceLocation) > 0 {
		path = strings.Join(validationError.InstanceLocation, ".")
	}

	if required, ok := validationError.ErrorKind.(*kind.Required); ok {
		for _, name := range required.Missing {
			fieldPath := name
			if path != "" {
				fieldPath = path + "." + name
			}
			verdict.Details[fieldPath] = "required"
			verdict.MissingRequired = append(verdict.MissingRequired, fieldPath)
		}
		return
	}

	msg := validationError.ErrorKind.LocalizedString(message.NewPrinter(language.English))
	// The compiler's root node renders the generic "jsonschema validation
	// failed" banner on top of its causes; it names no field, so keep it out
	// of the field-level messages whenever real causes exist.
	if msg != "" && !(strings.HasPrefix(msg, "jsonschema validation failed") && len(validationError.Causes) > 0) {
		if path != "" {
			msg = path + ": " + msg
		}
		verdict.Details[detailKey(path)] = msg
		verdict.OtherMessages = append(verdict.OtherMessages, msg)
	}

	for _, cause := range validationError.Causes {
		collectErrors(cause, path, verdict)
	}
}

func detailKey(path string) string {
	if path == "" {
		return "payload"
	}
	return fmt.Sprintf("payload.%s", path)
}

func decodeJSONPayload(data []byte) (any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("invalid JSON: multiple JSON values")
	}
	return payload, nil
}
