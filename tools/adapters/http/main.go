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

	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// http-adapter implements a generic HTTP invoker function: http.generic_invoke
// Request JSON: { method, url, headers: {..}, body }

type server struct{}

func (s *server) Invoke(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.InvokeResponse, error) {
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
				return &sdkv1.InvokeResponse{Payload: b}, nil
			}
			out, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "body": string(b)})
			return &sdkv1.InvokeResponse{Payload: out}, nil
		}
		return &sdkv1.InvokeResponse{Payload: b}, nil
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
				return &sdkv1.InvokeResponse{Payload: b}, nil
			}
			out, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "body": string(b)})
			return &sdkv1.InvokeResponse{Payload: out}, nil
		}
		return &sdkv1.InvokeResponse{Payload: b}, nil
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
			return &sdkv1.InvokeResponse{Payload: b}, nil
		}
		out, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "body": string(b)})
		return &sdkv1.InvokeResponse{Payload: out}, nil
	}
}

func main() {
	agent := os.Getenv("AGENT_ADDR")
	if agent == "" {
		agent = "127.0.0.1:19090" // TCP port
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

	log.Printf("http-adapter listening on %s (HTTP)", listen)
	log.Printf("connecting to agent %s (TCP)", agent)

	// Create TCP client for agent communication
	tcpClient, err := tcptr.NewClient(&tcptr.Config{
		Address:     agent,
		Insecure:    true,
		RecvTimeout: 10 * time.Second,
		SendTimeout: 10 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to create TCP client: %v", err)
	}
	defer tcpClient.Close()

	// Define JSON Schemas for function parameters
	genericInvokeInputSchema := `{
		"type": "object",
		"required": ["url"],
		"properties": {
			"method": {
				"type": "string",
				"enum": ["GET", "POST", "PUT", "DELETE", "PATCH"],
				"default": "GET",
				"description": "HTTP method"
			},
			"url": {
				"type": "string",
				"format": "uri",
				"description": "Target URL"
			},
			"headers": {
				"type": "object",
				"description": "HTTP headers"
			},
			"body": {
				"type": "string",
				"description": "Request body"
			}
		}
	}`

	alertmanagerListAlertsInputSchema := `{
		"type": "object",
		"required": ["base_url"],
		"properties": {
			"base_url": {
				"type": "string",
				"format": "uri",
				"description": "AlertManager base URL"
			},
			"silenced": {
				"type": "boolean",
				"description": "Filter by silenced status"
			},
			"inhibited": {
				"type": "boolean",
				"description": "Filter by inhibited status"
			},
			"active": {
				"type": "boolean",
				"description": "Filter by active status"
			}
		}
	}`

	grafanaSearchDashboardsInputSchema := `{
		"type": "object",
		"required": ["base_url"],
		"properties": {
			"base_url": {
				"type": "string",
				"format": "uri",
				"description": "Grafana base URL"
			},
			"query": {
				"type": "string",
				"description": "Search query"
			},
			"type": {
				"type": "string",
				"enum": ["dash-db", "dash-folder"],
				"default": "dash-db",
				"description": "Dashboard type"
			}
		}
	}`

	httpResponseSchema := `{
		"type": "object",
		"description": "HTTP response",
		"properties": {
			"status": {
				"type": "integer",
				"description": "HTTP status code"
			},
			"body": {
				"type": "string",
				"description": "Response body"
			}
		}
	}`

	// Register with agent using the provider-session handshake.
	regReq := &sdkv1.ProviderConnectRequest{
		ServiceId: serviceID,
		Version:   version,
		Functions: []*sdkv1.LocalFunctionDescriptor{
			{
				Id:           "http.generic_invoke",
				Version:      version,
				Tags:         []string{"http", "network", "generic"},
				Summary:      "Generic HTTP request invoker",
				Description:  "Execute arbitrary HTTP requests with configurable method, headers, and body",
				OperationId:  "httpGenericInvoke",
				InputSchema:  genericInvokeInputSchema,
				OutputSchema: httpResponseSchema,
				Category:     "integration",
				Risk:         "danger",
				Entity:       "",
				Operation:    "custom",
				Deprecated:   false,
			},
			{
				Id:           "alertmanager.list_alerts",
				Version:      version,
				Tags:         []string{"alertmanager", "monitoring", "alerts"},
				Summary:      "List AlertManager alerts",
				Description:  "Query AlertManager for alerts with optional filters",
				OperationId:  "alertmanagerListAlerts",
				InputSchema:  alertmanagerListAlertsInputSchema,
				OutputSchema: httpResponseSchema,
				Category:     "monitoring",
				Risk:         "safe",
				Entity:       "Alert",
				Operation:    "read",
			},
			{
				Id:           "grafana.search_dashboards",
				Version:      version,
				Tags:         []string{"grafana", "monitoring", "visualization"},
				Summary:      "Search Grafana dashboards",
				Description:  "Search for dashboards in Grafana",
				OperationId:  "grafanaSearchDashboards",
				InputSchema:  grafanaSearchDashboardsInputSchema,
				OutputSchema: httpResponseSchema,
				Category:     "monitoring",
				Risk:         "safe",
				Entity:       "Dashboard",
				Operation:    "read",
			},
		},
		SdkLanguage:           "go",
		SdkVersion:            version,
		ProtocolVersion:       "v1",
		SupportedTransports:   []string{"tcp"},
		TransportSecurityMode: "plain_tcp",
	}
	regData, err := proto.Marshal(regReq)
	if err != nil {
		log.Fatalf("Failed to marshal ProviderConnectRequest: %v", err)
	}

	ctx := context.Background()
	_, respData, err := tcpClient.Call(ctx, protocol.MsgProviderConnectRequest, regData)
	if err != nil {
		log.Fatalf("Failed to register with agent: %v", err)
	}
	regResp := &sdkv1.ProviderConnectResponse{}
	if err := proto.Unmarshal(respData, regResp); err != nil {
		log.Fatalf("Failed to parse ProviderConnectResponse: %v", err)
	}
	log.Printf("Registered with agent as service %s", serviceID)

	// keep heartbeating
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	hbReq := &sdkv1.ProviderHeartbeatRequest{
		ServiceId: serviceID,
		SessionId: regResp.GetSessionId(),
	}
	for range ticker.C {
		hbData, marshalErr := proto.Marshal(hbReq)
		if marshalErr != nil {
			log.Printf("Failed to marshal ProviderHeartbeatRequest: %v", marshalErr)
			continue
		}
		_, _, _ = tcpClient.Call(ctx, protocol.MsgProviderHeartbeatRequest, hbData)
	}
}
