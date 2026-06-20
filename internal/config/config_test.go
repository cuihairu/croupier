package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUnmarshalConfig_CanonicalLowerCamelCase(t *testing.T) {
	input := `
server:
  host: 0.0.0.0
  port: 18780
database:
  driver: sqlite
  dataSource: test.db
storage:
  driver: file
  baseDir: data/uploads
cache:
  enabled: true
  type: redis
log:
  level: info
metrics:
  perFunction: true
sse:
  updateInterval: 2
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if cfg.Server.Port != 18780 {
		t.Fatalf("Server.Port = %d, want 18780", cfg.Server.Port)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Fatalf("Database.Driver = %q, want sqlite", cfg.Database.Driver)
	}
	if cfg.Storage.Driver != "file" {
		t.Fatalf("Storage.Driver = %q, want file", cfg.Storage.Driver)
	}
	if cfg.Storage.BaseDir != "data/uploads" {
		t.Fatalf("Storage.BaseDir = %q, want data/uploads", cfg.Storage.BaseDir)
	}
	if !cfg.Cache.Enabled {
		t.Fatalf("Cache.Enabled = false, want true")
	}
	if cfg.Logging.Level != "info" {
		t.Fatalf("Logging.Level = %q, want info", cfg.Logging.Level)
	}
	if !cfg.Metrics.PerFunction {
		t.Fatalf("Metrics.PerFunction = false, want true")
	}
	if cfg.SSE.UpdateInterval != 2 {
		t.Fatalf("SSE.UpdateInterval = %d, want 2", cfg.SSE.UpdateInterval)
	}
}

func TestUnmarshalConfig_LegacyUppercaseKeysRemainCompatible(t *testing.T) {
	input := `
Server:
  Host: 0.0.0.0
  Port: 18780
database:
  Driver: sqlite
  DataSource: test.db
Control:
  Addr: ":19090"
AgentDispatch:
  TaskRoutingDir: data
  ToAgentTLS:
    Enabled: true
BootstrapData:
  BaseDir: configs
descriptors:
  Dir: descriptors
schemas:
  Dir: schemas
storage:
  Driver: file
  BaseDir: data/uploads
cache:
  Enabled: true
  Type: redis
Log:
  Level: debug
metrics:
  PerFunction: true
sse:
  UpdateInterval: 2
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("Server.Host = %q, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Database.DataSource != "test.db" {
		t.Fatalf("Database.DataSource = %q, want test.db", cfg.Database.DataSource)
	}
	if cfg.Control.Addr != ":19090" {
		t.Fatalf("Control.Addr = %q, want :19090", cfg.Control.Addr)
	}
	if cfg.AgentDispatch.TaskRoutingDir != "data" {
		t.Fatalf("AgentDispatch.TaskRoutingDir = %q, want data", cfg.AgentDispatch.TaskRoutingDir)
	}
	if !cfg.AgentDispatch.ToAgentTLS.Enabled {
		t.Fatalf("AgentDispatch.ToAgentTLS.Enabled = false, want true")
	}
	if cfg.BootstrapData.BaseDir != "configs" {
		t.Fatalf("BootstrapData.BaseDir = %q, want configs", cfg.BootstrapData.BaseDir)
	}
	if cfg.Descriptors.Dir != "descriptors" {
		t.Fatalf("Descriptors.Dir = %q, want descriptors", cfg.Descriptors.Dir)
	}
	if cfg.Schemas.Dir != "schemas" {
		t.Fatalf("Schemas.Dir = %q, want schemas", cfg.Schemas.Dir)
	}
	if cfg.Storage.Driver != "file" {
		t.Fatalf("Storage.Driver = %q, want file", cfg.Storage.Driver)
	}
	if cfg.Storage.BaseDir != "data/uploads" {
		t.Fatalf("Storage.BaseDir = %q, want data/uploads", cfg.Storage.BaseDir)
	}
	if !cfg.Cache.Enabled {
		t.Fatalf("Cache.Enabled = false, want true")
	}
	if cfg.Cache.Type != "redis" {
		t.Fatalf("Cache.Type = %q, want redis", cfg.Cache.Type)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("Logging.Level = %q, want debug", cfg.Logging.Level)
	}
	if !cfg.Metrics.PerFunction {
		t.Fatalf("Metrics.PerFunction = false, want true")
	}
	if cfg.SSE.UpdateInterval != 2 {
		t.Fatalf("SSE.UpdateInterval = %d, want 2", cfg.SSE.UpdateInterval)
	}
}

func TestUnmarshalConfig_MultiGameDatabase(t *testing.T) {
	input := `
database:
  driver: postgres
  dataSource: "postgres://user:pass@localhost:5432/croupier_meta"
  multiGame: true
  gameDbPrefix: "gm_"
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if cfg.Database.Driver != "postgres" {
		t.Fatalf("Database.Driver = %q, want postgres", cfg.Database.Driver)
	}
	if cfg.Database.DataSource != "postgres://user:pass@localhost:5432/croupier_meta" {
		t.Fatalf("Database.DataSource = %q", cfg.Database.DataSource)
	}
	if !cfg.Database.MultiGame {
		t.Fatal("Database.MultiGame = false, want true")
	}
	if cfg.Database.GameDBPrefix != "gm_" {
		t.Fatalf("Database.GameDBPrefix = %q, want gm_", cfg.Database.GameDBPrefix)
	}
}

func TestUnmarshalConfig_MultiGameDefaultsFalse(t *testing.T) {
	input := `
database:
  driver: sqlite
  dataSource: test.db
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if cfg.Database.MultiGame {
		t.Fatal("Database.MultiGame should default to false")
	}
	if cfg.Database.GameDBPrefix != "" {
		t.Fatalf("Database.GameDBPrefix = %q, want empty default", cfg.Database.GameDBPrefix)
	}
}
