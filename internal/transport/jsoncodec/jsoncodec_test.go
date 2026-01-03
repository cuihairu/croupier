package jsoncodec

import (
	"encoding/json"
	"math"
	"testing"

	"google.golang.org/grpc/encoding"
)

// TestCodec_Name 测试编解码器名称
func TestCodec_Name(t *testing.T) {
	var c codec
	name := c.Name()
	if name != Name {
		t.Errorf("Name() returned %q, want %q", name, Name)
	}
}

// TestCodec_Marshal 测试序列化
func TestCodec_Marshal(t *testing.T) {
	var c codec

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name:    "字符串",
			input:   "hello",
			wantErr: false,
		},
		{
			name:    "整数",
			input:   42,
			wantErr: false,
		},
		{
			name:    "浮点数",
			input:   3.14,
			wantErr: false,
		},
		{
			name:    "布尔值 true",
			input:   true,
			wantErr: false,
		},
		{
			name:    "布尔值 false",
			input:   false,
			wantErr: false,
		},
		{
			name:    "nil",
			input:   nil,
			wantErr: false,
		},
		{
			name:    "map",
			input:   map[string]string{"key": "value"},
			wantErr: false,
		},
		{
			name:    "slice",
			input:   []int{1, 2, 3},
			wantErr: false,
		},
		{
			name:    "结构体",
			input:   struct{ Name string }{"John"},
			wantErr: false,
		},
		{
			name:    "空 map",
			input:   map[string]string{},
			wantErr: false,
		},
		{
			name:    "空 slice",
			input:   []int{},
			wantErr: false,
		},
		{
			name:    "最大浮点数",
			input:   math.MaxFloat64,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := c.Marshal(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && data == nil {
				t.Error("Marshal() should return data for valid input")
			}

			// 验证是有效的 JSON
			if !tt.wantErr && data != nil {
				var js json.RawMessage
				if err := json.Unmarshal(data, &js); err != nil {
					t.Errorf("Marshal() returned invalid JSON: %v", err)
				}
			}
		})
	}
}

