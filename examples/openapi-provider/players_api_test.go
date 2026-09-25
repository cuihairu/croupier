package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// newTestHandler 构造与 main/run 相同的注册方式（api.Register(mux)），
// 用 httptest 直接驱动，覆盖全部路由与错误面。
func newTestHandler(t *testing.T) (*playersAPI, http.Handler) {
	t.Helper()
	api := newPlayersAPI()
	mux := http.NewServeMux()
	api.Register(mux)
	return api, mux
}

func doRequest(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, reader))
	return rec
}

type listResponse struct {
	Items []player `json:"items"`
	Total int      `json:"total"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func decodeList(t *testing.T, rec *httptest.ResponseRecorder) listResponse {
	t.Helper()
	var out listResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	return out
}

func decodePlayer(t *testing.T, rec *httptest.ResponseRecorder) player {
	t.Helper()
	var out player
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode player response: %v", err)
	}
	return out
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	var out errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return out
}

func playerIDs(items []player) []string {
	ids := make([]string, 0, len(items))
	for _, p := range items {
		ids = append(ids, p.ID)
	}
	return ids
}

func TestPlayersAPI_ListDefaults(t *testing.T) {
	_, h := newTestHandler(t)
	cases := []string{
		"/players",
		"/players?page=0&page_size=0",
		"/players?page=-3&page_size=-7",
		"/players?page=abc&page_size=xyz", // Atoi 失败 → 走默认值
	}
	for _, target := range cases {
		rec := doRequest(t, h, http.MethodGet, target, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: got status %d, want 200", target, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("%s: content-type = %q, want application/json", target, ct)
		}
		got := decodeList(t, rec)
		if got.Total != 3 || len(got.Items) != 3 {
			t.Fatalf("%s: got total=%d items=%d, want total=3 items=3", target, got.Total, len(got.Items))
		}
		if want := []string{"1001", "1002", "1003"}; !reflect.DeepEqual(playerIDs(got.Items), want) {
			t.Fatalf("%s: ids = %v, want %v", target, playerIDs(got.Items), want)
		}
	}
}

func TestPlayersAPI_ListPaginationBounds(t *testing.T) {
	_, h := newTestHandler(t)
	cases := []struct {
		name   string
		target string
		want   []string
	}{
		{"first page full", "/players?page=1&page_size=2", []string{"1001", "1002"}},
		{"end beyond length clamped", "/players?page=2&page_size=2", []string{"1003"}},
		{"start beyond length clamped", "/players?page=3&page_size=2", []string{}},
		{"page size beyond length", "/players?page=1&page_size=100", []string{"1001", "1002", "1003"}},
		{"far page clamped", "/players?page=99&page_size=1", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, h, http.MethodGet, tc.target, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			got := decodeList(t, rec)
			if got.Total != 3 {
				t.Fatalf("total = %d, want 3", got.Total)
			}
			if len(got.Items) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(playerIDs(got.Items), tc.want) {
				t.Fatalf("ids = %v, want %v", playerIDs(got.Items), tc.want)
			}
		})
	}
}

func TestPlayersAPI_Get(t *testing.T) {
	_, h := newTestHandler(t)

	rec := doRequest(t, h, http.MethodGet, "/players/1002", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := decodePlayer(t, rec); got.ID != "1002" || got.Name != "Ben" || got.Level != 20 {
		t.Fatalf("got %+v, want {1002 Ben 20}", got)
	}

	rec = doRequest(t, h, http.MethodGet, "/players/9999", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if errResp := decodeError(t, rec); errResp.Error != "not_found" {
		t.Fatalf("error = %q, want not_found", errResp.Error)
	}
}

func TestPlayersAPI_Create(t *testing.T) {
	_, h := newTestHandler(t)

	t.Run("success", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPost, "/players", `{"name":"Zoe","level":5}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", rec.Code)
		}
		created := decodePlayer(t, rec)
		if created.ID != "1004" || created.Name != "Zoe" || created.Level != 5 {
			t.Fatalf("created = %+v, want {1004 Zoe 5}", created)
		}
		// 新建的玩家进入列表尾部
		list := decodeList(t, doRequest(t, h, http.MethodGet, "/players", ""))
		if list.Total != 4 || len(list.Items) != 4 {
			t.Fatalf("total = %d items = %d, want 4/4", list.Total, len(list.Items))
		}
		if last := list.Items[3]; last.ID != "1004" || last.Name != "Zoe" {
			t.Fatalf("last item = %+v, want {1004 Zoe 5}", last)
		}
	})

	bad := []struct {
		name string
		body string
	}{
		{"empty name", `{"level":1}`},
		{"blank name", `{"name":"   ","level":1}`},
		{"malformed json", `{"name":`},
		{"empty body", ""},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, h, http.MethodPost, "/players", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if errResp := decodeError(t, rec); errResp.Error != "invalid_request" || errResp.Message == "" {
				t.Fatalf("error = %+v, want invalid_request with message", errResp)
			}
		})
	}
}

