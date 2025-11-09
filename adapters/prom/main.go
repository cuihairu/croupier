package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"google.golang.org/grpc"
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
	// Not implemented for prom adapter
	return &functionv1.StartJobResponse{JobId: ""}, nil
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

	// start FunctionService server
	lis, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	gs := grpc.NewServer()
	functionv1.RegisterFunctionServiceServer(gs, &server{prom: prom})
	go func() {
		log.Printf("prom-adapter listening on %s", listen)
		if err := gs.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	// register to agent local control
	cc, err := grpc.Dial(agent, grpc.WithInsecure())
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
