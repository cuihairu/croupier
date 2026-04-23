// Package main demonstrates end-to-end TCP communication
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/protocol"
	"github.com/cuihairu/croupier/sdks/go/pkg/croupier/transport"
)

// simpleHandler implements transport.Handler for testing
type simpleHandler struct{}

func (h *simpleHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	// Echo back the received data with a prefix
	return []byte(fmt.Sprintf("Echo: %s", string(body))), nil
}

func main() {
	// Configuration
	addr := "127.0.0.1:19100"

	// Start server
	fmt.Printf("Starting TCP server on %s...\n", addr)
	serverCfg := &transport.Config{
		Address:     addr,
		Insecure:    true,
		RecvTimeout: 30 * time.Second,
	}

	server, err := transport.NewTCPServer(serverCfg, &simpleHandler{})
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := server.Serve(ctx); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for server to be ready
	select {
	case <-server.Ready():
		fmt.Println("Server is ready")
	case <-time.After(5 * time.Second):
		log.Fatal("Server failed to start")
	}

	// Give server a moment to fully initialize
	time.Sleep(100 * time.Millisecond)

	// Create client
	fmt.Printf("Connecting TCP client to %s...\n", addr)
	clientCfg := &transport.Config{
		Address:     addr,
		Insecure:    true,
		SendTimeout: 5 * time.Second,
	}

	client, err := transport.NewTCPClient(clientCfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("Client connected")

	// Test Invoke call
	reqBody := []byte(`{"message": "Hello TCP!"}`)
	fmt.Printf("Sending Invoke request: %s\n", string(reqBody))

	respMsgID, respBody, err := client.Call(context.Background(), protocol.MsgInvokeRequest, reqBody)
	if err != nil {
		log.Fatalf("Call failed: %v", err)
	}

	fmt.Printf("Received response: MsgID=0x%06X, Body=%s\n", respMsgID, string(respBody))

	// Verify the response
	if respMsgID != protocol.MsgInvokeResponse {
		log.Fatalf("Expected MsgID 0x%06X, got 0x%06X", protocol.MsgInvokeResponse, respMsgID)
	}

	expectedResp := fmt.Sprintf("Echo: %s", string(reqBody))
	if string(respBody) != expectedResp {
		log.Fatalf("Expected response '%s', got '%s'", expectedResp, string(respBody))
	}

	fmt.Println("\n=== End-to-end test PASSED ===")
}
