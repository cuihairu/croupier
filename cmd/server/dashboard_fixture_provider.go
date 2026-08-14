package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// fixturePlayer is one deterministic record served by the /players provider.
type fixturePlayer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

// providerCall records one HTTP call received by the /players provider so E2E
// tests can assert that bindings hit the provider with selector-derived input.
type providerCall struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// playersProvider is the deterministic OpenAPI `/players` provider owned by
// the real-dashboard fixture. It serves list/detail/create/update/delete plus
// one row action, and publishes its own OpenAPI document at /openapi.json.
type playersProvider struct {
	mu      sync.Mutex
	players map[string]*fixturePlayer
	order   []string
	nextID  int
	callLog []providerCall
}

func newPlayersProvider() *playersProvider {
	p := &playersProvider{}
	p.reset()
	return p
}

func (p *playersProvider) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.players = map[string]*fixturePlayer{
		"p-001": {ID: "p-001", Name: "Ada", Level: 10},
		"p-002": {ID: "p-002", Name: "Ben", Level: 20},
	}
	p.order = []string{"p-001", "p-002"}
	p.nextID = 3
	p.callLog = nil
}

func (p *playersProvider) calls() []providerCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]providerCall, len(p.callLog))
	copy(out, p.callLog)
	return out
}

func (p *playersProvider) recordCall(method, path string, body json.RawMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callLog = append(p.callLog, providerCall{Method: method, Path: path, Body: body})
}

func (p *playersProvider) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(playersOpenAPIDoc())
	})
	mux.HandleFunc("/players", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			p.listPlayers(w, r)
		case http.MethodPost:
			p.createPlayer(w, r)
		default:
			writeFixtureJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		}
	})
	mux.HandleFunc("/players/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/players/")
		if strings.HasSuffix(rest, "/kick") && r.Method == http.MethodPost {
			p.kickPlayer(w, r, strings.TrimSuffix(rest, "/kick"))
			return
		}
		id := rest
		switch r.Method {
		case http.MethodGet:
			p.getPlayer(w, r, id)
		case http.MethodPut:
			p.updatePlayer(w, r, id)
		case http.MethodDelete:
			p.deletePlayer(w, r, id)
		default:
			writeFixtureJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		}
	})
	return mux
}

func (p *playersProvider) listPlayers(w http.ResponseWriter, r *http.Request) {
	p.recordCall(http.MethodGet, "/players", nil)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}

	p.mu.Lock()
	ids := make([]string, len(p.order))
	copy(ids, p.order)
	total := len(ids)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	items := make([]*fixturePlayer, 0, end-start)
	for _, id := range ids[start:end] {
		items = append(items, p.players[id])
	}
	p.mu.Unlock()

	writeFixtureJSON(w, http.StatusOK, map[string]interface{}{"items": items, "total": total})
}

func (p *playersProvider) getPlayer(w http.ResponseWriter, r *http.Request, id string) {
	p.recordCall(http.MethodGet, "/players/"+id, nil)
	p.mu.Lock()
	player, ok := p.players[id]
	p.mu.Unlock()
	if !ok {
		writeFixtureJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "player not found"})
		return
	}
	writeFixtureJSON(w, http.StatusOK, player)
}

func (p *playersProvider) createPlayer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Level int    `json:"level"`
	}
	body, _ := readBody(r)
	p.recordCall(http.MethodPost, "/players", body)
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeFixtureJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "name is required"})
		return
	}
	p.mu.Lock()
	id := fmt.Sprintf("p-%03d", p.nextID)
	p.nextID++
	player := &fixturePlayer{ID: id, Name: req.Name, Level: req.Level}
	p.players[id] = player
	p.order = append(p.order, id)
	p.mu.Unlock()
	writeFixtureJSON(w, http.StatusCreated, player)
}

