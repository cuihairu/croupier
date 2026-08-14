package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// dev-fixture starts the named "real-dashboard" E2E fixture: a genuine
// Server + Agent + Go SDK + /players OpenAPI provider with a dedicated clean
// scope. It prints a single FIXTURE_READY line once all components are
// listening, then blocks until SIGINT/SIGTERM and cleans up only its own
// scope before exiting.
var fixtureCmd = &cobra.Command{
	Use:    "dev-fixture",
	Short:  "Start the real-dashboard E2E fixture (server+agent+sdk+provider)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := DashboardFixtureOptions{
			BaseDir:        fixtureBaseDir,
			GameID:         fixtureGameID,
			Env:            fixtureEnv,
			HTTPAddr:       fixtureHTTPAddr,
			ControlAddr:    fixtureControlAddr,
			AgentLocalAddr: fixtureAgentLocalAddr,
			ProviderAddr:   fixtureProviderAddr,
			FixtureAddr:    fixtureAPIAddr,
			BootstrapDir:   fixtureBootstrapDir,
		}
		fixture, err := StartDashboardFixture(context.Background(), opts)
		if err != nil {
			return err
		}

		ready := map[string]interface{}{
			"gameId":       fixture.GameID,
			"env":          fixture.Env,
			"httpAddr":     fixture.HTTPAddr,
			"controlAddr":  fixture.ControlAddr,
			"agentAddr":    fixture.AgentLocalAddr,
			"providerAddr": fixture.ProviderAddr,
			"fixtureAddr":  fixture.FixtureAddr,
			"baseDir":      fixture.BaseDir,
		}
		raw, _ := json.Marshal(ready)
		fmt.Printf("FIXTURE_READY %s\n", raw)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		if err := fixture.CleanupScope(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "fixture scope cleanup failed: %v\n", err)
		}
		return fixture.Close(context.Background())
	},
}

var (
	fixtureBaseDir        string
	fixtureGameID         string
	fixtureEnv            string
	fixtureHTTPAddr       string
	fixtureControlAddr    string
	fixtureAgentLocalAddr string
	fixtureProviderAddr   string
	fixtureAPIAddr        string
	fixtureBootstrapDir   string
)

func init() {
	fixtureCmd.Flags().StringVar(&fixtureBaseDir, "dir", "", "fixture state directory (default: temp dir, removed on exit)")
	fixtureCmd.Flags().StringVar(&fixtureGameID, "game-id", "e2e-game", "fixture scope game id")
	fixtureCmd.Flags().StringVar(&fixtureEnv, "env", "e2e", "fixture scope env")
	fixtureCmd.Flags().StringVar(&fixtureHTTPAddr, "http-addr", "127.0.0.1:18780", "server HTTP listen address")
	fixtureCmd.Flags().StringVar(&fixtureControlAddr, "control-addr", "127.0.0.1:0", "server control TCP listen address")
	fixtureCmd.Flags().StringVar(&fixtureAgentLocalAddr, "agent-local-addr", "127.0.0.1:0", "agent local SDK TCP listen address")
	fixtureCmd.Flags().StringVar(&fixtureProviderAddr, "provider-addr", "127.0.0.1:0", "/players provider listen address")
	fixtureCmd.Flags().StringVar(&fixtureAPIAddr, "fixture-addr", "127.0.0.1:0", "fixture control API listen address")
	fixtureCmd.Flags().StringVar(&fixtureBootstrapDir, "bootstrap-dir", "", "bootstrap data dir with admins.json (default: repo configs/)")
	rootCmd.AddCommand(fixtureCmd)
}
