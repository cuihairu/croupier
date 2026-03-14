package requestbind

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// BindQueryCompat binds GET query parameters using form tags first and json tags as fallback.
// This keeps refactored handlers working even when DTOs only define json tags.
func BindQueryCompat(c *gin.Context, req interface{}) error {
	if err := c.ShouldBindQuery(req); err == nil {
		return nil
	}

	rv := reflect.ValueOf(req)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return binding.Validator.ValidateStruct(req)
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return binding.Validator.ValidateStruct(req)
	}

	rt := rv.Type()
	query := c.Request.URL.Query()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		value := rv.Field(i)
		if !value.CanSet() {
			continue
		}

		key := tagKey(field.Tag.Get("form"))
		if key == "" {
			key = tagKey(field.Tag.Get("json"))
		}
		if key == "" || key == "-" {
			continue
		}

		values, ok := query[key]
		if !ok || len(values) == 0 {
			continue
		}

		switch value.Kind() {
		case reflect.String:
			value.SetString(values[0])
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if parsed, err := strconv.ParseInt(values[0], 10, 64); err == nil {
				value.SetInt(parsed)
			}
		case reflect.Bool:
			if parsed, err := strconv.ParseBool(values[0]); err == nil {
				value.SetBool(parsed)
			}
		case reflect.Slice:
			if value.Type().Elem().Kind() == reflect.String {
				value.Set(reflect.ValueOf(values))
			}
		}
	}

	if binding.Validator == nil {
		return nil
	}
	return binding.Validator.ValidateStruct(req)
}

func tagKey(tag string) string {
	if tag == "" {
		return ""
	}
	key := strings.Split(tag, ",")[0]
	return strings.TrimSpace(key)
}
