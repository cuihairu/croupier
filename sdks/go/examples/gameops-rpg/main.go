// Command gameops-rpg is the complex-game demo server. It simulates an RPG
// world (players, items, quests, mail compensation, orders, bans) that
// evolves through a periodic tick, and registers its function contracts
// through the SDK→Agent chain so the platform exercises the full ops chain
// (invoke, approval, task execution) against living game state.
//
// Usage (start croupier-agent first):
//
//	go run ./examples/gameops-rpg -server 127.0.0.1:19090 \
//	  -game-id default -env dev
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	stateFile    string
	tickSeconds  int
	rngSeed      int64
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
	log.Printf("gameops-rpg starting: agent=%s game=%s env=%s service=%s tick=%ds state=%s",
		cfg.agentAddr, cfg.gameID, cfg.sdkenv, cfg.serviceID, cfg.tickSeconds, cfg.stateFile)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := newRPGStore(rand.New(rand.NewSource(cfg.rngSeed)), cfg.stateFile)
	if cfg.tickSeconds > 0 && !cfg.registerOnly {
		go runTickLoop(ctx, store, cfg.tickSeconds)
	}
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

	if err := registerRPGFunctions(client, store); err != nil {
		log.Printf("register gameops-rpg functions failed: %v", err)
		return 1
	}
	if cfg.registerOnly {
		log.Printf("register-only mode: %d functions registered, exiting", len(rpgFunctionDefinitions(store)))
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

// runTickLoop drives the world simulation until ctx is cancelled.
func runTickLoop(ctx context.Context, store *rpgStore, tickSeconds int) {
	interval := time.Duration(tickSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	local := rand.New(rand.NewSource(time.Now().UnixNano() + 1))
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			store.Tick(time.Now().UnixMilli(), local)
		}
	}
}

func newFlagSet(name string, output io.Writer) (*flag.FlagSet, *runConfig) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	cfg := &runConfig{}
	fs.StringVar(&cfg.agentAddr, "agent", "127.0.0.1:19091", "Croupier Agent local SDK listen address")
	fs.StringVar(&cfg.gameID, "game-id", "default", "demo game ID (platform scope)")
	fs.StringVar(&cfg.sdkenv, "env", "dev", "demo environment (platform scope)")
	fs.StringVar(&cfg.serviceID, "service-id", "gameops-rpg-service", "service identifier for this provider")
	fs.StringVar(&cfg.serviceVer, "service-version", "1.0.0", "service semver version")
	fs.IntVar(&cfg.timeoutSec, "timeout", 30, "connection timeout seconds")
	fs.BoolVar(&cfg.insecure, "insecure", true, "use insecure TCP (dev)")
	fs.BoolVar(&cfg.registerOnly, "register-only", false, "register and exit without Serve (smoke/test)")
	fs.StringVar(&cfg.stateFile, "state-file", "", "JSON file persisting the world snapshot across restarts")
	fs.IntVar(&cfg.tickSeconds, "tick-seconds", 5, "world simulation tick interval seconds (0 disables)")
	fs.Int64Var(&cfg.rngSeed, "seed", time.Now().UnixNano(), "rng seed for the tick simulation (deterministic when fixed)")
	return fs, cfg
}
