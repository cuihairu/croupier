package functioninvoke

import "encoding/json"

// marshalRawString encodes non-JSON bytes as a JSON string; it is a package
// seam because json.Marshal has no failing input for strings.
var marshalRawString = json.Marshal

// RawJSONFromAny converts an arbitrary value into a valid JSON raw message.
func RawJSONFromAny(value interface{}) json.RawMessage {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case json.RawMessage:
		return append(json.RawMessage(nil), v...)
	case []byte:
		return RawJSONFromBytes(v)
	case string:
		return RawJSONFromBytes([]byte(v))
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return RawJSONFromBytes(b)
	}
}

// RawJSONFromBytes normalizes raw bytes into a valid JSON raw message; plain
// text is JSON-string encoded instead of being dropped.
func RawJSONFromBytes(value []byte) json.RawMessage {
	value = append([]byte(nil), value...)
	if len(value) == 0 {
		return nil
	}
	if json.Valid(value) {
		return json.RawMessage(value)
	}
	encoded, err := marshalRawString(string(value))
	if err != nil {
		return nil
	}
	return json.RawMessage(encoded)
}
