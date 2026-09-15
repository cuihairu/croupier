// playersAPI 是一个最小的「游戏方 HTTP 服务」教学实现：
// 内存 players CRUD + kick 行为操作，并在 /openapi.json 导出自身契约。
//
// Agent 侧的 openapi provider 通过 providers.yaml 指向该服务：
//
//	providers:
//	  players:
//	    enabled: true
//	    type: openapi
//	    game_id: default
//	    env: dev
//	    config:
//	      baseUrl: "http://127.0.0.1:8091"
//	      openapiSpec: "http://127.0.0.1:8091/openapi.json"
//
// 之后 Agent 会把文档中的每个 operation 注册为函数
// players.<operationId>（如 players.player.list），Server 目录即可见、可调用。
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type player struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

// playersAPI 持有内存数据。真实游戏方服务替换成自己的存储与业务即可，
// 需要保持不变的只有两件事：
//  1. REST 形状（method + path 结构）——平台用它推导 capability；
//  2. /openapi.json 与实现一致——Agent 注册的函数契约来自它。
type playersAPI struct {
	mu     sync.Mutex
	byID   map[string]*player
	order  []string
	nextID int
}

func newPlayersAPI() *playersAPI {
	api := &playersAPI{
		byID: map[string]*player{
			"1001": {ID: "1001", Name: "Ada", Level: 10},
			"1002": {ID: "1002", Name: "Ben", Level: 20},
			"1003": {ID: "1003", Name: "Cleo", Level: 33},
		},
		order:  []string{"1001", "1002", "1003"},
		nextID: 1004,
	}
	return api
}

func (a *playersAPI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(a.openAPIDoc())
	})
	mux.HandleFunc("GET /players", a.list)
	mux.HandleFunc("POST /players", a.create)
	mux.HandleFunc("GET /players/{id}", a.get)
	mux.HandleFunc("PUT /players/{id}", a.update)
	mux.HandleFunc("DELETE /players/{id}", a.remove)
	mux.HandleFunc("POST /players/{id}/kick", a.kick)
}

// list 响应 GET /players。响应是 {items, total} 且请求带 page/page_size 查询
// 参数时，平台会识别分页语义并自动生成列表页的分页绑定。
func (a *playersAPI) list(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	start := (page - 1) * pageSize
	if start > len(a.order) {
		start = len(a.order)
	}
	end := start + pageSize
	if end > len(a.order) {
		end = len(a.order)
	}
	items := make([]*player, 0, end-start)
	for _, id := range a.order[start:end] {
		items = append(items, a.byID[id])
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(a.order)})
}

func (a *playersAPI) get(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	p, ok := a.byID[r.PathValue("id")]
	a.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "player not found"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *playersAPI) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Level int    `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "name is required"})
		return
	}
	a.mu.Lock()
	id := strconv.Itoa(a.nextID)
	a.nextID++
	p := &player{ID: id, Name: req.Name, Level: req.Level}
	a.byID[id] = p
	a.order = append(a.order, id)
	a.mu.Unlock()
	writeJSON(w, http.StatusCreated, p)
}

func (a *playersAPI) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  *string `json:"name"`
		Level *int    `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": err.Error()})
		return
	}
	a.mu.Lock()
	p, ok := a.byID[r.PathValue("id")]
	if ok {
		if req.Name != nil {
			p.Name = *req.Name
		}
		if req.Level != nil {
			p.Level = *req.Level
		}
	}
	a.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "player not found"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *playersAPI) remove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.mu.Lock()
	_, ok := a.byID[id]
	if ok {
		delete(a.byID, id)
		for i, existing := range a.order {
			if existing == id {
				a.order = append(a.order[:i], a.order[i+1:]...)
				break
			}
		}
	}
	a.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "player not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// kick 响应 POST /players/{id}/kick。不属于 REST 生命周期的形状会被分类为
