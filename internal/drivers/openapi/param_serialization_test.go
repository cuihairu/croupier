package openapi

import (
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// serializationCase 是 SUI-006 序列化矩阵的单点用例：给定参数位置/样式/
// explode 与值，断言线上形态（path 段 / query 对 / header / cookie）。
type serializationCase struct {
	name    string
	param   spec.RequestParameterSpec
	value   any
	wantSeg string
	wantQ   []queryPair
	wantHdr string
	wantCk  string
	wantErr string // 子串匹配；空 = 无错误
}

// qp 是 queryPair 构造简写。
func qp(name, value string) queryPair { return queryPair{name: name, value: value} }

func TestSerializeParamValue_Matrix(t *testing.T) {
	cases := []serializationCase{
		// path simple
		{name: "path simple primitive", param: spec.RequestParameterSpec{Name: "id", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "simple", Explode: false}}, value: "5", wantSeg: "5"},
		{name: "path simple array", param: spec.RequestParameterSpec{Name: "ids", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "simple", Explode: false}}, value: []any{"a", "b"}, wantSeg: "a,b"},
		{name: "path simple object noexplode", param: spec.RequestParameterSpec{Name: "u", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "simple", Explode: false}}, value: map[string]any{"id": "1", "tag": "alice"}, wantSeg: "id,1,tag,alice"},
		{name: "path simple object explode", param: spec.RequestParameterSpec{Name: "u", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "simple", Explode: true}}, value: map[string]any{"id": "1", "tag": "alice"}, wantSeg: "id=1,tag=alice"},
		{name: "path simple slash escaped", param: spec.RequestParameterSpec{Name: "id", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "simple", Explode: false}}, value: "a/b", wantSeg: "a%2Fb"},
		{name: "path simple number", param: spec.RequestParameterSpec{Name: "id", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "simple", Explode: false}}, value: float64(5), wantSeg: "5"},

		// path label
		{name: "path label primitive", param: spec.RequestParameterSpec{Name: "ver", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "label", Explode: false}}, value: "1.2", wantSeg: ".1.2"},
		{name: "path label array noexplode", param: spec.RequestParameterSpec{Name: "ids", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "label", Explode: false}}, value: []any{1, 2}, wantSeg: ".1,2"},
		{name: "path label array explode", param: spec.RequestParameterSpec{Name: "ids", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "label", Explode: true}}, value: []any{1, 2, 3}, wantSeg: ".1.2.3"},
		{name: "path label object explode", param: spec.RequestParameterSpec{Name: "u", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "label", Explode: true}}, value: map[string]any{"id": "1", "tag": "alice"}, wantSeg: ".id=1.tag=alice"},

		// path matrix
		{name: "path matrix primitive", param: spec.RequestParameterSpec{Name: "id", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "matrix", Explode: false}}, value: float64(5), wantSeg: ";id=5"},
		{name: "path matrix array noexplode", param: spec.RequestParameterSpec{Name: "ids", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "matrix", Explode: false}}, value: []any{1, 2, 3}, wantSeg: ";ids=1,2,3"},
		{name: "path matrix array explode", param: spec.RequestParameterSpec{Name: "ids", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "matrix", Explode: true}}, value: []any{1, 2}, wantSeg: ";ids=1;ids=2"},
		{name: "path matrix object explode", param: spec.RequestParameterSpec{Name: "keys", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "matrix", Explode: true}}, value: map[string]any{"id": "1", "tag": "alice"}, wantSeg: ";id=1;tag=alice"},

		// query form
		{name: "query form primitive", param: spec.RequestParameterSpec{Name: "q", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "form", Explode: false}}, value: "x", wantQ: []queryPair{qp("q", "x")}},
		{name: "query form array noexplode", param: spec.RequestParameterSpec{Name: "list", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "form", Explode: false}}, value: []any{"a", "b"}, wantQ: []queryPair{qp("list", "a,b")}},
		{name: "query form array explode", param: spec.RequestParameterSpec{Name: "list", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "form", Explode: true}}, value: []any{"a", "b", "c"}, wantQ: []queryPair{qp("list", "a"), qp("list", "b"), qp("list", "c")}},
		{name: "query form object explode", param: spec.RequestParameterSpec{Name: "user", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "form", Explode: true}}, value: map[string]any{"id": "1", "tag": "alice"}, wantQ: []queryPair{qp("id", "1"), qp("tag", "alice")}},
		{name: "query form space escaped", param: spec.RequestParameterSpec{Name: "q", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "form", Explode: false}}, value: "a b&c", wantQ: []queryPair{qp("q", "a+b%26c")}},

		// query spaceDelimited / pipeDelimited
		{name: "query spaced array", param: spec.RequestParameterSpec{Name: "s", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "spaceDelimited", Explode: false}}, value: []any{"a b", "c"}, wantQ: []queryPair{qp("s", "a+b+c")}},
		{name: "query pipe array", param: spec.RequestParameterSpec{Name: "p", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "pipeDelimited", Explode: false}}, value: []any{"x", "y"}, wantQ: []queryPair{qp("p", "x|y")}},
		{name: "query pipe object", param: spec.RequestParameterSpec{Name: "u", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "pipeDelimited", Explode: false}}, value: map[string]any{"id": "1", "tag": "alice"}, wantQ: []queryPair{qp("u", "id|1|tag|alice")}},

		// query deepObject
		{name: "query deepObject object", param: spec.RequestParameterSpec{Name: "meta", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "deepObject", Explode: true}}, value: map[string]any{"a": "1", "b": "2"}, wantQ: []queryPair{qp("meta%5Ba%5D", "1"), qp("meta%5Bb%5D", "2")}},

		// header simple
		{name: "header simple array", param: spec.RequestParameterSpec{Name: "X-Ids", In: spec.ParameterPositionHeader, Serialization: spec.RequestSerialization{Style: "simple", Explode: false}}, value: []any{1, 2, 3}, wantHdr: "1,2,3"},
		{name: "header simple object explode", param: spec.RequestParameterSpec{Name: "X-U", In: spec.ParameterPositionHeader, Serialization: spec.RequestSerialization{Style: "simple", Explode: true}}, value: map[string]any{"id": "1", "tag": "alice"}, wantHdr: "id=1,tag=alice"},

		// cookie form
		{name: "cookie primitive", param: spec.RequestParameterSpec{Name: "token", In: spec.ParameterPositionCookie, Serialization: spec.RequestSerialization{Style: "form", Explode: false}}, value: "8", wantCk: "8"},
		{name: "cookie number", param: spec.RequestParameterSpec{Name: "n", In: spec.ParameterPositionCookie, Serialization: spec.RequestSerialization{Style: "form", Explode: false}}, value: float64(8), wantCk: "8"},

		// unsupported combos -> error
		{name: "path form style invalid", param: spec.RequestParameterSpec{Name: "id", In: spec.ParameterPositionPath, Serialization: spec.RequestSerialization{Style: "form", Explode: false}}, value: "x", wantErr: "not valid for path"},
		{name: "query simple style invalid", param: spec.RequestParameterSpec{Name: "q", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "simple", Explode: false}}, value: "x", wantErr: "not valid for query"},
		{name: "header form style invalid", param: spec.RequestParameterSpec{Name: "H", In: spec.ParameterPositionHeader, Serialization: spec.RequestSerialization{Style: "form", Explode: false}}, value: "x", wantErr: "not valid for header"},
		{name: "cookie array unsupported", param: spec.RequestParameterSpec{Name: "c", In: spec.ParameterPositionCookie, Serialization: spec.RequestSerialization{Style: "form", Explode: false}}, value: []any{1, 2}, wantErr: "cookie parameters only support primitive"},
		{name: "deepObject array unsupported", param: spec.RequestParameterSpec{Name: "m", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "deepObject", Explode: true}}, value: []any{1, 2}, wantErr: "deepObject is only executable for object"},
		{name: "deepObject noexplode unsupported", param: spec.RequestParameterSpec{Name: "m", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "deepObject", Explode: false}}, value: map[string]any{"a": "1"}, wantErr: "deepObject requires explode=true"},
		{name: "nested array value unsupported", param: spec.RequestParameterSpec{Name: "u", In: spec.ParameterPositionQuery, Serialization: spec.RequestSerialization{Style: "form", Explode: false}}, value: map[string]any{"k": []any{1, 2}}, wantErr: "not scalar"},
		{name: "nested object value unsupported", param: spec.RequestParameterSpec{Name: "k", In: spec.ParameterPositionCookie, Serialization: spec.RequestSerialization{Style: "form", Explode: false}}, value: map[string]any{"a": "1"}, wantErr: "cookie parameters only support primitive"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := serializeParamValue(tt.param, tt.value)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("serializeParamValue() = %+v, want error containing %q", got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("serializeParamValue() error = %v", err)
			}
			if tt.wantSeg != "" && got.pathSegment != tt.wantSeg {
				t.Errorf("pathSegment = %q, want %q", got.pathSegment, tt.wantSeg)
			}
			if len(tt.wantQ) > 0 {
				if len(got.queryPairs) != len(tt.wantQ) {
					t.Fatalf("queryPairs = %v, want %v", got.queryPairs, tt.wantQ)
				}
				for i, w := range tt.wantQ {
					if got.queryPairs[i] != w {
						t.Errorf("queryPairs[%d] = %v, want %v", i, got.queryPairs[i], w)
					}
				}
			}
			if tt.wantHdr != "" && got.headerValue != tt.wantHdr {
				t.Errorf("headerValue = %q, want %q", got.headerValue, tt.wantHdr)
			}
			if tt.wantCk != "" && got.cookieValue != tt.wantCk {
				t.Errorf("cookieValue = %q, want %q", got.cookieValue, tt.wantCk)
			}
			if !got.present {
				t.Errorf("present = false, want true")
			}
		})
	}
}
