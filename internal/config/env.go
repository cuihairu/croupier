package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// EnvManager 环境变量管理器
type EnvManager struct {
	prefix       string
	mappings     map[string]string
	converters   map[reflect.Kind]EnvConverter
	transformers []EnvTransformer
}

// EnvConverter 环境变量转换器接口
type EnvConverter interface {
	Convert(value string) (interface{}, error)
	Kind() reflect.Kind
}

// EnvTransformer 环境变量转换器接口
type EnvTransformer interface {
	Transform(key, value string) (string, string, error)
	Name() string
}

// NewEnvManager 创建环境变量管理器
func NewEnvManager(prefix string) *EnvManager {
	em := &EnvManager{
		prefix:     prefix,
		mappings:   make(map[string]string),
		converters: make(map[reflect.Kind]EnvConverter),
	}

	// 注册默认转换器
	em.registerDefaultConverters()

	return em
}

// registerDefaultConverters 注册默认转换器
func (em *EnvManager) registerDefaultConverters() {
	em.converters[reflect.String] = &StringConverter{}
	em.converters[reflect.Int] = &IntConverter{}
	em.converters[reflect.Int64] = &Int64Converter{}
	em.converters[reflect.Float64] = &Float64Converter{}
	em.converters[reflect.Bool] = &BoolConverter{}
	em.converters[reflect.Slice] = &SliceConverter{}
	em.converters[reflect.Map] = &MapConverter{}
}

// AddConverter 添加转换器
func (em *EnvManager) AddConverter(converter EnvConverter) {
	em.converters[converter.Kind()] = converter
}

// AddTransformer 添加转换器
func (em *EnvManager) AddTransformer(transformer EnvTransformer) {
	em.transformers = append(em.transformers, transformer)
}

// LoadFromEnv 从环境变量加载配置
func (em *EnvManager) LoadFromEnv(config interface{}) error {
	configValue := reflect.ValueOf(config)
	if configValue.Kind() != reflect.Ptr || configValue.IsNil() {
		return fmt.Errorf("config必须是非nil指针")
	}

	configValue = configValue.Elem()
	if configValue.Kind() != reflect.Struct {
		return fmt.Errorf("config必须指向结构体")
	}

	return em.loadStruct(configValue, "")
}

// loadStruct 递归加载结构体
func (em *EnvManager) loadStruct(value reflect.Value, prefix string) error {
	structType := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := structType.Field(i)

		// 跳过不可设置字段
		if !field.CanSet() {
			continue
		}

		// 获取环境变量键名
		envKey := em.getEnvKey(fieldType, prefix)
		if envKey == "" {
			continue
		}

		// 获取环境变量值
		envValue := os.Getenv(envKey)
		if envValue == "" {
			// 如果没有直接的环境变量，尝试加载嵌套结构体
			if field.Kind() == reflect.Struct {
				if err := em.loadStruct(field, em.getFieldPrefix(fieldType, prefix)); err != nil {
					return err
				}
			}
			continue
		}

		// 应用转换器
		_, transformedValue, err := em.applyTransformers(envKey, envValue)
		if err != nil {
			return fmt.Errorf("转换环境变量失败 %s: %w", envKey, err)
		}

		// 设置字段值
		if err := em.setFieldValue(field, transformedValue); err != nil {
			return fmt.Errorf("设置字段值失败 %s: %w", envKey, err)
		}
	}

	return nil
}

// getEnvKey 获取环境变量键名
func (em *EnvManager) getEnvKey(field reflect.StructField, prefix string) string {
	// 检查env标签
	if envTag := field.Tag.Get("env"); envTag != "" {
		if envTag == "-" {
			return "" // 跳过此字段
		}
		return em.prefix + envTag
	}

	// 检查mapped标签
	if mappedTag := field.Tag.Get("mapped"); mappedTag != "" {
		if mappedKey, ok := em.mappings[mappedTag]; ok {
			return em.prefix + mappedKey
		}
	}

	// 默认使用字段名转换为环境变量名
	if prefix != "" {
		return em.prefix + prefix + "_" + strings.ToUpper(field.Name)
	}
	return em.prefix + strings.ToUpper(field.Name)
}

