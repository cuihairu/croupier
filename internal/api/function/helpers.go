package function

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	functioninvoke "github.com/cuihairu/croupier/internal/platform/functioninvoke"
)

// Function management implementations

// getStringFromMetadata gets a string value from metadata map
func getStringFromMetadata(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	if val, ok := metadata[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getIntFromMetadata gets an int value from metadata map
func getIntFromMetadata(metadata map[string]interface{}, key string) int {
	if metadata == nil {
		return 0
	}
	if val, ok := metadata[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			// Try to parse string as int
			var i int
			if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
				return i
			}
		}
	}
	return 0
}

// parseRolesFromJSON parses roles from JSON array or comma-separated string
func parseRolesFromJSON(data model.JSON) []string {
	if len(data) == 0 {
		return []string{}
	}
	// Try to parse as JSON array
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	// Try to parse as comma-separated string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str != "" {
			return strings.Split(str, ",")
		}
	}
	return []string{}
}

// parseActionsFromJSON parses actions from JSON array
func parseActionsFromJSON(data model.JSON) []string {
	if len(data) == 0 {
		return []string{}
	}
	// Try to parse as JSON array
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	// Try to parse as comma-separated string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str != "" {
			return strings.Split(str, ",")
		}
	}
	return []string{}
}

func jsonValueFromMetadata(metadata map[string]interface{}, key string) interface{} {
	if metadata == nil {
		return nil
	}
	if val, ok := metadata[key]; ok {
		return val
	}
	return nil
}

// getStringSliceFromMetadata gets a string slice value from metadata map
func getStringSliceFromMetadata(metadata map[string]interface{}, key string) []string {
	if metadata == nil {
		return []string{}
	}
	if val, ok := metadata[key]; ok {
		switch v := val.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return []string{}
}

// getBoolFromMetadata gets a bool value from metadata map
func getBoolFromMetadata(metadata map[string]interface{}, key string) bool {
	if metadata == nil {
		return false
	}
	if val, ok := metadata[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v == "true" || v == "1"
		case int:
			return v != 0
		case float64:
			return v != 0
		}
	}
	return false
}

func invokePayload(req *FunctionInvokeRequest) []byte {
	if req == nil {
		return []byte("null")
	}
	return functioninvoke.InvokePayload(functioninvokeRequestFromDTO(req))
}

func rawJSONFromAny(value interface{}) json.RawMessage {
	return functioninvoke.RawJSONFromAny(value)
}

func rawJSONFromBytes(value []byte) json.RawMessage {
	return functioninvoke.RawJSONFromBytes(value)
}

// getLocalizedTextFromMetadata gets a localized text map from metadata
func getLocalizedTextFromMetadata(metadata map[string]interface{}, key string) map[string]string {
	if metadata == nil {
		return nil
	}
	val, ok := metadata[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case map[string]string:
		return v
	case map[string]interface{}:
		result := make(map[string]string, len(v))
		for k, item := range v {
			if str, ok := item.(string); ok {
				result[k] = str
			}
		}
		return result
	}
	return nil
}

// firstNonEmptySlice returns the first non-empty slice, or nil if both are empty.
func firstNonEmptySlice(a, b []string) []string {
	if len(a) > 0 {
		return a
	}
	return b
}

// firstNonEmptyMap returns the first non-empty map, or nil if both are empty.
func firstNonEmptyMap(a, b map[string]string) map[string]string {
	if len(a) > 0 {
		return a
	}
	return b
}
