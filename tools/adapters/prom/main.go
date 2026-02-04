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

	"github.com/cuihairu/croupier/internal/nng"
	"github.com/cuihairu/croupier/pkg/protocol"
	localv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/agent/local/v1"
	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// prom-adapter implements FunctionService with function_id "prom.query_range".
// It registers itself to the Agent's LocalControlService (via NNG) and forwards QueryRange to Prometheus HTTP API.

type server struct {
	prom string
}

func (s *server) Invoke(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.InvokeResponse, error) {
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
	return &sdkv1.InvokeResponse{Payload: b}, nil
}

func main() {
	agent := os.Getenv("AGENT_ADDR") // e.g., 127.0.0.1:19091 (NNG port)
	if agent == "" {
		agent = "127.0.0.1:19091"
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

	log.Printf("prom-adapter listening on %s (HTTP)", listen)
	log.Printf("connecting to agent %s (NNG)", agent)

	// Create NNG client for agent communication
	nngClient := nng.NewClient(agent)
	if err := nngClient.Dial(); err != nil {
		log.Fatalf("Failed to connect to agent via NNG: %v", err)
	}
	defer nngClient.Close()

	// Register with agent
	regReq := &localv1.RegisterLocalRequest{
		ServiceId: serviceID,
		RpcAddr:   listen,
		Version:   version,
		Functions: []*localv1.LocalFunctionDescriptor{
			{Id: "prom.query", Version: version},
			{Id: "prom.query_range", Version: version},
		},
	}
	regData, err := proto.Marshal(regReq)
	if err != nil {
		log.Fatalf("Failed to marshal register request: %v", err)
	}

	ctx := context.Background()
	_, err = nngClient.Call(ctx, protocol.MsgRegisterLocalRequest, regData)
	if err != nil {
		log.Fatalf("Failed to register with agent: %v", err)
	}
	log.Printf("Registered with agent as service %s", serviceID)

	// keep heartbeating
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	hbReq := &localv1.HeartbeatRequest{ServiceId: serviceID}
	for range ticker.C {
		hbData, _ := proto.Marshal(hbReq)
		_, _ = nngClient.Call(ctx, protocol.MsgHeartbeatLocalRequest, hbData)
	}
}