// getFieldPrefix 获取字段前缀
func (em *EnvManager) getFieldPrefix(field reflect.StructField, prefix string) string {
	fieldName := strings.ToUpper(field.Name)
	if prefix != "" {
		return prefix + "_" + fieldName
	}
	return fieldName
}

// applyTransformers 应用转换器
func (em *EnvManager) applyTransformers(key, value string) (string, string, error) {
	transformedKey, transformedValue := key, value

	for _, transformer := range em.transformers {
		newKey, newValue, err := transformer.Transform(transformedKey, transformedValue)
		if err != nil {
			return "", "", err
		}
		transformedKey, transformedValue = newKey, newValue
	}

	return transformedKey, transformedValue, nil
}

// setFieldValue 设置字段值
func (em *EnvManager) setFieldValue(field reflect.Value, value string) error {
	kind := field.Kind()
	fieldType := field.Type()

	// 特殊处理time.Duration类型
	if fieldType == reflect.TypeOf(time.Duration(0)) {
		duration, err := parseDurationValue(value)
		if err != nil {
			return fmt.Errorf("解析时间段失败: %w", err)
		}
		field.SetInt(int64(duration))
		return nil
	}

	if converter, ok := em.converters[kind]; ok {
		convertedValue, err := converter.Convert(value)
		if err != nil {
			return err
		}

		// 处理time.Duration类型
		if duration, ok := convertedValue.(time.Duration); ok {
			field.SetInt(int64(duration))
			return nil
		}

		field.Set(reflect.ValueOf(convertedValue))
		return nil
	}

	// 特殊处理指针类型
	if kind == reflect.Ptr {
		if field.IsNil() {
			// 创建新实例
			elemType := field.Type().Elem()
			field.Set(reflect.New(elemType))
		}
		return em.setFieldValue(field.Elem(), value)
	}

	return fmt.Errorf("不支持的字段类型: %s", kind)
}

// AddMapping 添加字段映射
func (em *EnvManager) AddMapping(key, envKey string) {
	em.mappings[key] = envKey
}

// ExportToEnv 导出配置到环境变量
func (em *EnvManager) ExportToEnv(config interface{}) error {
	configValue := reflect.ValueOf(config)
	if configValue.Kind() != reflect.Ptr || configValue.IsNil() {
		return fmt.Errorf("config必须是非nil指针")
	}

	configValue = configValue.Elem()
	if configValue.Kind() != reflect.Struct {
		return fmt.Errorf("config必须指向结构体")
	}

	return em.exportStruct(configValue, "")
}