// action（低置信度），在目录里作为「行为操作」呈现，适合接入行按钮。
func (a *playersAPI) kick(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	_, ok := a.byID[r.PathValue("id")]
	a.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "player not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// openAPIDoc 导出与实现一致的 OpenAPI 3.0 文档。
//
// 编写要点（平台从这些信息推导函数契约与能力语义）：
//   - operationId 即函数名（players.<operationId>），必须稳定——改名等于换函数；
//   - REST 生命周期形状自动推导 capability：
//     GET /players → collection_query、POST /players → create、
//     GET /players/{id} → item_query、PUT /players/{id} → update、
//     DELETE /players/{id} → delete、POST /players/{id}/kick → action；
//   - 列表接口带 page/page_size 参数、返回 {items, total}，即可生成分页；
//   - x-execution 声明执行方式（sync=同步调用、task=异步任务）；
//   - x-risk / x-permission 声明治理字段，进入目录与权限校验；
//   - 只描述 API 契约：页面 schema、菜单、多语言等 UI 信息一律不允许出现。
func (a *playersAPI) openAPIDoc() map[string]any {
	playerSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":    map[string]any{"type": "string"},
			"name":  map[string]any{"type": "string"},
			"level": map[string]any{"type": "integer"},
		},
		"required": []string{"id", "name"},
	}
	playerInput := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string"},
			"level": map[string]any{"type": "integer"},
		},
		"required": []string{"name"},
	}
	jsonContent := func(schema map[string]any) map[string]any {
		return map[string]any{"application/json": map[string]any{"schema": schema}}
	}
	idParam := func() map[string]any {
		return map[string]any{
			"name": "id", "in": "path", "required": true,
			"schema": map[string]any{"type": "string"},
		}
	}
	return map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": "Players Demo API", "version": "1.0.0"},
		"paths": map[string]any{
			"/players": map[string]any{
				"get": map[string]any{
					"operationId": "player.list",
					"summary":     "List players",
					"parameters": []any{
						map[string]any{"name": "page", "in": "query", "schema": map[string]any{"type": "integer"}},
						map[string]any{"name": "page_size", "in": "query", "schema": map[string]any{"type": "integer"}},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK", "content": jsonContent(map[string]any{
							"type": "object",
							"properties": map[string]any{
								"items": map[string]any{"type": "array", "items": playerSchema},
								"total": map[string]any{"type": "integer"},
							},
						})},
					},
				},
				"post": map[string]any{
					"operationId": "player.create",
					"summary":     "Create player",
					"requestBody": map[string]any{"required": true, "content": jsonContent(playerInput)},
					"responses": map[string]any{
						"201": map[string]any{"description": "Created", "content": jsonContent(playerSchema)},
					},
				},
			},
			"/players/{id}": map[string]any{
				"get": map[string]any{
					"operationId": "player.get",
					"summary":     "Get player",
					"parameters":  []any{idParam()},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK", "content": jsonContent(playerSchema)},
					},
				},
				"put": map[string]any{
					"operationId": "player.update",
					"summary":     "Update player",
					"parameters":  []any{idParam()},
					"requestBody": map[string]any{"required": true, "content": jsonContent(playerInput)},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK", "content": jsonContent(playerSchema)},
					},
				},
				"delete": map[string]any{
					"operationId": "player.delete",
					"summary":     "Delete player",
					"parameters":  []any{idParam()},
					"responses": map[string]any{
						"204": map[string]any{"description": "Deleted"},
					},
				},
			},
			"/players/{id}/kick": map[string]any{
				"post": map[string]any{
					"operationId": "player.kick",
					"summary":     "Kick player",
					"description": "Force the player offline (action operation).",
					"parameters":  []any{idParam()},
					"x-execution": "sync",
					"x-risk":      "high",
					"responses": map[string]any{
						"200": map[string]any{"description": "OK", "content": jsonContent(map[string]any{
							"type": "object",
							"properties": map[string]any{
								"success": map[string]any{"type": "boolean"},
							},
						})},
					},
				},
			},
		},
	}
}
