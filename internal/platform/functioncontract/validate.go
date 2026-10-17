package functioncontract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/cuihairu/croupier/internal/common/errorx"
)

const schemaResourceName = "schema.json"

// ValidateInput validates a payload against a function descriptor's input
// schema. The platform intentionally treats unavailable or malformed schemas
// as pass-through: the game server remains the authoritative runtime validator.
func ValidateInput(schema map[string]interface{}, payload []byte) error {
	if len(schema) == 0 {
		return nil
	}
	compilerSchema := normalizeRequired(schema)

	var instance interface{}
	if err := json.Unmarshal(payload, &instance); err != nil {
		return nil
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	if err := compiler.AddResource(schemaResourceName, compilerSchema); err != nil {
		return nil
	}
	compiled, err := compiler.Compile(schemaResourceName)
	if err != nil {
		return nil
	}

	validationErr := compiled.Validate(instance)
	if validationErr == nil {
		return nil
	}
	validationError, ok := validationErr.(*jsonschema.ValidationError)
	if !ok {
		return nil
	}
	return badRequest(validationError)
}

func badRequest(validationError *jsonschema.ValidationError) error {
	details := make(map[string]any)
	var missing []string
	var otherMessages []string
	collectErrors(validationError, "", details, &missing, &otherMessages)

	if len(missing) > 0 {
		sort.Strings(missing)
		return errorx.NewBadRequestWithDetails(
			"缺少必填参数: "+strings.Join(missing, ", "),
			details,
		)
	}
	sort.Strings(otherMessages)
	return errorx.NewBadRequestWithDetails(
		"输入契约校验失败: "+strings.Join(otherMessages, "; "),
		details,
	)
}

func normalizeRequired(schema map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(schema))
	for key, value := range schema {
		normalized[key] = value
	}

	rawRequired, ok := normalized["required"].([]interface{})
	if !ok {
		return normalized
	}
	required := make([]interface{}, 0, len(rawRequired))
	for _, entry := range rawRequired {
		if name, ok := entry.(string); ok && name != "" {
			required = append(required, name)
		}
	}
	normalized["required"] = required
	return normalized
}

func collectErrors(
	validationError *jsonschema.ValidationError,
	path string,
	details map[string]any,
	missing *[]string,
	otherMessages *[]string,
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
			details[fieldPath] = "required"
			*missing = append(*missing, fieldPath)
		}
		return
	}

	message := validationError.ErrorKind.LocalizedString(message.NewPrinter(language.English))
	if path != "" {
		message = path + ": " + message
	}
	if message != "" {
		details[detailKey(path)] = message
		*otherMessages = append(*otherMessages, message)
	}

	for _, cause := range validationError.Causes {
		collectErrors(cause, path, details, missing, otherMessages)
	}
}

func detailKey(path string) string {
	if path == "" {
		return "payload"
	}
	return fmt.Sprintf("payload.%s", path)
}