// TestCodec_Unmarshal 测试反序列化
func TestCodec_Unmarshal(t *testing.T) {
	var c codec

	// 先序列化一些测试数据
	testData := map[string]interface{}{
		"name":   "John Doe",
		"age":    30,
		"active": true,
	}
	marshaled, _ := json.Marshal(testData)

	tests := []struct {
		name    string
		data    []byte
		target  interface{}
		wantErr bool
	}{
		{
			name:    "解析到 map",
			data:    []byte(`{"name":"John","age":30}`),
			target:  new(map[string]interface{}),
			wantErr: false,
		},
		{
			name:    "解析到结构体",
			data:    []byte(`{"Name":"John"}`),
			target:  new(struct{ Name string }),
			wantErr: false,
		},
		{
			name:    "解析空对象",
			data:    []byte(`{}`),
			target:  new(map[string]string),
			wantErr: false,
		},
		{
			name:    "解析数组",
			data:    []byte(`[1,2,3]`),
			target:  new([]int),
			wantErr: false,
		},
		{
			name:    "解析字符串",
			data:    []byte(`"hello"`),
			target:  new(string),
			wantErr: false,
		},
		{
			name:    "解析数字",
			data:    []byte(`42`),
			target:  new(int),
			wantErr: false,
		},
		{
			name:    "解析布尔值",
			data:    []byte(`true`),
			target:  new(bool),
			wantErr: false,
		},
		{
			name:    "无效的 JSON",
			data:    []byte(`{invalid}`),
			target:  new(map[string]interface{}),
			wantErr: true,
		},
		{
			name:    "空数据",
			data:    []byte{},
			target:  new(map[string]interface{}),
			wantErr: true,
		},
		{
			name:    "null 数据",
			data:    []byte(`null`),
			target:  new(map[string]interface{}),
			wantErr: false,
		},
		{
			name:    "原始序列化数据",
			data:    marshaled,
			target:  new(map[string]interface{}),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Unmarshal(tt.data, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCodec_MarshalUnmarshal_RoundTrip 测试序列化反序列化往返
func TestCodec_MarshalUnmarshal_RoundTrip(t *testing.T) {
	var c codec

	tests := []interface{}{
		"hello",
		42,
		3.14,
		true,
		false,
		[]int{1, 2, 3, 4, 5},
		map[string]string{"key": "value"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			// 序列化
			data, err := c.Marshal(tt)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// 反序列化
			var result interface{}
			err = c.Unmarshal(data, &result)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			// 验证结果（使用 JSON 比较）
			expectedJSON, _ := json.Marshal(tt)
			resultJSON, _ := json.Marshal(result)

			if string(expectedJSON) != string(resultJSON) {
				t.Errorf("RoundTrip mismatch:\nOriginal: %s\nResult:   %s",
					string(expectedJSON), string(resultJSON))
			}
		})
	}
}

// TestCodec_Marshal_Pointers 测试指针类型序列化
func TestCodec_Marshal_Pointers(t *testing.T) {
	var c codec

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name:    "字符串指针",
			input:   func() *string { s := "hello"; return &s }(),
			wantErr: false,
		},
		{
			name:    "整数指针",
			input:   func() *int { i := 42; return &i }(),
			wantErr: false,
		},
		{
			name:    "nil 指针",
			input:   (*string)(nil),
			wantErr: false,
		},
		{
			name:    "最大浮点数",
			input:   math.MaxFloat64,
			wantErr: false,
		},
		{
			name:    "最小非零浮点数",
			input:   math.SmallestNonzeroFloat64,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Marshal(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCodec_NameConstant 测试名称常量
func TestCodec_NameConstant(t *testing.T) {
	if Name != "json" {
		t.Errorf("Name constant should be 'json', got %q", Name)
	}

	var c codec
	if c.Name() != Name {
		t.Errorf("codec.Name() should return constant Name, got %q", c.Name())
	}
}

// TestCodec_EmptySlice 测试空切片
func TestCodec_EmptySlice(t *testing.T) {
	var c codec

	data, err := c.Marshal([]int{})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var result []int
	err = c.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty slice, got %d elements", len(result))
	}
}

// TestCodec_NestedStructures 测试嵌套结构
func TestCodec_NestedStructures(t *testing.T) {
	var c codec

	type Address struct {
		City    string `json:"city"`
		Country string `json:"country"`
	}

	type Person struct {
		Name    string  `json:"name"`
		Age     int     `json:"age"`
		Address Address `json:"address"`
	}

	person := Person{
		Name: "John Doe",
		Age:  30,
		Address: Address{
			City:    "New York",
			Country: "USA",
		},
	}

	// 序列化
	data, err := c.Marshal(person)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	// 反序列化
	var result Person
	err = c.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// 验证数据
	if result.Name != person.Name {
		t.Errorf("Name mismatch: got %q, want %q", result.Name, person.Name)
	}
	if result.Age != person.Age {
		t.Errorf("Age mismatch: got %d, want %d", result.Age, person.Age)
	}
	if result.Address.City != person.Address.City {
		t.Errorf("City mismatch: got %q, want %q", result.Address.City, person.Address.City)
	}
}

// TestCodec_ArrayTypes 测试数组类型
func TestCodec_ArrayTypes(t *testing.T) {
	var c codec

	tests := []struct {
		name  string
		input interface{}
	}{
		{"字符串数组", []string{"a", "b", "c"}},
		{"整数数组", [3]int{1, 2, 3}},
		{"浮点数数组", []float64{1.1, 2.2, 3.3}},
		{"布尔数组", []bool{true, false, true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := c.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// 验证是有效的 JSON
			var js json.RawMessage
			if err := json.Unmarshal(data, &js); err != nil {
				t.Errorf("Marshal() returned invalid JSON: %v", err)
			}
		})
	}
}

// BenchmarkCodec_Marshal 性能基准测试 - 序列化
func BenchmarkCodec_Marshal(b *testing.B) {
	var c codec
	data := map[string]interface{}{
		"name":  "John Doe",
		"age":   30,
		"email": "john@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Marshal(data)
	}
}

// BenchmarkCodec_Unmarshal 性能基准测试 - 反序列化
func BenchmarkCodec_Unmarshal(b *testing.B) {
	var c codec
	jsonData := []byte(`{"name":"John Doe","age":30,"email":"john@example.com"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]interface{}
		c.Unmarshal(jsonData, &result)
	}
}

// BenchmarkCodec_RoundTrip 性能基准测试 - 往返
func BenchmarkCodec_RoundTrip(b *testing.B) {
	var c codec
	data := struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email"`
	}{
		Name:  "John Doe",
		Age:   30,
		Email: "john@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes, _ := c.Marshal(data)
		var result struct {
			Name  string `json:"name"`
			Age   int    `json:"age"`
			Email string `json:"email"`
		}
		c.Unmarshal(bytes, &result)
	}
}

// TestCodec_LargeData 测试大数据
func TestCodec_LargeData(t *testing.T) {
	var c codec

	// 创建大型切片
	largeSlice := make([]int, 10000)
	for i := range largeSlice {
		largeSlice[i] = i
	}

	data, err := c.Marshal(largeSlice)
	if err != nil {
		t.Fatalf("Marshal() error for large data: %v", err)
	}

	var result []int
	err = c.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal() error for large data: %v", err)
	}

	if len(result) != 10000 {
		t.Errorf("Large data length mismatch: got %d, want 10000", len(result))
	}
}

// TestCodec_Unicode 测试 Unicode 字符
func TestCodec_Unicode(t *testing.T) {
	var c codec

	tests := []struct {
		name  string
		input interface{}
	}{
		{"中文字符串", "你好世界"},
		{"Emoji", "😀🎉🚀"},
		{"混合", "Hello 世界 🌍"},
		{"特殊字符", "©®™™"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := c.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Marshal() error: %v", err)
			}

			var result string
			err = c.Unmarshal(data, &result)
			if err != nil {
				t.Fatalf("Unmarshal() error: %v", err)
			}

			if result != tt.input {
				t.Errorf("Unicode roundtrip mismatch: got %q, want %q", result, tt.input)
			}
		})
	}
}

// TestInit 测试包初始化
func TestInit(t *testing.T) {
	// 验证编解码器已注册
	codec := encoding.GetCodec(Name)
	if codec == nil {
		t.Error("JSON codec should be registered via init()")
	}
}
