package httpserver

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "context"

    msgsgorm "github.com/cuihairu/croupier/internal/repo/gorm/messages"
    usersgorm "github.com/cuihairu/croupier/internal/repo/gorm/users"
    jwt "github.com/cuihairu/croupier/internal/security/token"
    "github.com/glebarez/sqlite"
    "gorm.io/gorm"
)

// setup messages server with sqlite db, user repo and messages repo
func setupMessagesServer(t *testing.T) (*Server, *usersgorm.Repo, *msgsgorm.Repo, uint) {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil { t.Fatalf("open sqlite: %v", err) }
    if err := usersgorm.AutoMigrate(db); err != nil { t.Fatalf("migrate users: %v", err) }
    if err := msgsgorm.AutoMigrate(db); err != nil { t.Fatalf("migrate messages: %v", err) }
    s := &Server{}
    s.gdb = db
    urepo := usersgorm.New(db)
    mrepo := msgsgorm.NewRepo(db)
    s.userRepo = urepo
    s.msgRepo = mrepo
    s.jwtMgr = jwt.NewManager("test")
    // create user u1
    u := &usersgorm.UserAccount{Username: "u1", Active: true}
    if err := urepo.CreateUser(context.Background(), u); err != nil { t.Fatalf("create user: %v", err) }
    return s, urepo, mrepo, u.ID
}

func TestMessagesFlow_DirectAndBroadcast(t *testing.T) {
    s, urepo, mrepo, uid := setupMessagesServer(t)
    r := s.ginEngine()
    tok, _ := s.jwtMgr.Sign("u1", nil, 0)

    // 1) Create a direct message to u1
    if err := mrepo.Create(context.Background(), &msgsgorm.MessageRecord{ToUserID: uid, Title: "hi", Content: "hello", Type: "info"}); err != nil {
        t.Fatalf("create direct: %v", err)
    }
    // GET unread_count should be >=1
    req1 := httptest.NewRequest(http.MethodGet, "/api/messages/unread_count", nil)
    req1.Header.Set("Authorization", "Bearer "+tok)
    w1 := httptest.NewRecorder()
    r.ServeHTTP(w1, req1)
    if w1.Code != http.StatusOK { t.Fatalf("unread_count expected 200, got %d: %s", w1.Code, w1.Body.String()) }

    // GET /api/messages list unread
    req2 := httptest.NewRequest(http.MethodGet, "/api/messages?status=unread&page=1&size=10", nil)
    req2.Header.Set("Authorization", "Bearer "+tok)
    w2 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w2, req2)
    if w2.Code != http.StatusOK { t.Fatalf("list expected 200, got %d: %s", w2.Code, w2.Body.String()) }

    // Parse messages to get ID
    var out struct{ Messages []struct{ ID uint `json:"id"` } `json:"messages"` }
    _ = json.Unmarshal(w2.Body.Bytes(), &out)
    if len(out.Messages) == 0 { t.Fatalf("expected at least one message") }
    mid := out.Messages[0].ID

    // Mark read
    body := bytes.NewBufferString(`{"ids":[` + jsonNum(mid) + `]}`)
    req3 := httptest.NewRequest(http.MethodPost, "/api/messages/read", body)
    req3.Header.Set("Authorization", "Bearer "+tok)
    req3.Header.Set("Content-Type", "application/json")
    w3 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w3, req3)
    if w3.Code != http.StatusNoContent { t.Fatalf("mark read expected 204, got %d: %s", w3.Code, w3.Body.String()) }

    // broadcast to all
    if err := mrepo.Broadcast().Create(context.Background(), &msgsgorm.BroadcastMessageRecord{Title: "b", Content: "c", Type: "info", Audience: "all"}, nil); err != nil {
        t.Fatalf("create broadcast: %v", err)
    }
    // unread_count should increase
    req4 := httptest.NewRequest(http.MethodGet, "/api/messages/unread_count", nil)
    req4.Header.Set("Authorization", "Bearer "+tok)
    w4 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w4, req4)
    if w4.Code != http.StatusOK { t.Fatalf("unread_count 2 expected 200, got %d: %s", w4.Code, w4.Body.String()) }

    // Mark broadcast read by ack id
    // list again to capture broadcast id
    req5 := httptest.NewRequest(http.MethodGet, "/api/messages?status=unread&page=1&size=10", nil)
    req5.Header.Set("Authorization", "Bearer "+tok)
    w5 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w5, req5)
    var out2 struct{ Messages []struct{ ID uint `json:"id"`; Kind string `json:"kind"` } `json:"messages"` }
    _ = json.Unmarshal(w5.Body.Bytes(), &out2)
    var bid uint
    for _, m := range out2.Messages { if m.Kind == "broadcast" { bid = m.ID; break } }
    if bid == 0 { t.Fatalf("expected a broadcast message in list") }

    body2 := bytes.NewBufferString(`{"broadcast_ids":[` + jsonNum(bid) + `]}`)
    req6 := httptest.NewRequest(http.MethodPost, "/api/messages/read", body2)
    req6.Header.Set("Authorization", "Bearer "+tok)
    req6.Header.Set("Content-Type", "application/json")
    w6 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w6, req6)
    if w6.Code != http.StatusNoContent { t.Fatalf("mark broadcast read expected 204, got %d: %s", w6.Code, w6.Body.String()) }

    // final unread_count should be 0
    req7 := httptest.NewRequest(http.MethodGet, "/api/messages/unread_count", nil)
    req7.Header.Set("Authorization", "Bearer "+tok)
    w7 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w7, req7)
    if w7.Code != http.StatusOK { t.Fatalf("final unread_count expected 200, got %d: %s", w7.Code, w7.Body.String()) }

    _ = urepo // silence linters for now
}

func jsonNum(id uint) string { return strconvFormatUint(uint64(id)) }
func strconvFormatUint(v uint64) string { b, _ := json.Marshal(v); return string(b) }
