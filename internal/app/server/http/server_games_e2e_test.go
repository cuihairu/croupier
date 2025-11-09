package httpserver

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strconv"
    "testing"

    "github.com/cuihairu/croupier/internal/security/rbac"
    jwt "github.com/cuihairu/croupier/internal/security/token"
    repogames "github.com/cuihairu/croupier/internal/repo/gorm/games"
    gamesvc "github.com/cuihairu/croupier/internal/service/games"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

// Test create game -> defaults applied -> CRUD envs
func TestGamesE2E_DefaultAndCRUD(t *testing.T) {
    // DB + repos
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil { t.Fatalf("open sqlite: %v", err) }
    if err := repogames.AutoMigrate(db); err != nil { t.Fatalf("migrate: %v", err) }
    s := &Server{}
    s.gdb = db
    s.games = repogames.NewRepo(db)
    // Service with defaults from repo configs
    defs, err := gamesvc.LoadDefaultsFromFile("../../../../configs/games.json")
    if err != nil { t.Fatalf("load defaults: %v", err) }
    s.gamesSvc = gamesvc.NewService(repogames.NewPortRepo(s.games), defs)
    // RBAC by require(): grant games:manage/read to user u1
    pol := rbac.NewPolicy()
    pol.Grant("user:u1", "games:manage")
    pol.Grant("user:u1", "games:read")
    s.rbac = pol
    s.jwtMgr = jwt.NewManager("test")
    tok, _ := s.jwtMgr.Sign("u1", nil, 0)
    r := s.ginEngine()

    // 1) Create game
    req := httptest.NewRequest(http.MethodPost, "/api/games", toJSON(map[string]any{"name": "e2e"}))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != http.StatusCreated { t.Fatalf("create game expected 201, got %d: %s", w.Code, w.Body.String()) }
    var resp map[string]any
    _ = json.Unmarshal(w.Body.Bytes(), &resp)
    id := uint(asInt(resp["id"]))
    if id == 0 { t.Fatalf("missing id in response") }

    // 2) Defaults applied
    req2 := httptest.NewRequest(http.MethodGet, "/api/games/"+strconv.Itoa(int(id)), nil)
    req2.Header.Set("Authorization", "Bearer "+tok)
    w2 := httptest.NewRecorder()
    r.ServeHTTP(w2, req2)
    if w2.Code != http.StatusOK { t.Fatalf("get game expected 200, got %d: %s", w2.Code, w2.Body.String()) }
    var ginfo map[string]any
    _ = json.Unmarshal(w2.Body.Bytes(), &ginfo)
    envs := arrString(ginfo["envs"])
    if len(envs) == 0 { t.Fatalf("expected default envs, got none") }

    // 3) Add env
    body3 := toJSON(map[string]any{"env": "staging", "description": "预发布", "color": "#faad14"})
    req3 := httptest.NewRequest(http.MethodPost, "/api/games/"+strconv.Itoa(int(id))+"/envs", body3)
    req3.Header.Set("Authorization", "Bearer "+tok)
    req3.Header.Set("Content-Type", "application/json")
    w3 := httptest.NewRecorder()
    r.ServeHTTP(w3, req3)
    if w3.Code != http.StatusNoContent { t.Fatalf("add env expected 204, got %d: %s", w3.Code, w3.Body.String()) }

    // 4) Rename env staging->preprod
    body4 := toJSON(map[string]any{"old_env": "staging", "env": "preprod", "color": "#722ed1"})
    req4 := httptest.NewRequest(http.MethodPut, "/api/games/"+strconv.Itoa(int(id))+"/envs", body4)
    req4.Header.Set("Authorization", "Bearer "+tok)
    req4.Header.Set("Content-Type", "application/json")
    w4 := httptest.NewRecorder()
    r.ServeHTTP(w4, req4)
    if w4.Code != http.StatusNoContent { t.Fatalf("update env expected 204, got %d: %s", w4.Code, w4.Body.String()) }

    // 5) Delete env preprod
    req5 := httptest.NewRequest(http.MethodDelete, "/api/games/"+strconv.Itoa(int(id))+"/envs?env=preprod", nil)
    req5.Header.Set("Authorization", "Bearer "+tok)
    w5 := httptest.NewRecorder()
    r.ServeHTTP(w5, req5)
    if w5.Code != http.StatusNoContent { t.Fatalf("delete env expected 204, got %d: %s", w5.Code, w5.Body.String()) }
}

func toJSON(v any) *bytes.Buffer {
    b, _ := json.Marshal(v)
    return bytes.NewBuffer(b)
}
func asInt(v any) int {
    switch x := v.(type) {
    case float64:
        return int(x)
    case json.Number:
        i, _ := x.Int64(); return int(i)
    case string:
        var i int; _, _ = fmt.Sscanf(x, "%d", &i); return i
    default:
        return 0
    }
}
func arrString(v any) []string {
    out := []string{}
    if arr, ok := v.([]any); ok {
        for _, it := range arr { if s, ok := it.(string); ok { out = append(out, s) } }
    }
    return out
}
