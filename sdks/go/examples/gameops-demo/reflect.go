package main

import (
	"fmt"
	"reflect"
)

// recordFieldString extracts a field (by JSON tag or Go field name) as a
// comparable string. Numbers use integer text so numeric ordering is correct;
// dates/text compare lexically (ISO-8601 and lexicographic order coincide).
func recordFieldString(v any, key string) string {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return ""
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		if tag := f.Tag.Get("json"); tag != "" {
			name, _, _ := cutJSONTag(tag)
			if name == key {
				return formatField(rv.Field(i))
			}
		}
		if f.Name == key {
			return formatField(rv.Field(i))
		}
	}
	return ""
}

func formatField(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%020d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%020d", v.Uint())
	case reflect.String:
		return v.String()
	case reflect.Pointer:
		if v.IsNil() {
			return ""
		}
		return formatField(v.Elem())
	case reflect.Slice, reflect.Array:
		return fmt.Sprintf("%v", v.Interface())
	default:
		return ""
	}
}

func cutJSONTag(tag string) (name, rest string, ok bool) {
	for i, r := range tag {
		if r == ',' {
			return tag[:i], tag[i:], true
		}
	}
	return tag, "", true
}
