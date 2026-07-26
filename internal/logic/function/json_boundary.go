package function

import (
	"encoding/json"
	"strings"
)

func rawJSONFromValue(value interface{}) json.RawMessage {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case json.RawMessage:
		return rawJSONFromBytes(v)
	case []byte:
		return rawJSONFromBytes(v)
	case string:
		return rawJSONFromBytes([]byte(strings.TrimSpace(v)))
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return rawJSONFromBytes(data)
	}
}

func rawJSONFromBytes(value []byte) json.RawMessage {
	value = append([]byte(nil), value...)
	if len(value) == 0 {
		return nil
	}
	if json.Valid(value) {
		return json.RawMessage(value)
	}
	encoded, err := json.Marshal(string(value))
	if err != nil {
		return nil
	}
	return json.RawMessage(encoded)
}

func jsonValueFromRaw(raw json.RawMessage) (interface{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func jsonObjectFromRaw(raw json.RawMessage) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value map[string]interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}
