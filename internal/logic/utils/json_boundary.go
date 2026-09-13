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
	// json.Marshal 对 string 恒成功，无错误路径。
	encoded, _ := json.Marshal(string(value))
	return json.RawMessage(encoded)
}
