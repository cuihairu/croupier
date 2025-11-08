package httpserver

import (
    "archive/tar"
    "bytes"
    "compress/gzip"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"

    auditchain "github.com/cuihairu/croupier/internal/audit/chain"
    "github.com/cuihairu/croupier/internal/server/registry"
    functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
    "context"
    jwt "github.com/cuihairu/croupier/internal/auth/token"
    "github.com/cuihairu/croupier/internal/auth/rbac"
    "time"
)

type fakeInvoker struct{}
func (fakeInvoker) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
    return &functionv1.InvokeResponse{Payload: []byte(`{}`)}, nil
}
func (fakeInvoker) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
    return &functionv1.StartJobResponse{JobId: ""}, nil
}
func (fakeInvoker) StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error) {
    return nil, nil
}
func (fakeInvoker) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) { return &functionv1.StartJobResponse{JobId: ""}, nil }

func TestPacks_List_And_UISchema_Export(t *testing.T) {
    dir := t.TempDir()
    // prepare minimal pack files
    mustWrite := func(p string, b []byte) {
        if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { t.Fatalf("mkdir: %v", err) }
        if err := os.WriteFile(p, b, 0o644); err != nil { t.Fatalf("write: %v", err) }
    }
    mani := map[string]any{"functions": []map[string]any{{"id": "ex.fn", "version": "1.0.0"}}}
    mb, _ := json.Marshal(mani)
    mustWrite(filepath.Join(dir, "manifest.json"), mb)
    mustWrite(filepath.Join(dir, "descriptors", "ex.fn.json"), []byte(`{"id":"ex.fn","version":"1.0.0","params":{"type":"object","properties":{}}}`))
    mustWrite(filepath.Join(dir, "ui", "ex.fn.schema.json"), []byte(`{"type":"object","properties":{}}`))
    mustWrite(filepath.Join(dir, "ui", "ex.fn.uischema.json"), []byte(`{"ui:layout":{"type":"grid","cols":2}}`))

    // audit endpoint reads from logs/audit.log, so write there for this test
    _ = os.MkdirAll("logs", 0o755)
    aw, err := auditchain.NewWriter("logs/audit.log")
    if err != nil { t.Fatalf("audit writer: %v", err) }
    defer aw.Close()
    srv, err := NewServer(dir, new(fakeInvoker), aw, nil, registry.NewStore(), nil, nil, nil)
    if err != nil { t.Fatalf("NewServer: %v", err) }

    // GET /api/packs/list
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/packs/list", nil)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("packs list code=%d body=%s", rr.Code, rr.Body.String()) }
    var out map[string]any
    if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil { t.Fatalf("json: %v", err) }
    if out["manifest"] == nil { t.Fatalf("missing manifest in list") }
    if etg, _ := out["etag"].(string); etg == "" { t.Fatalf("missing etag in packs/list") }

    // GET /api/ui_schema?id=ex.fn
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/ui_schema?id=ex.fn", nil)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("ui_schema code=%d body=%s", rr.Code, rr.Body.String()) }

    // GET /api/packs/export
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/packs/export", nil)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("export code=%d body=%s", rr.Code, rr.Body.String()) }
    if et := rr.Header().Get("ETag"); et == "" { t.Fatalf("missing ETag header on export") }
    // read tar.gz entries
    gz, err := gzip.NewReader(bytes.NewReader(rr.Body.Bytes()))
    if err != nil { t.Fatalf("gz: %v", err) }
    defer gz.Close()
    tr := tar.NewReader(gz)
    foundMani := false
    for {
        hdr, err := tr.Next()
        if err == io.EOF { break }
        if err != nil { t.Fatalf("tar: %v", err) }
        if hdr.Name == "manifest.json" { foundMani = true }
    }
    if !foundMani { t.Fatalf("export missing manifest.json") }
}

