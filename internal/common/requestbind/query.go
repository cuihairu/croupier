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
// 反射兜底始终执行（不只是 ShouldBindQuery 出错时）：gin 的 ShouldBindQuery
// 对无 form tag 字段按**字段名精确匹配**（大小写敏感），而 query 契约是
// lowerCamelCase——无 tag 结构体（如 resourcecatalog.ListRequest）的小写
// 参数从未绑定成功，且 string-only 结构 ShouldBindQuery 恒成功、旧代码的
// 反射 fallback 永远不会执行，过滤参数静默丢失（OPEN-ISSUES #5 实证）。
func BindQueryCompat(c *gin.Context, req interface{}) error {
	// 绑定错误不提前返回：反射兜底补齐 gin 没绑上的字段后再统一校验。
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

		// 无 tag 兜底：query key 契约是 lowerCamelCase，而 gin 对无 tag
		// 字段按字段名精确匹配（大小写敏感），前端小写参数从未绑定成功
		// （OPEN-ISSUES #5）。lcFirst 命中 Env/Category 这类，全小写命中
		// ID→id 这类；Go 缩写风格字段（GameID）与契约键（gameId）大小写
		// 分布不同、字符串变换推不出来，最后按大小写不敏感兜底。
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