// exportStruct 递归导出结构体
func (em *EnvManager) exportStruct(value reflect.Value, prefix string) error {
	// 处理指针类型
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	// 如果不是结构体，则无法导出
	if value.Kind() != reflect.Struct {
		return nil
	}

	structType := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := structType.Field(i)

		// 跳过不可设置的字段
		if !field.CanSet() {
			continue
		}

		// 跳过零值字段
		if field.IsZero() {
			continue
		}

		// 检查是否有env标签
		envTag := fieldType.Tag.Get("env")
		if envTag == "-" {
			continue // 跳过明确标记的字段
		}

		// 处理嵌套结构体
		if field.Kind() == reflect.Struct {
			// 检查是否是time.Duration类型
			if field.Type() == reflect.TypeOf(time.Duration(0)) {
				duration := field.Interface().(time.Duration)
				var durationEnvKey string
				if envTag != "" {
					durationEnvKey = em.prefix + envTag
				} else {
					durationEnvKey = em.prefix + strings.ToUpper(em.getFieldPrefix(fieldType, prefix)+"_"+fieldType.Name)
				}
				if err := os.Setenv(durationEnvKey, duration.String()); err != nil {
					return fmt.Errorf("设置环境变量失败 %s: %w", durationEnvKey, err)
				}
			} else {
				// 递归处理嵌套结构体
				newPrefix := em.getFieldPrefix(fieldType, prefix)
				if err := em.exportStruct(field, newPrefix); err != nil {
					return err
				}
			}
			continue
		}

		// 处理指针类型
		if field.Kind() == reflect.Ptr {
			if !field.IsNil() {
				if err := em.exportStruct(field.Elem(), em.getFieldPrefix(fieldType, prefix)); err != nil {
					return err
				}
			}
			continue
		}

		// 获取环境变量键名
		var envKey string
		if envTag != "" {
			envKey = em.prefix + envTag
		} else {
			keyPrefix := em.getFieldPrefix(fieldType, prefix)
			if keyPrefix == "" {
				continue
			}
			envKey = em.prefix + keyPrefix
		}

		if envKey == "" || envKey == em.prefix {
			continue
		}

		// 设置环境变量
		var envValue string
		switch field.Kind() {
		case reflect.String:
			envValue = field.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// 特殊处理time.Duration类型
			if field.Type() == reflect.TypeOf(time.Duration(0)) {
				envValue = field.Interface().(time.Duration).String()
			} else {
				envValue = fmt.Sprintf("%d", field.Int())
			}
		case reflect.Bool:
			envValue = fmt.Sprintf("%t", field.Bool())
		case reflect.Float32, reflect.Float64:
			envValue = fmt.Sprintf("%f", field.Float())
		case reflect.Slice:
			// 处理切片类型，将元素用逗号连接
			if field.Len() > 0 {
				var strSlice []string
				for i := 0; i < field.Len(); i++ {
					elem := field.Index(i)
					strSlice = append(strSlice, fmt.Sprintf("%v", elem.Interface()))
				}
				envValue = strings.Join(strSlice, ",")
			}
		case reflect.Array:
			// 处理数组类型
			if field.Len() > 0 {
				var strSlice []string
				for i := 0; i < field.Len(); i++ {
					elem := field.Index(i)
					strSlice = append(strSlice, fmt.Sprintf("%v", elem.Interface()))
				}
				envValue = strings.Join(strSlice, ",")
			}
		default:
			envValue = fmt.Sprintf("%v", field.Interface())
		}

		if err := os.Setenv(envKey, envValue); err != nil {
			return fmt.Errorf("设置环境变量失败 %s: %w", envKey, err)
		}
	}

	return nil
}

// GetEnvInfo 获取环境变量信息
func (em *EnvManager) GetEnvInfo() map[string]EnvInfo {
	info := make(map[string]EnvInfo)

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, em.prefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]

				info[key] = EnvInfo{
					Key:   key,
					Value: em.maskSensitiveValue(key, value),
					Set:   true,
				}
			}
		}
	}

	return info
}

// maskSensitiveValue 掩码敏感值
func (em *EnvManager) maskSensitiveValue(key, value string) string {
	keyUpper := strings.ToUpper(key)
	segments := strings.FieldsFunc(keyUpper, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})

	if isSensitiveKey(keyUpper, segments) {
		return maskValue(value)
	}

	return value
}

func isSensitiveKey(rawKey string, segments []string) bool {
	// 先检查常见的模式
	explicitPatterns := []string{
		"PASSWORD",
		"SECRET",
		"TOKEN",
		"CREDENTIAL",
		"CREDENTIALS",
		"PRIVATE_KEY",
		"ACCESS_KEY",
		"API_KEY",
		"SECRET_KEY",
	}

	for _, pattern := range explicitPatterns {
		if strings.Contains(rawKey, pattern) {
			return true
		}
	}

	// 再检查拆分后的段
	for _, segment := range segments {
		switch segment {
		case "PASSWORD", "PASS", "PWD", "SECRET", "TOKEN", "CREDENTIAL", "CREDENTIALS", "PRIVATE", "KEY":
			return true
		}

		if strings.HasSuffix(segment, "SECRET") ||
			strings.HasSuffix(segment, "PASSWORD") ||
			strings.HasSuffix(segment, "TOKEN") {
			return true
		}
	}

	return false
}

func maskValue(value string) string {
	if len(value) <= 4 {
		return "****"
	}

	return value[:2] + "****" + value[len(value)-2:]
}

// EnvInfo 环境变量信息
type EnvInfo struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Set   bool   `json:"set"`
}

// 默认转换器实现

// StringConverter 字符串转换器
type StringConverter struct{}

func (c *StringConverter) Convert(value string) (interface{}, error) {
	return value, nil
}