func TestPlayersAPI_Update(t *testing.T) {
	_, h := newTestHandler(t)

	t.Run("name only", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPut, "/players/1001", `{"name":"Ada Lovelace"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		got := decodePlayer(t, rec)
		if got.Name != "Ada Lovelace" || got.Level != 10 {
			t.Fatalf("got %+v, want name changed and level untouched (10)", got)
		}
	})

	t.Run("level only", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPut, "/players/1001", `{"level":42}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		got := decodePlayer(t, rec)
		if got.Level != 42 || got.Name != "Ada Lovelace" {
			t.Fatalf("got %+v, want level 42 with previous name", got)
		}
	})

	t.Run("both fields", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPut, "/players/1001", `{"name":"Ada2","level":7}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		got := decodePlayer(t, rec)
		if got.Name != "Ada2" || got.Level != 7 {
			t.Fatalf("got %+v, want {1001 Ada2 7}", got)
		}
	})

	t.Run("no fields", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPut, "/players/1001", `{}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		got := decodePlayer(t, rec)
		if got.Name != "Ada2" || got.Level != 7 {
			t.Fatalf("got %+v, want unchanged {1001 Ada2 7}", got)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPut, "/players/1001", `{not json`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if errResp := decodeError(t, rec); errResp.Error != "invalid_request" || errResp.Message == "" {
			t.Fatalf("error = %+v, want invalid_request with message", errResp)
		}
	})

	t.Run("missing player", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodPut, "/players/9999", `{"name":"x"}`)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		if errResp := decodeError(t, rec); errResp.Error != "not_found" {
			t.Fatalf("error = %q, want not_found", errResp.Error)
		}
	})
}

func TestPlayersAPI_Remove(t *testing.T) {
	_, h := newTestHandler(t)

	t.Run("remove middle keeps order", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodDelete, "/players/1002", "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		list := decodeList(t, doRequest(t, h, http.MethodGet, "/players", ""))
		if list.Total != 2 {
			t.Fatalf("total = %d, want 2", list.Total)
		}
		if want := []string{"1001", "1003"}; !reflect.DeepEqual(playerIDs(list.Items), want) {
			t.Fatalf("ids = %v, want %v", playerIDs(list.Items), want)
		}
	})

	t.Run("remove head", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodDelete, "/players/1001", "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		list := decodeList(t, doRequest(t, h, http.MethodGet, "/players", ""))
		if want := []string{"1003"}; !reflect.DeepEqual(playerIDs(list.Items), want) {
			t.Fatalf("ids = %v, want %v", playerIDs(list.Items), want)
		}
	})

	t.Run("remove tail", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodDelete, "/players/1003", "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		list := decodeList(t, doRequest(t, h, http.MethodGet, "/players", ""))
		if list.Total != 0 || len(list.Items) != 0 {
			t.Fatalf("total = %d items = %d, want empty list", list.Total, len(list.Items))
		}
	})

	t.Run("remove missing", func(t *testing.T) {
		rec := doRequest(t, h, http.MethodDelete, "/players/9999", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		if errResp := decodeError(t, rec); errResp.Error != "not_found" {
			t.Fatalf("error = %q, want not_found", errResp.Error)
		}
	})

	t.Run("remove twice", func(t *testing.T) {
		// 已删除的 id 再删一次 → 404（此时 byID 与 order 都已不含该 id）
		rec := doRequest(t, h, http.MethodDelete, "/players/1003", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
}

func TestPlayersAPI_Kick(t *testing.T) {
	_, h := newTestHandler(t)

	rec := doRequest(t, h, http.MethodPost, "/players/1001/kick", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode kick response: %v", err)
	}
	if success, ok := body["success"].(bool); !ok || !success {
		t.Fatalf("body = %v, want {success: true}", body)
	}

	rec = doRequest(t, h, http.MethodPost, "/players/9999/kick", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if errResp := decodeError(t, rec); errResp.Error != "not_found" {
		t.Fatalf("error = %q, want not_found", errResp.Error)
	}
}

// TestRegister_Routes 从 mux 层验证 Register 注册的 7 条路由形状
// （method + path 与 openapi 契约一致）。
func TestRegister_Routes(t *testing.T) {
	type routeCase struct {
		method string
		target string
		want   int
	}
	cases := []routeCase{
		{http.MethodGet, "/openapi.json", http.StatusOK},
		{http.MethodGet, "/players", http.StatusOK},
		{http.MethodPost, "/players", http.StatusCreated},
		{http.MethodGet, "/players/1001", http.StatusOK},
		{http.MethodPut, "/players/1001", http.StatusOK},
		{http.MethodDelete, "/players/1001", http.StatusNoContent},
		{http.MethodPost, "/players/1001/kick", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			_, h := newTestHandler(t)
			body := ""
			if tc.method == http.MethodPost && tc.target == "/players" {
				body = `{"name":"Rex"}`
			}
			if tc.method == http.MethodPut {
				body = `{"level":9}`
			}
			rec := doRequest(t, h, tc.method, tc.target, body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}

	t.Run("method not allowed", func(t *testing.T) {
		_, h := newTestHandler(t)
		if rec := doRequest(t, h, http.MethodDelete, "/players", ""); rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
	})
}

// TestRegister_OpenAPIJSON 验证 /openapi.json 的 wire 形状与 openAPIDoc() 完全一致。
func TestRegister_OpenAPIJSON(t *testing.T) {
	api, h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/openapi.json", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json", ct)
	}

	var decoded map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode openapi doc: %v", err)
	}
	// 文档内容与实现保持一致：直接比较重新序列化后的 JSON（map 键已排序）。
	want, err := json.Marshal(api.openAPIDoc())
	if err != nil {
		t.Fatalf("marshal expected doc: %v", err)
	}
	got, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal served doc: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("served doc mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func mustMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T (%v)", v, v)
	}
	return m
}

func mustSlice(t *testing.T, v any) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T (%v)", v, v)
	}
	return s
}

