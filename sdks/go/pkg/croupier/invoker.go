package croupier

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/xeipuuv/gojsonschema"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// invoker implements the Invoker interface
type invoker struct {
	config *InvokerConfig
	conn   *grpc.ClientConn
	client functionv1.FunctionServiceClient
	mu     sync.RWMutex

	// Function schemas for validation
	schemas map[string]map[string]interface{}

	// Connection state
	connected bool
}

// NewInvoker creates a new Croupier invoker
func NewInvoker(config *InvokerConfig) Invoker {
	if config == nil {
		config = &InvokerConfig{
			Address:        "localhost:8080",
			TimeoutSeconds: 30,
			Insecure:       true,
		}
	}

	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = time.Duration(config.TimeoutSeconds) * time.Second
	}

	return &invoker{
		config:  config,
		schemas: make(map[string]map[string]interface{}),
	}
}

// Connect implements Invoker.Connect
func (i *invoker) Connect(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.connected {
		return nil
	}

	fmt.Printf("Connecting to server/agent at: %s\n", i.config.Address)

	// Setup connection options
	var opts []grpc.DialOption

	if i.config.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsCfg, err := buildTLSConfig(i.config)
		if err != nil {
			return err
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}

	// Create connection with timeout
	ctx, cancel := context.WithTimeout(ctx, i.config.DefaultTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, i.config.Address, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", i.config.Address, err)
	}

	i.conn = conn
	i.client = functionv1.NewFunctionServiceClient(conn)
	i.connected = true

	fmt.Printf("Connected to: %s\n", i.config.Address)
	return nil
}

// Invoke implements Invoker.Invoke
func (i *invoker) Invoke(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error) {
	if !i.connected {
		if err := i.Connect(ctx); err != nil {
			return "", fmt.Errorf("not connected to server: %w", err)
		}
	}

	// Client-side validation
	if schema, exists := i.schemas[functionID]; exists {
		if err := i.validatePayload(payload, schema); err != nil {
			return "", fmt.Errorf("payload validation failed for function %s: %w", functionID, err)
		}
	}

	req := &functionv1.InvokeRequest{
		FunctionId:     functionID,
		IdempotencyKey: options.IdempotencyKey,
		Payload:        []byte(payload),
		Metadata:       map[string]string{},
	}
	for k, v := range options.Headers {
		req.Metadata[k] = v
	}

	callCtx, cancel := i.callContext(ctx, options.Timeout)
	defer cancel()

	resp, err := i.client.Invoke(callCtx, req)
	if err != nil {
		return "", fmt.Errorf("invoke RPC failed: %w", err)
	}

	return string(resp.GetPayload()), nil
}

// StartJob implements Invoker.StartJob
func (i *invoker) StartJob(ctx context.Context, functionID, payload string, options InvokeOptions) (string, error) {
	if !i.connected {
		if err := i.Connect(ctx); err != nil {
			return "", fmt.Errorf("not connected to server: %w", err)
		}
	}

	req := &functionv1.InvokeRequest{
		FunctionId:     functionID,
		IdempotencyKey: options.IdempotencyKey,
		Payload:        []byte(payload),
		Metadata:       map[string]string{},
	}
	for k, v := range options.Headers {
		req.Metadata[k] = v
	}

	callCtx, cancel := i.callContext(ctx, options.Timeout)
	defer cancel()

	resp, err := i.client.StartJob(callCtx, req)
	if err != nil {
		return "", fmt.Errorf("start job RPC failed: %w", err)
	}

	return resp.GetJobId(), nil
}

// StreamJob implements Invoker.StreamJob
func (i *invoker) StreamJob(ctx context.Context, jobID string) (<-chan JobEvent, error) {
	eventCh := make(chan JobEvent, 10)

	if !i.connected {
		if err := i.Connect(ctx); err != nil {
			close(eventCh)
			return eventCh, fmt.Errorf("not connected to server: %w", err)
		}
	}

	req := &functionv1.JobStreamRequest{JobId: jobID}

	stream, err := i.client.StreamJob(ctx, req)
	if err != nil {
		close(eventCh)
		return eventCh, fmt.Errorf("stream job RPC failed: %w", err)
	}

	go func() {
		defer close(eventCh)
		for {
			evt, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					eventCh <- JobEvent{
						EventType: "error",
						JobID:     jobID,
						Error:     err.Error(),
						Done:      true,
					}
				}
				return
			}

			event := JobEvent{
				EventType: evt.GetType(),
				JobID:     jobID,
				Payload:   string(evt.GetPayload()),
				Done:      strings.EqualFold(evt.GetType(), "done"),
			}
			if evt.GetType() == "error" {
				event.Error = evt.GetMessage()
				event.Done = true
			} else if msg := evt.GetMessage(); msg != "" {
				event.Payload = msg
			}

			select {
			case eventCh <- event:
				if event.Done {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventCh, nil
}

// CancelJob implements Invoker.CancelJob
func (i *invoker) CancelJob(ctx context.Context, jobID string) error {
	if !i.connected {
		if err := i.Connect(ctx); err != nil {
			return fmt.Errorf("not connected to server: %w", err)
		}
	}

	req := &functionv1.CancelJobRequest{JobId: jobID}
	callCtx, cancel := i.callContext(ctx, 0)
	defer cancel()

	if _, err := i.client.CancelJob(callCtx, req); err != nil {
		return fmt.Errorf("cancel job RPC failed: %w", err)
	}
	return nil
}

// SetSchema implements Invoker.SetSchema
func (i *invoker) SetSchema(functionID string, schema map[string]interface{}) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.schemas[functionID] = schema
	fmt.Printf("Set schema for function: %s\n", functionID)
	return nil
}

// Close implements Invoker.Close
func (i *invoker) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.connected = false
	i.client = nil

	if i.conn != nil {
		i.conn.Close()
		i.conn = nil
	}

	i.schemas = make(map[string]map[string]interface{})
	fmt.Println("Invoker closed")
	return nil
}

// validatePayload validates payload against schema
func (i *invoker) validatePayload(payload string, schema map[string]interface{}) error {
	if len(schema) == 0 {
		if payload == "" {
			return fmt.Errorf("payload cannot be empty")
		}
		return nil
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	payloadLoader := gojsonschema.NewStringLoader(payload)

	result, err := gojsonschema.Validate(schemaLoader, payloadLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if result.Valid() {
		return nil
	}

	var errs []string
	for _, desc := range result.Errors() {
		errs = append(errs, desc.String())
	}

	return fmt.Errorf("payload validation failed: %s", strings.Join(errs, "; "))
}

func (i *invoker) callContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = i.config.DefaultTimeout
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func buildTLSConfig(cfg *InvokerConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{}

	// Root CA handling
	if cfg.CAFile != "" {
		caBytes, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caBytes) {
			return nil, fmt.Errorf("failed to append CA certificates")
		}
		tlsCfg.RootCAs = pool
	} else {
		systemPool, err := x509.SystemCertPool()
		if err == nil {
			tlsCfg.RootCAs = systemPool
		}
	}

	// Client certificate (optional)
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	// Infer server name for TLS verification
	if cfg.Address != "" {
		host := cfg.Address
		if h, _, err := net.SplitHostPort(cfg.Address); err == nil && h != "" {
			host = h
		}
		tlsCfg.ServerName = host
	}

	return tlsCfg, nil
}
