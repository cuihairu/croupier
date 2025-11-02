package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net"
    "net/http"
    "os"
    "time"
    "strings"
    "net/url"

    functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
    localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
    "google.golang.org/grpc"
)

// http-adapter implements a generic HTTP invoker function: http.generic_invoke
// Request JSON: { method, url, headers: {..}, body }

type server struct{ functionv1.UnimplementedFunctionServiceServer }

func (s *server) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
    httpClient := &http.Client{ Timeout: 15 * time.Second }
    switch req.GetFunctionId() {
    case "alertmanager.list_alerts":
        // Map simple params to GET {base}/api/v2/alerts?...
        var in struct{
            BaseURL string `json:"base_url"`
            Silenced *bool  `json:"silenced,omitempty"`
            Inhibited *bool `json:"inhibited,omitempty"`
            Active *bool    `json:"active,omitempty"`
        }
        if err := json.Unmarshal(req.GetPayload(), &in); err != nil { return nil, fmt.Errorf("bad payload: %w", err) }
        if in.BaseURL == "" { return nil, fmt.Errorf("base_url required") }
        u, err := url.Parse(strings.TrimRight(in.BaseURL, "/") + "/api/v2/alerts")
        if err != nil { return nil, err }
        q := u.Query()
        if in.Silenced != nil { q.Set("silenced", fmt.Sprintf("%v", *in.Silenced)) }
        if in.Inhibited != nil { q.Set("inhibited", fmt.Sprintf("%v", *in.Inhibited)) }
        if in.Active != nil { q.Set("active", fmt.Sprintf("%v", *in.Active)) }
        u.RawQuery = q.Encode()
        r, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
        if err != nil { return nil, err }
        if req.Metadata != nil {
            if r.Header.Get("X-Trace-Id") == "" { if v := req.Metadata["trace_id"]; v != "" { r.Header.Set("X-Trace-Id", v) } }
            if r.Header.Get("X-Game-Id") == "" { if v := req.Metadata["game_id"]; v != "" { r.Header.Set("X-Game-Id", v) } }
            if r.Header.Get("X-Env") == "" { if v := req.Metadata["env"]; v != "" { r.Header.Set("X-Env", v) } }
        }
        resp, err := httpClient.Do(r)
        if err != nil { return nil, err }
        defer resp.Body.Close()
        b, _ := io.ReadAll(resp.Body)
        if resp.StatusCode/100 != 2 {
            if json.Valid(b) { return &functionv1.InvokeResponse{Payload: b}, nil }
            out, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "body": string(b)})
            return &functionv1.InvokeResponse{Payload: out}, nil
        }
        return &functionv1.InvokeResponse{Payload: b}, nil
    case "grafana.search_dashboards":
        // Map params to GET {base}/api/search?query=...&type=dash-db
        var in struct{
            BaseURL string `json:"base_url"`
            Query   string `json:"query"`
            Type    string `json:"type"` // default dash-db
        }
        if err := json.Unmarshal(req.GetPayload(), &in); err != nil { return nil, fmt.Errorf("bad payload: %w", err) }
        if in.BaseURL == "" { return nil, fmt.Errorf("base_url required") }
        if in.Type == "" { in.Type = "dash-db" }
        u, err := url.Parse(strings.TrimRight(in.BaseURL, "/") + "/api/search")
        if err != nil { return nil, err }
        q := u.Query()
        if in.Query != "" { q.Set("query", in.Query) }
        if in.Type != "" { q.Set("type", in.Type) }
        u.RawQuery = q.Encode()
        r, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
        if err != nil { return nil, err }
        if req.Metadata != nil {
            if r.Header.Get("X-Trace-Id") == "" { if v := req.Metadata["trace_id"]; v != "" { r.Header.Set("X-Trace-Id", v) } }
            if r.Header.Get("X-Game-Id") == "" { if v := req.Metadata["game_id"]; v != "" { r.Header.Set("X-Game-Id", v) } }
            if r.Header.Get("X-Env") == "" { if v := req.Metadata["env"]; v != "" { r.Header.Set("X-Env", v) } }
        }
        resp, err := httpClient.Do(r)
        if err != nil { return nil, err }
        defer resp.Body.Close()
        b, _ := io.ReadAll(resp.Body)
        if resp.StatusCode/100 != 2 {
            if json.Valid(b) { return &functionv1.InvokeResponse{Payload: b}, nil }
            out, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "body": string(b)})
            return &functionv1.InvokeResponse{Payload: out}, nil
        }
        return &functionv1.InvokeResponse{Payload: b}, nil
    default:
        // generic http.generic_invoke path
        var in struct{ Method, Url string; Headers map[string]string; Body string }
        if err := json.Unmarshal(req.GetPayload(), &in); err != nil { return nil, fmt.Errorf("bad payload: %w", err) }
        if in.Method == "" { in.Method = "GET" }
        var body io.Reader
        if in.Body != "" { body = strings.NewReader(in.Body) }
        r, err := http.NewRequestWithContext(ctx, in.Method, in.Url, body)
        if err != nil { return nil, err }
        for k, v := range in.Headers { r.Header.Set(k, v) }
        if req.Metadata != nil {
            if r.Header.Get("X-Trace-Id") == "" { if v := req.Metadata["trace_id"]; v != "" { r.Header.Set("X-Trace-Id", v) } }
            if r.Header.Get("X-Game-Id") == "" { if v := req.Metadata["game_id"]; v != "" { r.Header.Set("X-Game-Id", v) } }
            if r.Header.Get("X-Env") == "" { if v := req.Metadata["env"]; v != "" { r.Header.Set("X-Env", v) } }
        }
        resp, err := httpClient.Do(r)
        if err != nil { return nil, err }
        defer resp.Body.Close()
        b, _ := io.ReadAll(resp.Body)
        if json.Valid(b) { return &functionv1.InvokeResponse{Payload: b}, nil }
        out, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "body": string(b)})
        return &functionv1.InvokeResponse{Payload: out}, nil
    }
}

func (s *server) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) { return &functionv1.StartJobResponse{JobId: ""}, nil }

func main() {
    agent := os.Getenv("AGENT_ADDR")
    if agent == "" { agent = "127.0.0.1:19090" }
    listen := os.Getenv("RPC_ADDR")
    if listen == "" { listen = ":20081" }
    serviceID := os.Getenv("SERVICE_ID")
    if serviceID == "" { serviceID = "http-adapter" }
    version := os.Getenv("VERSION")
    if version == "" { version = "1.0.0" }

    lis, err := net.Listen("tcp", listen)
    if err != nil { log.Fatal(err) }
    gs := grpc.NewServer()
    functionv1.RegisterFunctionServiceServer(gs, &server{})
    go func(){ log.Printf("http-adapter listening on %s", listen); if err := gs.Serve(lis); err != nil { log.Fatal(err) } }()

    cc, err := grpc.Dial(agent, grpc.WithInsecure())
    if err != nil { log.Fatal(err) }
    defer cc.Close()
    lc := localv1.NewLocalControlServiceClient(cc)
    req := &localv1.RegisterLocalRequest{ServiceId: serviceID, Version: version, RpcAddr: listen,
        Functions: []*localv1.LocalFunctionDescriptor{
            {Id: "http.generic_invoke", Version: version },
            {Id: "alertmanager.list_alerts", Version: version },
        },
    }
    if _, err := lc.RegisterLocal(context.Background(), req); err != nil { log.Fatal(err) }
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C { _, _ = lc.Heartbeat(context.Background(), &localv1.HeartbeatRequest{ServiceId: serviceID}) }
}
