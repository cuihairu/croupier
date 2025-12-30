package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// http-adapter implements a generic HTTP invoker function: http.generic_invoke
// Request JSON: { method, url, headers: {..}, body }

type server struct {
	functionv1.UnimplementedFunctionServiceServer
}

func (s *server) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	switch req.GetFunctionId() {
	case "alertmanager.list_alerts":
		// Map simple params to GET {base}/api/v2/alerts?...
		var in struct {
			BaseURL   string `json:"base_url"`
			Silenced  *bool  `json:"silenced,omitempty"`
			Inhibited *bool  `json:"inhibited,omitempty"`
			Active    *bool  `json:"active,omitempty"`
		}
		if err := json.Unmarshal(req.GetPayload(), &in); err != nil {
			return nil, fmt.Errorf("bad payload: %w", err)
		}
		if in.BaseURL == "" {
			return nil, fmt.Errorf("base_url required")
		}
		u, err := url.Parse(strings.TrimRight(in.BaseURL, "/") + "/api/v2/alerts")
		if err != nil {
			return nil, err
		}
		q := u.Query()
		if in.Silenced != nil {
			q.Set("silenced", fmt.Sprintf("%v", *in.Silenced))
		}
		if in.Inhibited != nil {
			q.Set("inhibited", fmt.Sprintf("%v", *in.Inhibited))
		}
		if in.Active != nil {
			q.Set("active", fmt.Sprintf("%v", *in.Active))
		}
		u.RawQuery = q.Encode()
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		if req.Metadata != nil {
			if r.Header.Get("X-Trace-Id") == "" {
				if v := req.Metadata["trace_id"]; v != "" {
					r.Header.Set("X-Trace-Id", v)
				}
			}
			if r.Header.Get("X-Game-Id") == "" {
				if v := req.Metadata["game_id"]; v != "" {
					r.Header.Set("X-Game-Id", v)
				}
			}
			if r.Header.Get("X-Env") == "" {
				if v := req.Metadata["env"]; v != "" {
					r.Header.Set("X-Env", v)
				}
			}
		}
		resp, err := httpClient.Do(r)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode/100 != 2 {
			if json.Valid(b) {
				return &functionv1.InvokeResponse{Payload: b}, nil
			}
			out, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "body": string(b)})
			return &functionv1.InvokeResponse{Payload: out}, nil
		}
		return &functionv1.InvokeResponse{Payload: b}, nil
	case "grafana.search_dashboards":
		// Map params to GET {base}/api/search?query=...&type=dash-db
		var in struct {
			BaseURL string `json:"base_url"`
			Query   string `json:"query"`
			Type    string `json:"type"` // default dash-db
		}
		if err := json.Unmarshal(req.GetPayload(), &in); err != nil {
			return nil, fmt.Errorf("bad payload: %w", err)
		}
		if in.BaseURL == "" {
			return nil, fmt.Errorf("base_url required")
		}
		if in.Type == "" {
			in.Type = "dash-db"
		}
		u, err := url.Parse(strings.TrimRight(in.BaseURL, "/") + "/api/search")
		if err != nil {
			return nil, err
		}
		q := u.Query()
		if in.Query != "" {
			q.Set("query", in.Query)
		}
		if in.Type != "" {
			q.Set("type", in.Type)
		}
		u.RawQuery = q.Encode()
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		if req.Metadata != nil {
			if r.Header.Get("X-Trace-Id") == "" {
				if v := req.Metadata["trace_id"]; v != "" {
					r.Header.Set("X-Trace-Id", v)
				}
			}
			if r.Header.Get("X-Game-Id") == "" {
				if v := req.Metadata["game_id"]; v != "" {
					r.Header.Set("X-Game-Id", v)
				}
			}
			if r.Header.Get("X-Env") == "" {
				if v := req.Metadata["env"]; v != "" {
					r.Header.Set("X-Env", v)
				}
			}
		}
		resp, err := httpClient.Do(r)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode/100 != 2 {
			if json.Valid(b) {
				return &functionv1.InvokeResponse{Payload: b}, nil
			}
			out, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "body": string(b)})
			return &functionv1.InvokeResponse{Payload: out}, nil
		}
		return &functionv1.InvokeResponse{Payload: b}, nil
	default:
		// generic http.generic_invoke path
		var in struct {
			Method, Url string
			Headers     map[string]string
			Body        string
		}
		if err := json.Unmarshal(req.GetPayload(), &in); err != nil {
			return nil, fmt.Errorf("bad payload: %w", err)
		}
		if in.Method == "" {
			in.Method = "GET"
		}
		var body io.Reader
		if in.Body != "" {
			body = strings.NewReader(in.Body)
		}
		r, err := http.NewRequestWithContext(ctx, in.Method, in.Url, body)
		if err != nil {
			return nil, err
		}
		for k, v := range in.Headers {
			r.Header.Set(k, v)
		}
		if req.Metadata != nil {
			if r.Header.Get("X-Trace-Id") == "" {
				if v := req.Metadata["trace_id"]; v != "" {
					r.Header.Set("X-Trace-Id", v)
				}
			}
			if r.Header.Get("X-Game-Id") == "" {
				if v := req.Metadata["game_id"]; v != "" {
					r.Header.Set("X-Game-Id", v)
				}
			}
			if r.Header.Get("X-Env") == "" {
				if v := req.Metadata["env"]; v != "" {
					r.Header.Set("X-Env", v)
				}
			}
		}
		resp, err := httpClient.Do(r)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if json.Valid(b) {
			return &functionv1.InvokeResponse{Payload: b}, nil
		}
		out, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "body": string(b)})
		return &functionv1.InvokeResponse{Payload: out}, nil
	}
}

