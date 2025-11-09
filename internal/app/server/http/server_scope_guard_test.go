package httpserver

import (
    "bytes"
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    repogames "github.com/cuihairu/croupier/internal/repo/gorm/games"
    usersgorm "github.com/cuihairu/croupier/internal/repo/gorm/users"
    jwt "github.com/cuihairu/croupier/internal/security/token"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupScopeServer(t *testing.T) (*Server, *repogames.Repo, *usersgorm.Repo) {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil { t.Fatalf("open sqlite: %v", err) }
    if err := repogames.AutoMigrate(db); err != nil { t.Fatalf("migrate games: %v", err) }
    if err := usersgorm.AutoMigrate(db); err != nil { t.Fatalf("migrate users: %v", err) }
    s := &Server{}
    s.gdb = db
    grepo := repogames.NewRepo(db)
    s.games = grepo
    urepo := usersgorm.New(db)
    s.userRepo = urepo
    s.jwtMgr = jwt.NewManager("test")
    return s, grepo, urepo
}

// Game denied when X-Game-ID not within user's game scopes
func TestScopeGuard_GameDenied(t *testing.T) {
    s, grepo, urepo := setupScopeServer(t)
    // create game g1 and g2
    g1 := &repogames.Game{Name: "g1", Enabled: true}
    _ = grepo.Create(context.Background(), g1)
    g2 := &repogames.Game{Name: "g2", Enabled: true}
    _ = grepo.Create(context.Background(), g2)
    // user u1 scoped only to g1
    u := &usersgorm.UserAccount{Username: "u1", Active: true}
    _ = urepo.CreateUser(context.Background(), u)
    _ = urepo.ReplaceUserGameIDs(context.Background(), u.ID, []uint{g1.ID})
    // call /api/invoke with X-Game-ID=g2 (denied)
    tok, _ := s.jwtMgr.Sign("u1", []string{"developer"}, 0)
    r := s.ginEngine()
    req := httptest.NewRequest(http.MethodPost, "/api/invoke", bytes.NewBufferString(`{"function_id":"f1"}`))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Game-ID", "g2") // name lookup supported by scope guard
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != http.StatusForbidden {
        t.Fatalf("expected 403 for game scope denied, got %d (%s)", w.Code, w.Body.String())
    }
}

// Env denied when X-Env not in user's env scopes for the game
func TestScopeGuard_EnvDenied(t *testing.T) {
    s, grepo, urepo := setupScopeServer(t)
    // create game g1 with envs prod,test
    g1 := &repogames.Game{Name: "g1", Enabled: true}
    _ = grepo.Create(context.Background(), g1)
    _ = grepo.AddEnvWithMeta(context.Background(), g1.ID, "prod", "", "")
    _ = grepo.AddEnvWithMeta(context.Background(), g1.ID, "test", "", "")
    // user u1 scoped to g1 and env only prod
    u := &usersgorm.UserAccount{Username: "u1", Active: true}
    _ = urepo.CreateUser(context.Background(), u)
    _ = urepo.ReplaceUserGameIDs(context.Background(), u.ID, []uint{g1.ID})
    _ = urepo.ReplaceUserGameEnvs(context.Background(), u.ID, g1.ID, []string{"prod"})
    // call /api/invoke with X-Game-ID=g1 and X-Env=test (denied)
    tok, _ := s.jwtMgr.Sign("u1", []string{"developer"}, 0)
    r := s.ginEngine()
    req := httptest.NewRequest(http.MethodPost, "/api/invoke", bytes.NewBufferString(`{"function_id":"f1"}`))
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Game-ID", "g1")
    req.Header.Set("X-Env", "test")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != http.StatusForbidden {
        t.Fatalf("expected 403 for env scope denied, got %d (%s)", w.Code, w.Body.String())
    }
}

