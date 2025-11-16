package httpserver

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "context"

    repogames "github.com/cuihairu/croupier/internal/repo/gorm/games"
    usersgorm "github.com/cuihairu/croupier/internal/repo/gorm/users"
    gamesvc "github.com/cuihairu/croupier/internal/service/games"
    jwt "github.com/cuihairu/croupier/internal/security/token"
    "github.com/glebarez/sqlite"
    "gorm.io/gorm"
)

// setupServerWithMemDB prepares a minimal server with in-memory sqlite, games repo/service and user repo.
func setupServerWithMemDB(t *testing.T) *Server {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil { t.Fatalf("open sqlite: %v", err) }
    if err := repogames.AutoMigrate(db); err != nil { t.Fatalf("migrate games: %v", err) }
    if err := usersgorm.AutoMigrate(db); err != nil { t.Fatalf("migrate users: %v", err) }
    s := &Server{}
    s.gdb = db
    s.games = repogames.NewRepo(db)
    s.userRepo = usersgorm.New(db)
    // JWT for auth
    s.jwtMgr = jwt.NewManager("test-secret")
    // Service with no defaults
    s.gamesSvc = gamesvc.NewService(repogames.NewPortRepo(s.games), nil)
    return s
}

func TestMeGamesEnvScopes(t *testing.T) {
    s := setupServerWithMemDB(t)
    // Create game with envs: prod, test, dev
    g := &repogames.Game{Name: "g1", Enabled: true}
    if err := s.games.Create(context.Background(), g); err != nil { t.Fatalf("create game: %v", err) }
    _ = s.games.AddEnvWithMeta(context.Background(), g.ID, "prod", "生产", "#52c41a")
    _ = s.games.AddEnvWithMeta(context.Background(), g.ID, "test", "测试", "#722ed1")
    _ = s.games.AddEnvWithMeta(context.Background(), g.ID, "dev",  "开发", "#1677ff")

    // Create user u1
    u := &usersgorm.UserAccount{Username: "u1", Active: true}
    if err := s.userRepo.CreateUser(context.Background(), u); err != nil { t.Fatalf("create user: %v", err) }
    // Scope: only game g1 and env prod
    if err := s.userRepo.ReplaceUserGameIDs(context.Background(), u.ID, []uint{g.ID}); err != nil { t.Fatalf("scope games: %v", err) }
    if err := s.userRepo.ReplaceUserGameEnvs(context.Background(), u.ID, g.ID, []string{"prod"}); err != nil { t.Fatalf("scope envs: %v", err) }

    // Call /api/me/games with JWT of u1
    tok, _ := s.jwtMgr.Sign("u1", []string{"developer"}, 0)
    r := s.ginEngine()
    req := httptest.NewRequest(http.MethodGet, "/api/me/games", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
    }
    // Expect only one game with envs ["prod"] and game_envs containing only prod
    body := w.Body.String()
    if !(strContains(body, `"games"`) && strContains(body, `"envs":["prod"]`)) {
        t.Fatalf("unexpected body: %s", body)
    }
    if !(strContains(body, `"game_envs"`) && strContains(body, `"env":"prod"`) && !strContains(body, `"env":"test"`)) {
        t.Fatalf("game_envs not filtered by env scopes: %s", body)
    }
}

// strContains is a tiny helper to avoid pulling json packages here.
func strContains(s, sub string) bool { return len(s) >= len(sub) && (func() bool { return (string)([]byte(s)) != "" && (len(sub) == 0 || (len(s) >= len(sub) && (indexOfStr(s, sub) >= 0)) ) })() }
func indexOfStr(s, sub string) int {
    for i := 0; i+len(sub) <= len(s); i++ { if s[i:i+len(sub)] == sub { return i } }
    return -1
}
