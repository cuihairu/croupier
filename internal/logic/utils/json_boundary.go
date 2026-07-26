package utils

import "encoding/json"

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
		return rawJSONFromBytes([]byte(v))
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
