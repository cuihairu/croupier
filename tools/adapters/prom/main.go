package main

import (
	"context"
	"log"
	"os"
	"time"

	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// prom-adapter implements FunctionService with function_id "prom.query_range".
// It registers itself to the Agent via the provider-session TCP handshake and forwards QueryRange to Prometheus HTTP API.

func main() {
	agent := os.Getenv("AGENT_ADDR") // e.g., 127.0.0.1:19090 (TCP port)
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

	log.Printf("prom-adapter listening on %s (HTTP)", listen)
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
	queryInputSchema := `{
		"type": "object",
		"required": ["expr"],
		"properties": {
			"expr": {
				"type": "string",
				"description": "PromQL expression to query"
			},
			"time": {
				"type": "string",
				"description": "Evaluation timestamp (RFC3339 or Unix timestamp)"
			}
		}
	}`

	queryRangeInputSchema := `{
		"type": "object",
		"required": ["expr", "start", "end"],
		"properties": {
			"expr": {
				"type": "string",
				"description": "PromQL expression to query"
			},
			"start": {
				"type": "string",
				"description": "Start timestamp (RFC3339 or Unix timestamp)"
			},
			"end": {
				"type": "string",
				"description": "End timestamp (RFC3339 or Unix timestamp)"
			},
			"step": {
				"type": "string",
				"description": "Query resolution step width (e.g., '15s', '1m')",
				"default": "15s"
			}
		}
	}`

	promResponseSchema := `{
		"type": "object",
		"description": "Prometheus query response",
		"properties": {
			"status": {
				"type": "string",
				"enum": ["success", "error"]
			},
			"data": {
				"type": "object",
				"description": "Query result data"
			},
			"errorType": {
				"type": "string",
				"description": "Error type (if status is 'error')"
			},
			"error": {
				"type": "string",
				"description": "Error message (if status is 'error')"
			}
		}
	}`

	// Register with agent using the provider-session handshake.
	regReq := &sdkv1.ProviderConnectRequest{
		ServiceId: serviceID,
		Version:   version,
		Functions: []*sdkv1.ProviderFunctionDescriptor{
			{
				Id:           "prom.query",
				Version:      version,
				Tags:         []string{"prometheus", "monitoring", "metrics"},
				Summary:      "Execute instant PromQL query",
				Description:  "Evaluate a PromQL expression at a single timestamp or now",
				OperationId:  "promQuery",
				InputSchema:  queryInputSchema,
				OutputSchema: promResponseSchema,
				Resource:     "prom",
				Risk:         "safe",
				Operation:    "read",
			},
			{
				Id:           "prom.query_range",
				Version:      version,
				Tags:         []string{"prometheus", "monitoring", "metrics", "timeseries"},
				Summary:      "Execute PromQL range query",
				Description:  "Evaluate a PromQL expression over a time range",
				OperationId:  "promQueryRange",
				InputSchema:  queryRangeInputSchema,
				OutputSchema: promResponseSchema,
				Resource:     "prom",
				Risk:         "safe",
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