func (c *StringConverter) Kind() reflect.Kind {
	return reflect.String
}

// IntConverter 整数转换器
type IntConverter struct{}

func (c *IntConverter) Convert(value string) (interface{}, error) {
	return strconv.Atoi(value)
}

func (c *IntConverter) Kind() reflect.Kind {
	return reflect.Int
}

// Int64Converter 64位整数转换器
type Int64Converter struct{}

func (c *Int64Converter) Convert(value string) (interface{}, error) {
	return strconv.ParseInt(value, 10, 64)
}

func (c *Int64Converter) Kind() reflect.Kind {
	return reflect.Int64
}

// Float64Converter 浮点数转换器
type Float64Converter struct{}

func (c *Float64Converter) Convert(value string) (interface{}, error) {
	return strconv.ParseFloat(value, 64)
}

func (c *Float64Converter) Kind() reflect.Kind {
	return reflect.Float64
}

// BoolConverter 布尔转换器
type BoolConverter struct{}

func (c *BoolConverter) Convert(value string) (interface{}, error) {
	lower := strings.ToLower(value)
	switch lower {
	case "true", "1", "yes", "on", "enabled":
		return true, nil
	case "false", "0", "no", "off", "disabled":
		return false, nil
	default:
		return strconv.ParseBool(value)
	}
}

func (c *BoolConverter) Kind() reflect.Kind {
	return reflect.Bool
}

// SliceConverter 切片转换器
type SliceConverter struct{}

func (c *SliceConverter) Convert(value string) (interface{}, error) {
	if value == "" {
		return []string{}, nil
	}

	// 支持逗号、分号、空格分隔
	separators := []string{",", ";", " ", "|"}
	for _, sep := range separators {
		if strings.Contains(value, sep) {
			return strings.Split(value, sep), nil
		}
	}

	return []string{value}, nil
}

func (c *SliceConverter) Kind() reflect.Kind {
	return reflect.Slice
}

// MapConverter 映射转换器
type MapConverter struct{}

func (c *MapConverter) Convert(value string) (interface{}, error) {
	result := make(map[string]string)

	if value == "" {
		return result, nil
	}

	// 支持key=value,key2=value2格式
	pairs := strings.Split(value, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	return result, nil
}

func (c *MapConverter) Kind() reflect.Kind {
	return reflect.Map
}

func parseDurationValue(value string) (time.Duration, error) {
	if duration, err := time.ParseDuration(value); err == nil {
		return duration, nil
	}

	if numeric, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
		return time.Duration(numeric) * time.Second, nil
	}

	return 0, fmt.Errorf("无法解析时间段: %s", value)
}

// DurationConverter 时间段转换器
type DurationConverter struct{}

func (c *DurationConverter) Convert(value string) (interface{}, error) {
	duration, err := parseDurationValue(value)
	if err != nil {
		return nil, err
	}
	return duration, nil
}

func (c *DurationConverter) Kind() reflect.Kind {
	return reflect.Int64 // time.Duration的底层类型
}

// URLTransformer URL转换器
type URLTransformer struct{}

func (t *URLTransformer) Transform(key, value string) (string, string, error) {
	if strings.Contains(strings.ToLower(key), "url") {
		// 验证URL格式
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			if value != "" {
				value = "http://" + value
			}
		}
	}
	return key, value, nil
}

func (t *URLTransformer) Name() string {
	return "URLTransformer"
}

// LowerCaseTransformer 小写转换器
type LowerCaseTransformer struct{}

func (t *LowerCaseTransformer) Transform(key, value string) (string, string, error) {
	if strings.Contains(strings.ToLower(key), "email") ||
		strings.Contains(strings.ToLower(key), "username") {
		return key, strings.ToLower(value), nil
	}
	return key, value, nil
}

func (t *LowerCaseTransformer) Name() string {
	return "LowerCaseTransformer"
}

// TrimSpaceTransformer 去空格转换器
type TrimSpaceTransformer struct{}

func (t *TrimSpaceTransformer) Transform(key, value string) (string, string, error) {
	return key, strings.TrimSpace(value), nil
}

func (t *TrimSpaceTransformer) Name() string {
	return "TrimSpaceTransformer"
}
