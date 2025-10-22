package sdk

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "io/ioutil"
    "log"
    "net"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/keepalive"

    functionv1 "github.com/your-org/croupier/gen/go/croupier/function/v1"
    localv1 "github.com/your-org/croupier/gen/go/croupier/agent/local/v1"
    "github.com/your-org/croupier/internal/validation"
    "github.com/your-org/croupier/internal/transport/interceptors"
)

// ClientConfig defines SDK client options.
type ClientConfig struct {
    Addr       string
    UseTLS     bool
    CertFile   string
    KeyFile    string
    CAFile     string
    ServerName string
    LocalListen string // e.g. 127.0.0.1:0
}

// Client is a minimal SDK client placeholder.
type Client struct {
    cfg  ClientConfig
    conn *grpc.ClientConn
    // local server hosting handlers
    l    *localServer
}

func NewClient(cfg ClientConfig) *Client { return &Client{cfg: cfg} }

func (c *Client) Connect(ctx context.Context) error {
    var opt grpc.DialOption
    if c.cfg.UseTLS {
        creds, err := c.loadTLS()
        if err != nil {
            return err
        }
        opt = grpc.WithTransportCredentials(creds)
    } else {
        opt = grpc.WithTransportCredentials(insecure.NewCredentials())
    }
    cc, err := grpc.DialContext(ctx, c.cfg.Addr, opt,
        grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second}),
        grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")),
        interceptors.Chain(nil)...,
    )
    if err != nil {
        return err
    }
    c.conn = cc
    // Start local server for handlers if registered
    if c.l != nil {
        if err := c.l.start(); err != nil { return err }
        // Register local functions to agent
        cli := localv1.NewLocalControlServiceClient(c.conn)
        var fns []*localv1.LocalFunctionDescriptor
        for fid, ver := range c.l.functions {
            fns = append(fns, &localv1.LocalFunctionDescriptor{Id: fid, Version: ver})
        }
        _, err := cli.RegisterLocal(ctx, &localv1.RegisterLocalRequest{
            ServiceId: c.l.serviceID,
            Version:   c.l.version,
            RpcAddr:   c.l.addr,
            Functions: fns,
        })
        if err != nil { return fmt.Errorf("register local: %w", err) }
        log.Printf("sdk registered local functions: n=%d addr=%s", len(fns), c.l.addr)
    }
    return nil
}

func (c *Client) Close() error {
    if c.conn != nil {
        return c.conn.Close()
    }
    return nil
}

func (c *Client) loadTLS() (credentials.TransportCredentials, error) {
    cert, err := tls.LoadX509KeyPair(c.cfg.CertFile, c.cfg.KeyFile)
    if err != nil {
        return nil, err
    }
    caPEM, err := ioutil.ReadFile(c.cfg.CAFile)
    if err != nil {
        return nil, err
    }
    pool := x509.NewCertPool()
    pool.AppendCertsFromPEM(caPEM)
    return credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: pool, ServerName: c.cfg.ServerName}), nil
}

// RegisterFunction registers a handler that will be served locally via a gRPC server.
// Must be called before Connect.
func (c *Client) RegisterFunction(desc Function, h Handler) error {
    if c.l == nil {
        if c.cfg.LocalListen == "" { c.cfg.LocalListen = "127.0.0.1:0" }
        c.l = &localServer{listen: c.cfg.LocalListen, functions: map[string]string{}, handlers: map[string]Handler{}, serviceID: "svc-1", version: "0.1.0", schemas: map[string]map[string]any{}}
    }
    c.l.functions[desc.ID] = desc.Version
    c.l.handlers[desc.ID] = h
    if desc.Schema != nil { c.l.schemas[desc.ID] = desc.Schema }
    return nil
}

// Invoke via Core is not provided in SDK; SDK only provides local handler hosting in this skeleton.

// Types for handlers
type Handler func(ctx context.Context, payload []byte) ([]byte, error)
type Function struct { ID, Version string; Schema map[string]any }
// With optional JSON Schema for server-side validation
// Note: only a minimal subset of JSON Schema is supported in this skeleton.
type FunctionSchema struct {
    Type       string                 `json:"type"`
    Properties map[string]any         `json:"properties"`
    Required   []any                  `json:"required"`
}

// local server hosting FunctionService, dispatching to handlers
type localServer struct {
    listen    string
    addr      string
    functions map[string]string
    handlers  map[string]Handler
    serviceID string
    version   string
    schemas   map[string]map[string]any
}

func (s *localServer) start() error {
    ln, err := net.Listen("tcp", s.listen)
    if err != nil { return err }
    s.addr = ln.Addr().String()
    srv := grpc.NewServer()
    functionv1.RegisterFunctionServiceServer(srv, s)
    go func() { _ = srv.Serve(ln) }()
    return nil
}

// Implement FunctionServiceServer
func (s *localServer) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
    h, ok := s.handlers[req.GetFunctionId()]
    if !ok { return nil, fmt.Errorf("unknown function: %s", req.GetFunctionId()) }
    if sc, ok := s.schemas[req.GetFunctionId()]; ok && sc != nil {
        if err := validation.ValidateJSON(sc, req.GetPayload()); err != nil { return nil, fmt.Errorf("payload invalid: %v", err) }
    }
    out, err := h(ctx, req.GetPayload())
    if err != nil { return nil, err }
    return &functionv1.InvokeResponse{Payload: out}, nil
}

func (s *localServer) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
    return &functionv1.StartJobResponse{JobId: "job-" + req.GetFunctionId()}, nil
}

func (s *localServer) StreamJob(req *functionv1.JobStreamRequest, stream functionv1.FunctionService_StreamJobServer) error {
    return nil
}

// NewIdempotencyKey creates a random hex string for use as idempotency keys.
func NewIdempotencyKey() string {
    b := make([]byte, 16)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}
