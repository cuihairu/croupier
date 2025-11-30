package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	agentapp "github.com/cuihairu/croupier/internal/app/agent"
	"google.golang.org/grpc"
)

func main() {
	var (
		defaultServer = firstNonEmpty(os.Getenv("CROUPIER_SERVER_ADDR"), os.Getenv("CROUPIER_CONTROL_ADDR"), "127.0.0.1:18443")
		defaultGame   = os.Getenv("CROUPIER_GAME_ID")
		defaultEnv    = os.Getenv("CROUPIER_ENV")
		defaultVer    = firstNonEmpty(os.Getenv("CROUPIER_AGENT_VERSION"), "dev")
	)

	listen := flag.String("listen", ":18890", "gRPC listen address for local SDKs")
	server := flag.String("server", defaultServer, "Control server gRPC address")
	agentID := flag.String("agent-id", os.Getenv("CROUPIER_AGENT_ID"), "Agent identifier (defaults to hostname + timestamp if empty)")
	gameID := flag.String("game-id", defaultGame, "Game identifier used during registration")
	env := flag.String("env", defaultEnv, "Environment label sent to the server")
	version := flag.String("version", defaultVer, "Agent version reported upstream")
	announce := flag.String("announce", "", "Public RPC address advertised to the server (defaults to listen address)")
	flag.Parse()

	if *server == "" {
		fatal("server address is required (--server)")
	}
	if *agentID == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "agent"
		}
		*agentID = fmt.Sprintf("%s-%d", host, time.Now().Unix())
	}

	lis, err := net.Listen("tcp", *listen)
	if err != nil {
		fatal("failed to listen on %s: %v", *listen, err)
	}
	rpcAddr := strings.TrimSpace(*announce)
	if rpcAddr == "" {
		rpcAddr = lis.Addr().String()
	}

	app := agentapp.New(*server, *agentID)
	app.WithUpstreamMetadata(agentapp.UpstreamMetadata{
		GameID:  *gameID,
		Env:     *env,
		Version: *version,
		RPCAddr: rpcAddr,
	})

	grpcServer := grpc.NewServer()
	app.RegisterGRPC(grpcServer)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := app.Run(ctx); err != nil {
			slog.Error("upstream sync failed", "error", err)
			cancel()
		}
	}()

	go func() {
		slog.Info("agent gRPC server started", "listen", lis.Addr().String(), "public", rpcAddr)
		if err := grpcServer.Serve(lis); err != nil && ctx.Err() == nil {
			slog.Error("grpc serve failed", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down agentd")

	shutdown := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(shutdown)
	}()

	select {
	case <-shutdown:
	case <-time.After(5 * time.Second):
		grpcServer.Stop()
	}

	time.Sleep(100 * time.Millisecond)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func fatal(format string, args ...interface{}) {
	slog.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}
