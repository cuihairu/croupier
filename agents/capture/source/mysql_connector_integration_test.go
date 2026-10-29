//go:build integration

package source

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
)

func TestMySQLConnectorContract(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not configured")
	}
	const dbname = "capture_mcadapter_mysql"
	mustExecStmts(t, dsn,
		"DROP DATABASE IF EXISTS "+dbname,
		"CREATE DATABASE "+dbname,
		"CREATE TABLE "+dbname+".t1 (id INT PRIMARY KEY AUTO_INCREMENT, v VARCHAR(64))",
		fmt.Sprintf("INSERT INTO %s.t1 (v) VALUES ('seed')", dbname),
	)
	t.Cleanup(func() {
		mustExecStmts(t, dsn, "DROP DATABASE IF EXISTS "+dbname)
	})

	cfg := connector.SourceConfig{
		SourceID:   "mysql-itest",
		SourceType: "mysql",
		DSNEnv:     "TEST_MYSQL_DSN",
		Options: map[string]string{
			"serverId": "424243",
			"flavor":   "mysql",
			"tables":   dbname + ".t1",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	snapshotConnector := NewMySQLConnector(cfg, nil)
	if err := snapshotConnector.Open(ctx, cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = snapshotConnector.Close(ctx) }()

	disc, err := snapshotConnector.Discover(ctx, connector.DiscoveryRequest{Tables: []string{dbname + ".t1"}})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(disc.Tables) != 1 || disc.Tables[0].Name != dbname+".t1" {
		t.Fatalf("discover tables = %+v", disc.Tables)
	}
	if !disc.Capabilities.AfterImage || !disc.Capabilities.ResumableCheckpoint {
		t.Fatalf("capabilities = %+v", disc.Capabilities)
	}

	events, err := snapshotConnector.Poll(ctx, connector.PollRequest{Tables: []string{dbname + ".t1"}})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	mustExecStmts(t, dsn, fmt.Sprintf("INSERT INTO %s.t1 (v) VALUES ('marker-mc')", dbname))
	waitEvent := func(match func(connector.ChangeEvent) bool) connector.ChangeEvent {
		t.Helper()
		deadline := time.After(15 * time.Second)
		for {
			select {
			case ev, ok := <-events:
				if !ok {
					t.Fatal("event stream closed before marker")
				}
				if match(ev) {
					return ev
				}
			case <-deadline:
				t.Fatal("timed out waiting for marker")
			}
		}
	}
	insert := waitEvent(func(ev connector.ChangeEvent) bool {
		return ev.Operation == connector.OpInsert && string(ev.After["1"]) == `"marker-mc"`
	})
	if insert.Database != dbname || insert.Table != "t1" || insert.ColumnsMapped {
		t.Fatalf("identity = %+v", insert)
	}
	if insert.Position.Kind != "gtid" || !insert.Position.Valid() || !insert.Tx.End || insert.Tx.ID == "" {
		t.Fatalf("position/tx = %+v", insert)
	}

	caps := MySQLCapabilities()
	if caps.Snapshot || !caps.TransactionMetadata || caps.SchemaHistory {
		t.Fatalf("public recovery capabilities = %+v, want fail-closed (no snapshot/schema-history, txn-boundary required)", caps)
	}
	if _, err := snapshotConnector.Snapshot(ctx, connector.SnapshotRequest{}); err == nil {
		t.Fatal("public Snapshot must reject")
	}
	_, err = CurrentGTIDProbe(MySQLConfig{Flavor: "mariadb"})
	if err == nil {
		t.Fatal("MariaDB snapshot probe must reject")
	}

	before, err := CurrentGTIDProbe(MySQLConfig{
		Host: "127.0.0.1", Port: portFromDSN(t, dsn), User: userFromDSN(t, dsn), Password: passFromDSN(t, dsn), Flavor: "mysql",
	})
	if err != nil {
		t.Fatalf("gtid probe: %v", err)
	}
	position, err := before(ctx)
	if err != nil || position == "" {
		t.Fatalf("before position = %q, %v", position, err)
	}
	injected := NewMySQLConnector(cfg, nil)
	if err := injected.Open(ctx, cfg); err != nil {
		t.Fatalf("open invariant connector: %v", err)
	}
	defer func() { _ = injected.Close(ctx) }()
	injected.execSQL = func(ctx context.Context, query string) (sql.Result, error) {
		return nil, errors.New("injected snapshot consistency failure")
	}
	if _, err := injected.SnapshotWithProbe(ctx, connector.SnapshotRequest{Tables: []string{dbname + ".t1"}}, before); err == nil {
		t.Fatal("snapshot must reject failed consistency lock")
	}

	snapshots, err := snapshotConnector.SnapshotWithProbe(ctx, connector.SnapshotRequest{Tables: []string{dbname + ".t1"}}, before)
	if err != nil {
		t.Fatalf("snapshot with probe: %v", err)
	}
	mustExecStmts(t, dsn, fmt.Sprintf("INSERT INTO %s.t1 (v) VALUES ('after-snapshot')", dbname))
	var snapshotted []connector.ChangeEvent
	for event := range snapshots {
		if event.Position.Kind != "gtid" || event.Position.Value != position || !event.ColumnsMapped || !event.Tx.End {
			t.Fatalf("snapshot event = %+v", event)
		}
		snapshotted = append(snapshotted, event)
	}
	if len(snapshotted) != 2 || string(snapshotted[0].After["0"]) != "1" || string(snapshotted[1].After["0"]) != "2" {
		t.Fatalf("snapshot rows = %+v", snapshotted)
	}
	incremental := waitConnectorEvent(t, events, func(ev connector.ChangeEvent) bool {
		return ev.Operation == connector.OpInsert && string(ev.After["1"]) == `"after-snapshot"`
	})
	if err := snapshotConnector.AckCheckpoint(ctx, incremental.Position); err != nil {
		t.Fatalf("ack incremental: %v", err)
	}
	if _, err := snapshotConnector.Snapshot(ctx, connector.SnapshotRequest{}); err == nil {
		t.Fatal("generic Snapshot must stay unsupported after guarded capture")
	}
}

func waitConnectorEvent(t *testing.T, events <-chan connector.ChangeEvent, match func(connector.ChangeEvent) bool) connector.ChangeEvent {
	t.Helper()
	deadline := time.After(15 * time.Second)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatal("connector event stream closed before marker")
			}
			if match(event) {
				return event
			}
		case <-deadline:
			t.Fatal("timed out waiting for connector marker")
		}
	}
}
