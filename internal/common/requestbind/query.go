package requestbind

import (
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// BindQueryCompat binds GET query parameters using form tags first and json tags as fallback.
// This keeps refactored handlers working even when DTOs only define json tags.
//
// The reflection fallback always runs (not only when ShouldBindQuery fails):
// gin's ShouldBindQuery matches **fields by exact field name**
// (case-sensitive) for fields without a form tag, while the query contract is
// lowerCamelCase — lowercase parameters of untagged structs (e.g.
// resourcecatalog.ListRequest) were never bound successfully, and
// ShouldBindQuery always succeeds for string-only structs, so the old
// reflection fallback never ran and filter parameters were silently lost
// (OPEN-ISSUES #5 evidence).
func BindQueryCompat(c *gin.Context, req interface{}) error {
	// Do not return early on a binding error: the reflection fallback fills in
	// the fields gin did not bind before validating everything together.
	_ = c.ShouldBindQuery(req)

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
		if key == "-" {
			continue
		}
		if key != "" {
			if values, ok := query[key]; ok && len(values) > 0 {
				setQueryValue(value, values)
			}
			continue
		}

		// Untagged fallback: the query key contract is lowerCamelCase, while
		// gin matches untagged fields by exact field name (case-sensitive),
		// so lowercase frontend parameters were never bound successfully
		// (OPEN-ISSUES #5). lcFirst catches names like Env/Category,
		// all-lowercase catches ID→id; Go abbreviation-style fields (GameID)
		// differ in case distribution from the contract key (gameId) in a way
		// no string transform can infer, so it finally falls back to a
		// case-insensitive match.
		bound := false
		for _, candidate := range []string{lcFirst(field.Name), strings.ToLower(field.Name)} {
			if values, ok := query[candidate]; ok && len(values) > 0 {
				setQueryValue(value, values)
				bound = true
				break
			}
		}
		if !bound {
			for key, values := range query {
				if len(values) > 0 && strings.EqualFold(key, field.Name) {
					setQueryValue(value, values)
					break
				}
			}
		}
	}

	if binding.Validator == nil {
		return nil
	}
	return binding.Validator.ValidateStruct(req)
}

func setQueryValue(value reflect.Value, values []string) {
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

func lcFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(unicode.ToLower(r[0])) + string(r[1:])
}

func tagKey(tag string) string {
	if tag == "" {
		return ""
	}
	key := strings.Split(tag, ",")[0]
	return strings.TrimSpace(key)
}