// mustStrings 兼容两种形态：原生 openAPIDoc 的 []string 与 JSON 反序列化后的 []any。
func mustStrings(t *testing.T, v any) []string {
	t.Helper()
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			out = append(out, mustString(t, e))
		}
		return out
	default:
		t.Fatalf("expected []string or []any, got %T (%v)", v, v)
		return nil
	}
}

func mustString(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected string, got %T (%v)", v, v)
	}
	return s
}

// TestOpenAPIDoc_Contracts 覆盖平台依赖的关键契约：operationId（函数名）、
// capability 推导所需的 REST 形状、分页参数、x-execution / x-risk 治理字段，
// 以及「只描述 API 契约、不出现 UI 信息」的边界。
func TestOpenAPIDoc_Contracts(t *testing.T) {
	doc := newPlayersAPI().openAPIDoc()

	if got := mustString(t, doc["openapi"]); got != "3.0.3" {
		t.Fatalf("openapi = %q, want 3.0.3", got)
	}
	info := mustMap(t, doc["info"])
	if mustString(t, info["title"]) != "Players Demo API" || mustString(t, info["version"]) != "1.0.0" {
		t.Fatalf("info = %v, want Players Demo API/1.0.0", info)
	}

	// 顶层只允许契约键：页面 schema、菜单、多语言等 UI 信息一律不允许出现。
	allowedTop := map[string]bool{"openapi": true, "info": true, "paths": true}
	for k := range doc {
		if !allowedTop[k] {
			t.Fatalf("unexpected top-level key %q (UI info is not allowed in the API contract)", k)
		}
	}

	paths := mustMap(t, doc["paths"])
	if len(paths) != 3 {
		t.Fatalf("paths = %v, want 3 entries", mapKeys(paths))
	}
	for _, p := range []string{"/players", "/players/{id}", "/players/{id}/kick"} {
		if _, ok := paths[p]; !ok {
			t.Fatalf("missing path %q", p)
		}
	}

	wantOps := map[string]string{
		"GET /players":            "player.list",
		"POST /players":           "player.create",
		"GET /players/{id}":       "player.get",
		"PUT /players/{id}":       "player.update",
		"DELETE /players/{id}":    "player.delete",
		"POST /players/{id}/kick": "player.kick",
	}
	seen := map[string]bool{}
	for key, wantID := range wantOps {
		parts := strings.SplitN(key, " ", 2)
		op := mustMap(t, mustMap(t, paths[parts[1]])[strings.ToLower(parts[0])])
		if got := mustString(t, op["operationId"]); got != wantID {
			t.Fatalf("%s operationId = %q, want %q", key, got, wantID)
		}
		seen[wantID] = true
		if mustString(t, op["summary"]) == "" {
			t.Fatalf("%s summary is empty", key)
		}
	}
	if len(seen) != 6 {
		t.Fatalf("operationIds = %v, want 6 unique ids", seen)
	}

	// GET /players：分页参数（page/page_size）+ {items,total} 响应
	listOp := mustMap(t, mustMap(t, paths["/players"])["get"])
	params := mustSlice(t, listOp["parameters"])
	if len(params) != 2 {
		t.Fatalf("list parameters = %v, want page+page_size", params)
	}
	var paramNames []string
	for _, raw := range params {
		p := mustMap(t, raw)
		paramNames = append(paramNames, mustString(t, p["name"]))
		if mustString(t, p["in"]) != "query" {
			t.Fatalf("param %v in = %q, want query", p, mustString(t, p["in"]))
		}
		if mustMap(t, p["schema"])["type"] != "integer" {
			t.Fatalf("param %v schema = %v, want integer", p, p["schema"])
		}
	}
	if !reflect.DeepEqual(paramNames, []string{"page", "page_size"}) {
		t.Fatalf("params = %v, want [page page_size]", paramNames)
	}
	listOK := mustMap(t, mustMap(t, listOp["responses"])["200"])
	listSchema := mustMap(t, mustMap(t, mustMap(t, listOK["content"])["application/json"])["schema"])
	props := mustMap(t, listSchema["properties"])
	items := mustMap(t, props["items"])
	if items["type"] != "array" {
		t.Fatalf("items type = %v, want array", items["type"])
	}
	if _, ok := props["total"]; !ok {
		t.Fatalf("list response schema missing total: %v", props)
	}
	itemSchema := mustMap(t, items["items"])
	if itemSchema["type"] != "object" {
		t.Fatalf("item schema type = %v, want object", itemSchema["type"])
	}
	if req := mustStrings(t, itemSchema["required"]); !reflect.DeepEqual(req, []string{"id", "name"}) {
		t.Fatalf("item required = %v, want [id name]", req)
	}
	itemProps := mustMap(t, itemSchema["properties"])
	if got := mustMap(t, itemProps["id"])["type"]; got != "string" {
		t.Fatalf("id type = %v, want string", got)
	}
	if got := mustMap(t, itemProps["level"])["type"]; got != "integer" {
		t.Fatalf("level type = %v, want integer", got)
	}

	// 路径参数 id：GET/PUT/DELETE /players/{id} 与 kick 都必须声明
	for _, key := range []string{"GET /players/{id}", "PUT /players/{id}", "DELETE /players/{id}", "POST /players/{id}/kick"} {
		parts := strings.SplitN(key, " ", 2)
		op := mustMap(t, mustMap(t, paths[parts[1]])[strings.ToLower(parts[0])])
		ps := mustSlice(t, op["parameters"])
		if len(ps) != 1 {
			t.Fatalf("%s parameters = %v, want single id path param", key, ps)
		}
		p := mustMap(t, ps[0])
		if mustString(t, p["name"]) != "id" || mustString(t, p["in"]) != "path" || p["required"] != true {
			t.Fatalf("%s param = %v, want required path id", key, p)
		}
	}

	// 治理字段：kick 是行为操作（action），声明执行方式与风险等级
	kick := mustMap(t, mustMap(t, paths["/players/{id}/kick"])["post"])
	if got := mustString(t, kick["x-execution"]); got != "sync" {
		t.Fatalf("kick x-execution = %q, want sync", got)
	}
	if got := mustString(t, kick["x-risk"]); got != "high" {
		t.Fatalf("kick x-risk = %q, want high", got)
	}
	if mustString(t, kick["description"]) == "" {
		t.Fatal("kick description is empty")
	}
	kickOK := mustMap(t, mustMap(t, kick["responses"])["200"])
	kickSchema := mustMap(t, mustMap(t, mustMap(t, kickOK["content"])["application/json"])["schema"])
	if got := mustMap(t, mustMap(t, kickSchema["properties"])["success"])["type"]; got != "boolean" {
		t.Fatalf("kick success type = %v, want boolean", got)
	}

	// 生命周期状态码：create 201、delete 204（平台据此推导 capability）
	wantStatus := map[string][]string{
		"GET /players":            {"200"},
		"POST /players":           {"201"},
		"GET /players/{id}":       {"200"},
		"PUT /players/{id}":       {"200"},
		"DELETE /players/{id}":    {"204"},
		"POST /players/{id}/kick": {"200"},
	}
	for key, want := range wantStatus {
		parts := strings.SplitN(key, " ", 2)
		op := mustMap(t, mustMap(t, paths[parts[1]])[strings.ToLower(parts[0])])
		responses := mustMap(t, op["responses"])
		var got []string
		for code := range responses {
			got = append(got, code)
		}
		sortStrings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s responses = %v, want %v", key, got, want)
		}
	}

	// 请求体：create/update 必须声明 name 必填
	for _, key := range []string{"POST /players", "PUT /players/{id}"} {
		parts := strings.SplitN(key, " ", 2)
		op := mustMap(t, mustMap(t, paths[parts[1]])[strings.ToLower(parts[0])])
		body := mustMap(t, op["requestBody"])
		if body["required"] != true {
			t.Fatalf("%s requestBody = %v, want required", key, body)
		}
		schema := mustMap(t, mustMap(t, mustMap(t, body["content"])["application/json"])["schema"])
		if req := mustStrings(t, schema["required"]); !reflect.DeepEqual(req, []string{"name"}) {
			t.Fatalf("%s required = %v, want [name]", key, req)
		}
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(s []string) {
	sort.Strings(s)
}
