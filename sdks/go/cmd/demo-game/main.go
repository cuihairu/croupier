// Command demo-game is the persistent demo game process: a simulated
// integrator that dials the agent SDK gateway (19091) through the public Go
// SDK, registers the full game-ops contract surface (34 functions), and
// serves them against a durable exactly-once store.
//
// Typical runs:
//
//	demo-game -agent 127.0.0.1:19091 -seed 7 -dsn /var/lib/demo-game/demo.db
//	demo-game -register-only            # verify registration, exit
//
// See docs/design/demo-game.md for the contract surface, idempotency
// protocol and deployment topology.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuihairu/croupier/sdks/go/cmd/demo-game/httpapi"
	"github.com/cuihairu/croupier/sdks/go/cmd/demo-game/store"
	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func main() {
	cfg := config{}
	flag.StringVar(&cfg.agentAddr, "agent", "127.0.0.1:19091", "agent local SDK gateway address (host:port)")
	flag.StringVar(&cfg.gameID, "game-id", "demo-game", "game scope id")
	flag.StringVar(&cfg.env, "env", "demo", "environment scope")
	flag.StringVar(&cfg.serviceID, "service-id", "demo-game", "provider service id")
	flag.StringVar(&cfg.serviceVersion, "service-version", "1.0.0", "provider service version")
	flag.IntVar(&cfg.timeoutSeconds, "timeout", 10, "agent connection timeout in seconds")
	flag.BoolVar(&cfg.insecure, "insecure", true, "connect to the agent without TLS (demo default)")
	flag.BoolVar(&cfg.registerOnly, "register-only", false, "register functions then exit without serving")
	flag.StringVar(&cfg.dsn, "dsn", "", "demo store DSN (postgres://... or sqlite path; empty = in-memory)")
	flag.IntVar(&cfg.seed, "seed", 0, "seed N for the deterministic dataset (0 = never seed)")
	flag.BoolVar(&cfg.reseed, "reseed", false, "wipe and re-seed even if the seed marker already matches")
	flag.StringVar(&cfg.httpAddr, "http-addr", "127.0.0.1:8090", "health endpoint listen address")
	flag.StringVar(&cfg.anchor, "anchor", "2026-11-01T00:00:00Z", "fixed seed anchor timestamp (RFC3339, never wall-clock)")
	flag.Parse()

	if err := run(cfg); err != nil {
		log.Fatalf("demo-game: %v", err)
	}
}

type config struct {
	agentAddr      string
	gameID         string
	env            string
	serviceID      string
	serviceVersion string
	timeoutSeconds int
	insecure       bool
	registerOnly   bool
	dsn            string
	seed           int
	reseed         bool
	httpAddr       string
	anchor         string
}

func run(cfg config) error {
	// Install the signal handler before anything prints a readiness line:
	// orchestrators send SIGTERM as soon as the process looks ready.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	anchorMS, err := parseAnchor(cfg.anchor)
	if err != nil {
		return err
	}

	backend, err := store.Open(cfg.dsn)
	if err != nil {
		return err
	}
	st, err := store.NewStore(cfg.gameID, cfg.env, backend)
	if err != nil {
		_ = backend.Close()
		return err
	}
	defer st.Close()

	if cfg.seed > 0 {
		seeded, err := st.EnsureSeed(cfg.seed, anchorMS, cfg.reseed)
		if err != nil {
			return fmt.Errorf("seed: %w", err)
		}
		if seeded {
			log.Printf("demo-game: seeded dataset N=%d (anchor %s)", cfg.seed, cfg.anchor)
		}
	}

	client := croupier.NewClient(&croupier.ClientConfig{
		AgentAddr:      cfg.agentAddr,
		GameID:         cfg.gameID,
		Env:            cfg.env,
		ServiceID:      cfg.serviceID,
		ServiceVersion: cfg.serviceVersion,
		TimeoutSeconds: cfg.timeoutSeconds,
		Insecure:       cfg.insecure,
	})
	if err := registerGameFunctions(client, st); err != nil {
		_ = client.Close()
		return err
	}
	log.Printf("demo-game: %d functions registered (game=%s env=%s service=%s@%s)",
		34, cfg.gameID, cfg.env, cfg.serviceID, cfg.serviceVersion)

	if cfg.registerOnly {
		_ = client.Close()
		log.Print("demo-game: register-only, exiting")
		return nil
	}

	// Health endpoint so the deployment stack (compose healthcheck) can see
	// the process is up even before/while the agent link reconnects.
	var ready httpapi.Ready
	var httpLn net.Listener
	if cfg.httpAddr != "" {
		httpLn, err = net.Listen("tcp", cfg.httpAddr)
		if err != nil {
			_ = client.Close()
			return fmt.Errorf("health listener: %w", err)
		}
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/healthz", httpapi.Healthz)
			mux.HandleFunc("/readyz", httpapi.Readyz(&ready))
			srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
			if err := srv.Serve(httpLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("demo-game: health server: %v", err)
			}
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() { serveErr <- client.Serve(ctx) }()
	ready.SetReady(true)

	log.Printf("demo-game: serving (agent %s, store %s, health %s)",
		cfg.agentAddr, storeDescribe(cfg.dsn), cfg.httpAddr)

	select {
	case sig := <-quit:
		log.Printf("demo-game: %v, shutting down", sig)
		cancel()
		if err := client.Stop(); err != nil {
			log.Printf("demo-game: stop: %v", err)
		}
	case err := <-serveErr:
		if err != nil && !errors.Is(err, context.Canceled) {
			_ = client.Close()
			if httpLn != nil {
				_ = httpLn.Close()
			}
			return fmt.Errorf("serve: %w", err)
		}
	}
	ready.SetReady(false)
	if err := client.Close(); err != nil {
		log.Printf("demo-game: close: %v", err)
	}
	if httpLn != nil {
		_ = httpLn.Close()
	}
	log.Print("demo-game: stopped")
	return nil
}

func parseAnchor(s string) (int64, error) {
	if s == "" {
		return store.DefaultSeedAnchor.UnixMilli(), nil
	}
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0, fmt.Errorf("parse -anchor %q: %w", s, err)
	}
	return ts.UnixMilli(), nil
}

// storeDescribe renders a short store identifier for logs (no credentials).
func storeDescribe(dsn string) string {
	if dsn == "" {
		return "memory"
	}
	if name, err := store.DatabaseName(dsn); err == nil && name != "" {
		return name
	}
	if len(dsn) > 40 {
		return dsn[:40] + "..."
	}
	return dsn
}