func TestAssignments_Get_Post_And_PacksReload(t *testing.T) {
    dir := t.TempDir()
    // prepare minimal pack files and one descriptor
    mustWrite := func(p string, b []byte) {
        if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { t.Fatalf("mkdir: %v", err) }
        if err := os.WriteFile(p, b, 0o644); err != nil { t.Fatalf("write: %v", err) }
    }
    mani := map[string]any{"functions": []map[string]any{{"id": "ex.fn", "version": "1.0.0"}}}
    mb, _ := json.Marshal(mani)
    mustWrite(filepath.Join(dir, "manifest.json"), mb)
    mustWrite(filepath.Join(dir, "descriptors", "ex.fn.json"), []byte(`{"id":"ex.fn","version":"1.0.0","params":{"type":"object","properties":{}}}`))

    aw, err := auditchain.NewWriter(filepath.Join(dir, "audit.log"))
    if err != nil { t.Fatalf("audit writer: %v", err) }
    defer aw.Close()
    // enable JWT for assignments endpoint
    mgr := jwt.NewManager("test-secret")
    srv, err := NewServer(dir, new(fakeInvoker), aw, nil, registry.NewStore(), mgr, nil, nil)
    if err != nil { t.Fatalf("NewServer: %v", err) }
    tok, _ := mgr.Sign("tester", []string{"admin"}, 0)

    // GET /api/assignments should require auth
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/assignments?game_id=g1&env=dev", nil)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code != http.StatusUnauthorized { t.Fatalf("assignments get expect 401, got %d", rr.Code) }
    rr = httptest.NewRecorder()
    req.Header.Set("Authorization", "Bearer "+tok)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("assignments get code=%d body=%s", rr.Code, rr.Body.String()) }

    // POST /api/assignments with known+unknown ids
    body := bytes.NewBufferString(`{"GameID":"g1","Env":"dev","Functions":["ex.fn","bad.fn"]}`)
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodPost, "/api/assignments", body)
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("assignments post code=%d body=%s", rr.Code, rr.Body.String()) }
    var postOut struct{ Ok bool `json:"ok"`; Unknown []string `json:"unknown"` }
    if err := json.Unmarshal(rr.Body.Bytes(), &postOut); err != nil { t.Fatalf("json: %v", err) }
    if len(postOut.Unknown) != 1 || postOut.Unknown[0] != "bad.fn" { t.Fatalf("unexpected unknown: %+v", postOut.Unknown) }

    // GET again and verify only valid function is stored
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/assignments?game_id=g1&env=dev", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("assignments get2 code=%d body=%s", rr.Code, rr.Body.String()) }
    var getOut struct{ Assignments map[string][]string `json:"assignments"` }
    if err := json.Unmarshal(rr.Body.Bytes(), &getOut); err != nil { t.Fatalf("json: %v", err) }
    if len(getOut.Assignments) != 1 || len(getOut.Assignments["g1|dev"]) != 1 || getOut.Assignments["g1|dev"][0] != "ex.fn" {
        t.Fatalf("unexpected assignments: %+v", getOut.Assignments)
    }

    // Now add a new descriptor and trigger reload
    mustWrite(filepath.Join(dir, "descriptors", "ex2.fn.json"), []byte(`{"id":"ex2.fn","version":"1.0.0","params":{"type":"object","properties":{}}}`))
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodPost, "/api/packs/reload", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("reload code=%d body=%s", rr.Code, rr.Body.String()) }

    // Verify /api/descriptors includes the new one
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/descriptors", nil)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("descriptors code=%d body=%s", rr.Code, rr.Body.String()) }
    var descs []map[string]any
    if err := json.Unmarshal(rr.Body.Bytes(), &descs); err != nil { t.Fatalf("json: %v", err) }
    have := map[string]bool{}
    for _, d := range descs { if id, ok := d["id"].(string); ok { have[id] = true } }
    if !(have["ex.fn"] && have["ex2.fn"]) { t.Fatalf("expected ex.fn and ex2.fn after reload, got: %+v", have) }
}