func (p *playersProvider) updatePlayer(w http.ResponseWriter, r *http.Request, id string) {
	body, _ := readBody(r)
	p.recordCall(http.MethodPut, "/players/"+id, body)
	p.mu.Lock()
	player, ok := p.players[id]
	p.mu.Unlock()
	if !ok {
		writeFixtureJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "player not found"})
		return
	}
	var req struct {
		Name  *string `json:"name"`
		Level *int    `json:"level"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeFixtureJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": err.Error()})
		return
	}
	p.mu.Lock()
	if req.Name != nil {
		player.Name = *req.Name
	}
	if req.Level != nil {
		player.Level = *req.Level
	}
	p.mu.Unlock()
	writeFixtureJSON(w, http.StatusOK, player)
}

func (p *playersProvider) deletePlayer(w http.ResponseWriter, r *http.Request, id string) {
	p.recordCall(http.MethodDelete, "/players/"+id, nil)
	p.mu.Lock()
	_, ok := p.players[id]
	if ok {
		delete(p.players, id)
		for i, existing := range p.order {
			if existing == id {
				p.order = append(p.order[:i], p.order[i+1:]...)
				break
			}
		}
	}
	p.mu.Unlock()
	if !ok {
		writeFixtureJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "player not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (p *playersProvider) kickPlayer(w http.ResponseWriter, r *http.Request, id string) {
	body, _ := readBody(r)
	p.recordCall(http.MethodPost, "/players/"+id+"/kick", body)
	p.mu.Lock()
	_, ok := p.players[id]
	p.mu.Unlock()
	if !ok {
		writeFixtureJSON(w, http.StatusNotFound, map[string]string{"error": "not_found", "message": "player not found"})
		return
	}
	writeFixtureJSON(w, http.StatusOK, map[string]interface{}{"success": true, "playerId": id})
}

func readBody(r *http.Request) (json.RawMessage, error) {
	if r.Body == nil {
		return nil, nil
	}
	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body, nil
}

// playersOpenAPIDoc describes the provider API. REST semantics
// (collection/item/create/update/delete and the `kick` row action) are
// derived by the platform's controlled REST classifier from method + path
// shape only; no UI extensions are present.
func playersOpenAPIDoc() []byte {
	playerSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":    map[string]interface{}{"type": "string"},
			"name":  map[string]interface{}{"type": "string"},
			"level": map[string]interface{}{"type": "integer"},
		},
		"required": []string{"id", "name"},
	}
	playerInputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":  map[string]interface{}{"type": "string"},
			"level": map[string]interface{}{"type": "integer"},
		},
		"required": []string{"name"},
	}
	jsonContent := func(schema map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"application/json": map[string]interface{}{"schema": schema},
		}
	}
	idParam := map[string]interface{}{
		"name":     "id",
		"in":       "path",
		"required": true,
		"schema":   map[string]interface{}{"type": "string"},
	}
	doc := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Players API", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.list",
					"summary":     "List players",
					"parameters": []map[string]interface{}{
						{"name": "page", "in": "query", "schema": map[string]interface{}{"type": "integer"}},
						{"name": "page_size", "in": "query", "schema": map[string]interface{}{"type": "integer"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OK",
							"content": jsonContent(map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"items": map[string]interface{}{"type": "array", "items": playerSchema},
									"total": map[string]interface{}{"type": "integer"},
								},
							}),
						},
					},
				},
				"post": map[string]interface{}{
					"operationId": "player.create",
					"summary":     "Create player",
					"requestBody": map[string]interface{}{"required": true, "content": jsonContent(playerInputSchema)},
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "Created", "content": jsonContent(playerSchema)},
					},
				},
			},
			"/players/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.get",
					"summary":     "Get player",
					"parameters":  []map[string]interface{}{idParam},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "OK", "content": jsonContent(playerSchema)},
					},
				},
				"put": map[string]interface{}{
					"operationId": "player.update",
					"summary":     "Update player",
					"parameters":  []map[string]interface{}{idParam},
					"requestBody": map[string]interface{}{"required": true, "content": jsonContent(playerInputSchema)},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "OK", "content": jsonContent(playerSchema)},
					},
				},
				"delete": map[string]interface{}{
					"operationId": "player.delete",
					"summary":     "Delete player",
					"parameters":  []map[string]interface{}{idParam},
					"responses": map[string]interface{}{
						"204": map[string]interface{}{"description": "Deleted"},
					},
				},
			},
			"/players/{id}/kick": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId": "player.kick",
					"summary":     "Kick player",
					"parameters":  []map[string]interface{}{idParam},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "OK",
							"content": jsonContent(map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"success":  map[string]interface{}{"type": "boolean"},
									"playerId": map[string]interface{}{"type": "string"},
								},
							}),
						},
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(doc)
	return raw
}
