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
    "github.com/cuihairu/croupier/internal/server/games"
    "github.com/cuihairu/croupier/internal/server/registry"
    functionv1 "github.com/cuihairu/croupier/gen/go/croupier/function/v1"
    "context"
    jwt "github.com/cuihairu/croupier/internal/auth/token"
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

    aw, err := auditchain.NewWriter(filepath.Join(dir, "audit.log"))
    if err != nil { t.Fatalf("audit writer: %v", err) }
    defer aw.Close()
    srv, err := NewServer(dir, new(fakeInvoker), aw, nil, games.NewStore("") , registry.NewStore(), nil, nil, nil, nil)
    if err != nil { t.Fatalf("NewServer: %v", err) }

    // GET /api/packs/list
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/packs/list", nil)
    srv.mux.ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("packs list code=%d body=%s", rr.Code, rr.Body.String()) }
    var out map[string]any
    if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil { t.Fatalf("json: %v", err) }
    if out["manifest"] == nil { t.Fatalf("missing manifest in list") }

    // GET /api/ui_schema?id=ex.fn
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/ui_schema?id=ex.fn", nil)
    srv.mux.ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("ui_schema code=%d body=%s", rr.Code, rr.Body.String()) }

    // GET /api/packs/export
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/packs/export", nil)
    srv.mux.ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("export code=%d body=%s", rr.Code, rr.Body.String()) }
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
    srv, err := NewServer(dir, new(fakeInvoker), aw, nil, games.NewStore(""), registry.NewStore(), nil, mgr, nil, nil)
    if err != nil { t.Fatalf("NewServer: %v", err) }
    tok, _ := mgr.Sign("tester", []string{"admin"}, 0)

    // GET /api/assignments should work with auth
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/assignments?game_id=g1&env=dev", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    srv.mux.ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("assignments get code=%d body=%s", rr.Code, rr.Body.String()) }

    // POST /api/assignments with known+unknown ids
    body := bytes.NewBufferString(`{"GameID":"g1","Env":"dev","Functions":["ex.fn","bad.fn"]}`)
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodPost, "/api/assignments", body)
    req.Header.Set("Authorization", "Bearer "+tok)
    req.Header.Set("Content-Type", "application/json")
    srv.mux.ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("assignments post code=%d body=%s", rr.Code, rr.Body.String()) }
    var postOut struct{ Ok bool `json:"ok"`; Unknown []string `json:"unknown"` }
    if err := json.Unmarshal(rr.Body.Bytes(), &postOut); err != nil { t.Fatalf("json: %v", err) }
    if len(postOut.Unknown) != 1 || postOut.Unknown[0] != "bad.fn" { t.Fatalf("unexpected unknown: %+v", postOut.Unknown) }

    // GET again and verify only valid function is stored
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/assignments?game_id=g1&env=dev", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    srv.mux.ServeHTTP(rr, req)
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
    srv.mux.ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("reload code=%d body=%s", rr.Code, rr.Body.String()) }

    // Verify /api/descriptors includes the new one
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/api/descriptors", nil)
    srv.mux.ServeHTTP(rr, req)
    if rr.Code/100 != 2 { t.Fatalf("descriptors code=%d body=%s", rr.Code, rr.Body.String()) }
    var descs []map[string]any
    if err := json.Unmarshal(rr.Body.Bytes(), &descs); err != nil { t.Fatalf("json: %v", err) }
    have := map[string]bool{}
    for _, d := range descs { if id, ok := d["id"].(string); ok { have[id] = true } }
    if !(have["ex.fn"] && have["ex2.fn"]) { t.Fatalf("expected ex.fn and ex2.fn after reload, got: %+v", have) }
}