func (s *server) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	// HTTP adapter is synchronous by nature, doesn't support asynchronous jobs
	// Return explicit error to make it clear this operation is not supported
	return nil, status.Error(codes.Unimplemented, "HTTP adapter does not support asynchronous jobs. Use Invoke instead.")
}

func main() {
	agent := os.Getenv("AGENT_ADDR")
	if agent == "" {
		agent = "127.0.0.1:19090"
	}
	listen := os.Getenv("RPC_ADDR")
	if listen == "" {
		listen = ":20081"
	}
	serviceID := os.Getenv("SERVICE_ID")
	if serviceID == "" {
		serviceID = "http-adapter"
	}
	version := os.Getenv("VERSION")
	if version == "" {
		version = "1.0.0"
	}

	// Create gRPC server with optional TLS
	var serverOpts []grpc.ServerOption

	// Check if TLS is enabled for the gRPC server
	serverCertFile := os.Getenv("SERVER_CERT_FILE")
	serverKeyFile := os.Getenv("SERVER_KEY_FILE")
	caFile := os.Getenv("CA_FILE")
	requireClientCert := os.Getenv("REQUIRE_CLIENT_CERT") == "true"

	if serverCertFile != "" && serverKeyFile != "" {
		// Use TLS for server
		creds, err := tlsutil.ServerTLS(serverCertFile, serverKeyFile, caFile, requireClientCert)
		if err != nil {
			log.Fatalf("Failed to create server TLS credentials: %v", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
		log.Printf("http-adapter listening on %s with TLS", listen)
	} else {
		// Use insecure server
		log.Printf("http-adapter listening on %s (insecure)", listen)
	}

	rpcConf := zrpc.RpcServerConf{ListenOn: listen}
	rpcConf.Name = serviceID
	gs := zrpc.MustNewServer(rpcConf, func(s *grpc.Server) {
		functionv1.RegisterFunctionServiceServer(s, &server{})
	})
	gs.AddOptions(serverOpts...)
	go gs.Start()

	// Create connection to agent with optional TLS
	var dialOpts []grpc.DialOption
	agentTLS := os.Getenv("AGENT_TLS_ENABLED") == "true"
	if agentTLS {
		// Use TLS for agent connection
		clientCertFile := os.Getenv("CLIENT_CERT_FILE")
		clientKeyFile := os.Getenv("CLIENT_KEY_FILE")
		agentCAFile := os.Getenv("AGENT_CA_FILE")
		serverName := os.Getenv("AGENT_SERVER_NAME")

		creds, err := tlsutil.ClientTLS(clientCertFile, clientKeyFile, agentCAFile, serverName)
		if err != nil {
			log.Fatalf("Failed to create client TLS credentials: %v", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
		log.Printf("connecting to agent %s with TLS", agent)
	} else {
		// Use insecure connection
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		log.Printf("connecting to agent %s (insecure)", agent)
	}

	cc, err := grpc.Dial(agent, dialOpts...)
	if err != nil {
		log.Fatal(err)
	}
	defer cc.Close()
	lc := localv1.NewLocalControlServiceClient(cc)
	req := &localv1.RegisterLocalRequest{ServiceId: serviceID, Version: version, RpcAddr: listen,
		Functions: []*localv1.LocalFunctionDescriptor{
			{Id: "http.generic_invoke", Version: version},
			{Id: "alertmanager.list_alerts", Version: version},
			{Id: "grafana.search_dashboards", Version: version},
		},
	}
	if _, err := lc.RegisterLocal(context.Background(), req); err != nil {
		log.Fatal(err)
	}
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		_, _ = lc.Heartbeat(context.Background(), &localv1.HeartbeatRequest{ServiceId: serviceID})
	}
}
