package generator

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// FuzzRequiredProperties 夜间 fuzz 入口（CR-FIX-25）：任意字节喂
// requiredProperties，断言非法 JSON 恒返回空、合法输入不 panic 且返回键
// 均来自输入字节（panic/越界类缺陷由 fuzz 直接暴露）。
func FuzzRequiredProperties(f *testing.F) {
	f.Add([]byte(`{"required":["a","b"]}`))
	f.Add([]byte(`{"required":["uid",true,"name"]}`))
	f.Add([]byte(`{"required":"nope"}`))
	f.Add([]byte(`{"required":null}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, schema []byte) {
		out := requiredProperties(spec.JSONSchema(schema))
		if !json.Valid(schema) {
			if len(out) != 0 {
				t.Fatalf("invalid JSON schema must yield empty result, got %v", out)
			}
			return
		}
		for k := range out {
			// encoding/json 对非法 UTF-8 字节的标准强制行为：解码值中以
			// U+FFFD 出现，原始字节里没有对应序列——含替换符的键豁免
			// 出现性检查（testdata 反例 189a00228b478025 固化该行为）。
			if strings.ContainsRune(k, utf8.RuneError) {
				continue
			}
			if !bytes.Contains(schema, []byte(k)) {
				t.Fatalf("returned key %q does not occur in schema bytes", k)
			}
		}
	})
}
