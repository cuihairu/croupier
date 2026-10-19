//go:build !no_go_plugin

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

type echo struct{}

func (echo) Describe(context.Context) (string, string, error) {
	return "echo", "1.0.0", nil
}

func (echo) Call(ctx context.Context, request string) (string, error) {
	if os.Getenv("ECHO_PLUGIN_FAIL") != "" {
		return "", errors.New("echo: forced failure")
	}
	if delay := os.Getenv("ECHO_PLUGIN_DELAY_MS"); delay != "" {
		var milliseconds time.Duration
		if _, err := fmt.Sscanf(delay, "%d", &milliseconds); err != nil {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(milliseconds * time.Millisecond):
		}
	}
	return request, nil
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{ProtocolVersion: 1, MagicCookieKey: "CROUPIER_ECHO", MagicCookieValue: "echo"},
		Plugins:         map[string]plugin.Plugin{"echo": &echoGRPCPlugin{impl: echo{}}},
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}

type echoGRPCPlugin struct {
	plugin.Plugin
	impl echo
}

func (p *echoGRPCPlugin) GRPCServer(*plugin.GRPCBroker, *grpc.Server) error { return nil }

func (p *echoGRPCPlugin) GRPCClient(context.Context, *plugin.GRPCBroker, *grpc.ClientConn) (any, error) {
	return p.impl, nil
}
