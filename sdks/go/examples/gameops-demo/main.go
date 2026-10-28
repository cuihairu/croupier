// Command gameops-demo is the E2 demo server. It mirrors the E1 game-ops
// browser seed as a mutable process-local store and registers the exact
// function contracts through the existing SDK→Agent registration chain, so a
// published demo page's bindings resolve to these callable functions.
//
// Usage (start croupier-agent first, or let the embedded Agent connect):
//
//	go run ./examples/gameops-demo -server 127.0.0.1:19090 \
//	  -game-id croupier-demo -env local-demo
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

var exit = os.Exit

type runConfig struct {
	agentAddr    string
	gameID       string
	sdkenv       string
	serviceID    string
	serviceVer   string
	timeoutSec   int
	insecure     bool
	registerOnly bool
}

func main() {
	exit(runMain(os.Args[0], os.Args[1:], os.Stderr))
}

func runMain(name string, args []string, output io.Writer) int {
	fs, cfg := newFlagSet(name, output)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	log.Printf("gameops-demo starting: agent=%s game=%s env=%s service=%s",
		cfg.agentAddr, cfg.gameID, cfg.sdkenv, cfg.serviceID)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := newGameOpsStore()
	client := croupier.NewClient(&croupier.ClientConfig{
		AgentAddr:      cfg.agentAddr,
		GameID:         cfg.gameID,
		Env:            cfg.sdkenv,
		ServiceID:      cfg.serviceID,
		ServiceVersion: cfg.serviceVer,
		TimeoutSeconds: cfg.timeoutSec,
		Insecure:       cfg.insecure,
		InboundWorkers: 1,
	})

	if err := registerGameOpsFunctions(client, store); err != nil {
		log.Printf("register gameops functions failed: %v", err)
		return 1
	}
	if cfg.registerOnly {
		log.Printf("register-only mode: %d functions registered, exiting", len(gameOpsFunctionDefinitions(store)))
		return 0
	}
	if err := client.Serve(ctx); err != nil {
		log.Printf("serve failed: %v", err)
		return 1
	}
	if err := client.Close(); err != nil {
		log.Printf("close failed: %v", err)
		return 1
	}
	return 0
}

func newFlagSet(name string, output io.Writer) (*flag.FlagSet, *runConfig) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	cfg := &runConfig{}
	fs.StringVar(&cfg.agentAddr, "agent", "127.0.0.1:19091", "Croupier Agent local SDK listen address")
	fs.StringVar(&cfg.gameID, "game-id", "croupier-demo", "demo game ID (matches E1 scope)")
	fs.StringVar(&cfg.sdkenv, "env", "local-demo", "demo environment (matches E1 scope)")
	fs.StringVar(&cfg.serviceID, "service-id", "gameops-demo-service", "service identifier for this provider")
	fs.StringVar(&cfg.serviceVer, "service-version", "1.0.0", "service semver version")
	fs.IntVar(&cfg.timeoutSec, "timeout", 30, "connection timeout seconds")
	fs.BoolVar(&cfg.insecure, "insecure", true, "use insecure TCP (dev)")
	fs.BoolVar(&cfg.registerOnly, "register-only", false, "register and exit without Serve (smoke/test)")
	return fs, cfg
}
