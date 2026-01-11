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
	"time"

	localv1 "github.com/cuihairu/croupier/generated/croupier/agent/local/v1"
	functionv1 "github.com/cuihairu/croupier/generated/croupier/function/v1"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// prom-adapter implements FunctionService with function_id "prom.query_range".
// It registers itself to the Agent's LocalControlService and forwards QueryRange to Prometheus HTTP API.

type server struct {
	functionv1.UnimplementedFunctionServiceServer
	prom string
}

func (s *server) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	// Expect JSON payloads
	// prom.query:       { expr, time? }
	// prom.query_range: { expr, start, end, step }
	var in map[string]string
	if err := json.Unmarshal(req.GetPayload(), &in); err != nil {
		return nil, fmt.Errorf("bad payload: %w", err)
	}
	httpClient := &http.Client{Timeout: 15 * time.Second}
	var u string
	switch req.GetFunctionId() {
	case "prom.query":
		q := url.Values{}
		q.Set("query", in["expr"]) // required
		if t := in["time"]; t != "" {
			q.Set("time", t)
		}
		u = s.prom + "/api/v1/query?" + q.Encode()
	default: // prom.query_range
		q := url.Values{}
		q.Set("query", in["expr"]) // required
		if v := in["start"]; v != "" {
			q.Set("start", v)
		}
		if v := in["end"]; v != "" {
			q.Set("end", v)
		}
		if v := in["step"]; v != "" {
			q.Set("step", v)
		}
		u = s.prom + "/api/v1/query_range?" + q.Encode()
	}
	// build request so we can inject headers (trace)
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
	if resp.StatusCode/100 != 2 {
		var b []byte
		b, _ = io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prom error: %s", string(b))
	}
	b, _ := io.ReadAll(resp.Body)
	return &functionv1.InvokeResponse{Payload: b}, nil
}

func (s *server) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	// Prom adapter is synchronous by nature, doesn't support asynchronous jobs
	// Return explicit error to make it clear this operation is not supported
	return nil, status.Error(codes.Unimplemented, "Prom adapter does not support asynchronous jobs. Use Invoke instead.")
}

func main() {
	agent := os.Getenv("AGENT_ADDR") // e.g., 127.0.0.1:19090
	if agent == "" {
		agent = "127.0.0.1:19090"
	}
	prom := os.Getenv("PROM_URL") // e.g., http://prometheus:9090
	if prom == "" {
		log.Fatal("PROM_URL required")
	}
	listen := os.Getenv("RPC_ADDR")
	if listen == "" {
		listen = ":20080"
	}
	serviceID := os.Getenv("SERVICE_ID")
	if serviceID == "" {
		serviceID = "prom-adapter"
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
		log.Printf("prom-adapter listening on %s with TLS", listen)
	} else {
		// Use insecure server
		log.Printf("prom-adapter listening on %s (insecure)", listen)
	}

	rpcConf := zrpc.RpcServerConf{ListenOn: listen}
	rpcConf.Name = serviceID
	gs := zrpc.MustNewServer(rpcConf, func(s *grpc.Server) {
		functionv1.RegisterFunctionServiceServer(s, &server{prom: prom})
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
			{Id: "prom.query", Version: version},
			{Id: "prom.query_range", Version: version},
		},
	}
	if _, err := lc.RegisterLocal(context.Background(), req); err != nil {
		log.Fatal(err)
	}
	// keep heartbeating
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		_, _ = lc.Heartbeat(context.Background(), &localv1.HeartbeatRequest{ServiceId: serviceID})
	}
}