func TestPacksReload_RBAC(t *testing.T) {
    dir := t.TempDir()
    // minimal pack state
    _ = os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"functions":[]}`), 0o644)
    aw, err := auditchain.NewWriter(filepath.Join(dir, "audit.log"))
    if err != nil { t.Fatalf("audit writer: %v", err) }
    defer aw.Close()
    mgr := jwt.NewManager("s")
    pol := rbac.NewPolicy()
    pol.Grant("ok", "packs:reload")
    srv, err := NewServer(dir, new(fakeInvoker), aw, pol, registry.NewStore(), mgr, nil, nil)
    if err != nil { t.Fatalf("NewServer: %v", err) }
    // unauthorized user (no auth)
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPost, "/api/packs/reload", nil)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code != http.StatusUnauthorized { t.Fatalf("expected 401, got %d", rr.Code) }
    // forbidden user
    tokFail, _ := mgr.Sign("nope", nil, 0)
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodPost, "/api/packs/reload", nil)
    req.Header.Set("Authorization", "Bearer "+tokFail)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code != http.StatusForbidden { t.Fatalf("expected 403, got %d", rr.Code) }
    // allowed user
    tokOK, _ := mgr.Sign("ok", nil, 0)
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodPost, "/api/packs/reload", nil)
    req.Header.Set("Authorization", "Bearer "+tokOK)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("expected 2xx, got %d", rr.Code) }
}

func TestRegistry_Coverage_HealthyTotal_Uncovered(t *testing.T) {
    dir := t.TempDir()
    // minimal pack
    _ = os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"functions":[]}`), 0o644)
    aw, err := auditchain.NewWriter(filepath.Join(dir, "audit.log"))
    if err != nil { t.Fatalf("audit writer: %v", err) }
    defer aw.Close()
    mgr := jwt.NewManager("s")
    reg := registry.NewStore()
    srv, err := NewServer(dir, new(fakeInvoker), aw, nil, reg, mgr, nil, nil)
    if err != nil { t.Fatalf("NewServer: %v", err) }
    // prepare two agents for g1 with ex.fn: one healthy, one expired; and no agent for ex2.fn
    now := time.Now()
    reg.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "g1", Env: "dev", RPCAddr: "127.0.0.1:1", Functions: map[string]registry.FunctionMeta{"ex.fn": {Enabled: true}}, ExpireAt: now.Add(60 * time.Second)})
    reg.UpsertAgent(&registry.AgentSession{AgentID: "a2", GameID: "g1", Env: "dev", RPCAddr: "127.0.0.1:2", Functions: map[string]registry.FunctionMeta{"ex.fn": {Enabled: true}}, ExpireAt: now.Add(-10 * time.Second)})
    // set assignments for g1|dev -> [ex.fn, ex2.fn]
    srv.assignments["g1|dev"] = []string{"ex.fn", "ex2.fn"}
    // call /api/registry
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/registry", nil)
    // auth
    tok, _ := mgr.Sign("u", nil, 0)
    req.Header.Set("Authorization", "Bearer "+tok)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("registry code=%d body=%s", rr.Code, rr.Body.String()) }
    var out struct{
        Coverage []struct{
            GameEnv string `json:"game_env"`
            Functions map[string]struct{ Healthy, Total int } `json:"functions"`
            Uncovered []string `json:"uncovered"`
        } `json:"coverage"`
    }
    if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil { t.Fatalf("json: %v", err) }
    if len(out.Coverage) == 0 { t.Fatalf("missing coverage") }
    var cov map[string]struct{ Healthy, Total int }
    var unc []string
    for _, c := range out.Coverage { if c.GameEnv == "g1|dev" { cov = c.Functions; unc = c.Uncovered; break } }
    if cov == nil { t.Fatalf("missing coverage for g1|dev") }
    if cov["ex.fn"].Healthy != 1 || cov["ex.fn"].Total != 2 { t.Fatalf("unexpected ex.fn cov: %+v", cov["ex.fn"]) }
    if cov["ex2.fn"].Healthy != 0 || cov["ex2.fn"].Total != 0 { t.Fatalf("unexpected ex2.fn cov: %+v", cov["ex2.fn"]) }
    // uncovered should include ex2.fn
    found := false
    for _, id := range unc { if id == "ex2.fn" { found = true; break } }
    if !found { t.Fatalf("ex2.fn should be in uncovered: %+v", unc) }
}

func TestRegistry_RBAC(t *testing.T) {
    dir := t.TempDir()
    _ = os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"functions":[]}`), 0o644)
    aw, err := auditchain.NewWriter(filepath.Join(dir, "audit.log"))
    if err != nil { t.Fatalf("audit writer: %v", err) }
    defer aw.Close()
    mgr := jwt.NewManager("s")
    pol := rbac.NewPolicy()
    pol.Grant("ok", "registry:read")
    srv, err := NewServer(dir, new(fakeInvoker), aw, pol, registry.NewStore(), mgr, nil, nil)
    if err != nil { t.Fatalf("NewServer: %v", err) }
    // unauthorized
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/registry", nil)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code != http.StatusUnauthorized { t.Fatalf("expect 401, got %d", rr.Code) }
    // forbidden
    tokNope, _ := mgr.Sign("nope", nil, 0)
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/registry", nil)
    req.Header.Set("Authorization", "Bearer "+tokNope)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code != http.StatusForbidden { t.Fatalf("expect 403, got %d", rr.Code) }
    // allowed
    tokOK, _ := mgr.Sign("ok", nil, 0)
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/registry", nil)
    req.Header.Set("Authorization", "Bearer "+tokOK)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("expect 2xx, got %d", rr.Code) }
}

func TestAudit_RBAC(t *testing.T) {
    dir := t.TempDir()
    _ = os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"functions":[]}`), 0o644)
    aw, _ := auditchain.NewWriter(filepath.Join(dir, "audit.log"))
    defer aw.Close()
    mgr := jwt.NewManager("s")
    pol := rbac.NewPolicy()
    pol.Grant("ok", "audit:read")
    srv, err := NewServer(dir, new(fakeInvoker), aw, pol, registry.NewStore(), mgr, nil, nil)
    if err != nil { t.Fatalf("NewServer: %v", err) }
    // unauthorized
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code != http.StatusUnauthorized { t.Fatalf("expect 401, got %d", rr.Code) }
    // forbidden
    tokNope, _ := mgr.Sign("nope", nil, 0)
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/audit", nil)
    req.Header.Set("Authorization", "Bearer "+tokNope)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code != http.StatusForbidden { t.Fatalf("expect 403, got %d", rr.Code) }
    // allowed
    tokOK, _ := mgr.Sign("ok", nil, 0)
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/audit", nil)
    req.Header.Set("Authorization", "Bearer "+tokOK)
    srv.ginEngine().ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("expect 2xx, got %d body=%s", rr.Code, rr.Body.String()) }
}

// assignments audit is covered by manual tests; integration test omitted due to external log path dependency.
