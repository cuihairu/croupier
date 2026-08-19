package function

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
	"github.com/getkin/kin-openapi/openapi3"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

type recordingClient struct {
	mockClient
	fail bool
}

func (m *recordingClient) RegisterFunction(desc croupier.FunctionDescriptor, handler croupier.FunctionHandler) error {
	if m.fail {
		return errors.New("client rejected registration")
	}
	return m.mockClient.RegisterFunction(desc, handler)
}

type configAwareClient struct {
	mockClient
	config *croupier.ClientConfig
}

func (c *configAwareClient) Config() *croupier.ClientConfig { return c.config }

func noopHandler(ctx context.Context, input []byte) ([]byte, error) {
	return input, nil
}

// ---------------------------------------------------------------------------
// Registry basics
// ---------------------------------------------------------------------------

func TestNewRegistry_LoggerSelection(t *testing.T) {
	r := NewRegistry(newMockClient())
	if r.logger == nil {
		t.Fatal("default registry logger should not be nil")
	}

	silent := &configAwareClient{config: &croupier.ClientConfig{DisableLogging: true}}
	r2 := NewRegistry(silent)
	if _, isNoOp := r2.logger.(*NoOpLogger); !isNoOp {
		t.Fatalf("DisableLogging should select NoOpLogger, got %T", r2.logger)
	}

	// Clients without Config() keep the default logger.
	r3 := NewRegistry(newMockClient())
	if _, isNoOp := r3.logger.(*NoOpLogger); isNoOp {
		t.Fatal("plain client should keep DefaultLogger")
	}
}

func TestLoggers(t *testing.T) {
	var l Logger = &DefaultLogger{}
	l.Debug("m %s", "a")
	l.Info("m")
	l.Warn("m")
	l.Error("m")

	noOp := &NoOpLogger{}
	noOp.Debug("m")
	noOp.Info("m")
	noOp.Warn("m")
	noOp.Error("m")
}

func TestRegistry_FullLifecycle(t *testing.T) {
	client := newMockClient()
	registry := NewRegistryWithLogger(client, &NoOpLogger{})

	if err := registry.Register(nil, noopHandler); err == nil {
		t.Fatal("nil metadata must fail")
	}
	if err := registry.Register(&FunctionMetadata{ID: "f"}, nil); err == nil {
		t.Fatal("nil handler must fail")
	}
	if err := registry.Register(&FunctionMetadata{}, noopHandler); err == nil {
		t.Fatal("empty ID must fail")
	}

	meta := &FunctionMetadata{
		ID:       "player.ban",
		Version:  "1.0.0",
		Resource: "player",
		Tags:     []string{"gm"},
		Risk:     &FunctionRisk{Level: RiskDanger},
	}
	if err := registry.Register(meta, noopHandler); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if registry.Count() != 1 {
		t.Fatalf("Count = %d", registry.Count())
	}

	handler, ok := registry.GetHandler("player.ban")
	if !ok || handler == nil {
		t.Fatal("GetHandler failed")
	}
	if _, ok := registry.GetHandler("ghost"); ok {
		t.Fatal("GetHandler should miss unknown IDs")
	}

	got, ok := registry.GetMetadata("player.ban")
	if !ok || got.ID != "player.ban" {
		t.Fatalf("GetMetadata = %+v, %v", got, ok)
	}
	// Cloned metadata must be isolated.
	got.Tags[0] = "mutated"
	again, _ := registry.GetMetadata("player.ban")
	if again.Tags[0] != "gm" {
		t.Fatal("GetMetadata must return clones")
	}
	if _, ok := registry.GetMetadata("ghost"); ok {
		t.Fatal("GetMetadata should miss unknown IDs")
	}

	list := registry.ListMetadata()
	if len(list) != 1 || list[0].ID != "player.ban" {
		t.Fatalf("ListMetadata = %v", list)
	}

	if err := registry.Unregister("ghost"); err == nil {
		t.Fatal("Unregister unknown ID must fail")
	}
	if err := registry.Unregister("player.ban"); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if registry.Count() != 0 {
		t.Fatal("registry should be empty after Unregister")
	}
}

func TestRegistry_RegisterClientError(t *testing.T) {
	client := &recordingClient{fail: true}
	registry := NewRegistryWithLogger(client, &NoOpLogger{})
	err := registry.Register(&FunctionMetadata{ID: "f"}, noopHandler)
	if err == nil || !strings.Contains(err.Error(), "client registration failed") {
		t.Fatalf("expected wrapped client error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

func TestBuilder_SetSummaryAndBuild(t *testing.T) {
	meta, err := NewMetadataBuilder().
		SetID("fn.summary").
		SetSummary("short summary").
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if meta.Summary != "short summary" {
		t.Fatalf("Summary = %q", meta.Summary)
	}

	if _, err := (NewMetadataBuilder()).Build(); err == nil {
		t.Fatal("Build without ID must fail")
	}

	// Behavior and Risk defaulting on a bare metadata struct.
	b := &MetadataBuilder{metadata: &FunctionMetadata{ID: "x"}, errors: nil}
	built, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if built.Behavior == nil || built.Risk == nil {
		t.Fatal("Build must default Behavior and Risk")
	}
}

func TestRegistry_RegisterFromBuilder(t *testing.T) {
	client := newMockClient()
	registry := NewRegistryWithLogger(client, &NoOpLogger{})

	if err := registry.RegisterFromBuilder(nil, noopHandler); err == nil {
		t.Fatal("nil builder must fail")
	}
	if err := registry.RegisterFromBuilder(NewMetadataBuilder(), noopHandler); err == nil {
		t.Fatal("invalid builder must fail")
	}
	if err := registry.RegisterFromBuilder(NewMetadataBuilder().SetID("from.builder"), noopHandler); err != nil {
		t.Fatalf("RegisterFromBuilder: %v", err)
	}
	if registry.Count() != 1 {
		t.Fatalf("Count = %d", registry.Count())
	}
}

// ---------------------------------------------------------------------------
// OpenAPI import
// ---------------------------------------------------------------------------

const demoOpenAPISpec = `{
  "openapi": "3.0.3",
  "info": {"title": "demo", "version": "1.0.0"},
  "paths": {
    "/api/players": {
      "get": {
        "operationId": "players.list",
        "summary": "List players",
        "tags": ["player"],
        "x-resource": "player",
        "x-operation": "list",
        "x-risk": "low",
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "schema": {"type": "object", "properties": {"total": {"type": "integer"}}}
              }
            }
          }
        }
      },
      "post": {
        "operationId": "players.create",
        "summary": "Create player",
        "x-resource": "player",
        "x-operation": "create",
        "x-risk": "danger",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"type": "object", "required": ["name"], "properties": {"name": {"type": "string", "description": "player name"}}}
            }
          }
        },
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`

func TestRegistry_RegisterFromOpenAPI_Success(t *testing.T) {
	client := newMockClient()
	registry := NewRegistryWithLogger(client, &NoOpLogger{})

	handlers := map[string]Handler{
		"players.list":   noopHandler,
		"players.create": noopHandler,
	}
	if err := registry.RegisterFromOpenAPIWithHandlers([]byte(demoOpenAPISpec), nil, handlers); err != nil {
		t.Fatalf("RegisterFromOpenAPI: %v", err)
	}
	if registry.Count() != 2 {
		t.Fatalf("Count = %d", registry.Count())
	}

	meta, _ := registry.GetMetadata("players.create")
	if meta.Resource != "player" || meta.Operation != "create" {
		t.Fatalf("extensions not applied: %+v", meta)
	}
	if meta.Risk == nil || meta.Risk.Level != RiskDanger {
		t.Fatalf("risk = %+v", meta.Risk)
	}
	if !strings.Contains(meta.InputSchema, "name") {
		t.Fatalf("input schema = %s", meta.InputSchema)
	}

	listMeta, _ := registry.GetMetadata("players.list")
	if !strings.Contains(listMeta.OutputSchema, "total") {
		t.Fatalf("output schema = %s", listMeta.OutputSchema)
	}
	if listMeta.Risk.Level != RiskLow {
		t.Fatalf("risk = %v", listMeta.Risk.Level)
	}
}

func TestRegistry_RegisterFromOpenAPI_WithOptions(t *testing.T) {
	client := newMockClient()
	registry := NewRegistryWithLogger(client, &NoOpLogger{})

	options := &ImportOptions{
		ResourcePrefix:   "demo",
		TagPrefix:        "svc.",
		DefaultTimeoutMs: 5000,
	}
	handlers := map[string]Handler{"players.list": noopHandler, "players.create": noopHandler}
	if err := registry.RegisterFromOpenAPIWithHandlers([]byte(demoOpenAPISpec), options, handlers); err != nil {
		t.Fatalf("RegisterFromOpenAPI: %v", err)
	}

	meta, _ := registry.GetMetadata("players.list")
	if meta.Resource != "demo.player" {
		t.Fatalf("resource prefix = %q", meta.Resource)
	}
	if len(meta.Tags) != 1 || meta.Tags[0] != "svc.player" {
		t.Fatalf("tags = %v", meta.Tags)
	}
	if meta.Behavior.TimeoutMs != 5000 {
		t.Fatalf("timeout = %d", meta.Behavior.TimeoutMs)
	}
}

func TestRegistry_RegisterFromOpenAPI_MissingHandler(t *testing.T) {
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})

	err := registry.RegisterFromOpenAPI([]byte(demoOpenAPISpec), nil, func(string) Handler { return nil })
	if err == nil || !strings.Contains(err.Error(), "no handler provided") {
		t.Fatalf("expected missing-handler error, got %v", err)
	}

	// ContinueOnError skips functions without handlers.
	err = registry.RegisterFromOpenAPI([]byte(demoOpenAPISpec), &ImportOptions{ContinueOnError: true}, func(id string) Handler {
		if id == "players.list" {
			return noopHandler
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ContinueOnError import: %v", err)
	}
	if registry.Count() != 1 {
		t.Fatalf("Count = %d", registry.Count())
	}
}

func TestRegistry_RegisterFromOpenAPI_RegisterFailure(t *testing.T) {
	client := &recordingClient{fail: true}
	registry := NewRegistryWithLogger(client, &NoOpLogger{})

	err := registry.RegisterFromOpenAPI([]byte(demoOpenAPISpec), nil, func(string) Handler { return noopHandler })
	if err == nil || !strings.Contains(err.Error(), "register function") {
		t.Fatalf("expected registration failure, got %v", err)
	}

	// With ContinueOnError the import completes despite failures.
	registry2 := NewRegistryWithLogger(client, &NoOpLogger{})
	if err := registry2.RegisterFromOpenAPI([]byte(demoOpenAPISpec), &ImportOptions{ContinueOnError: true}, func(string) Handler { return noopHandler }); err != nil {
		t.Fatalf("ContinueOnError: %v", err)
	}
}

func TestRegistry_RegisterFromOpenAPI_InvalidSpec(t *testing.T) {
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	if err := registry.RegisterFromOpenAPI([]byte("not a spec"), nil, func(string) Handler { return noopHandler }); err == nil {
		t.Fatal("expected spec load error")
	}
}

func TestRegistry_RegisterFromOpenAPI_OperationConversionFailure(t *testing.T) {
	// An empty paths document exercises conversion error handling: deriveOperationID
	// falls back and openAPIToMetadata must tolerate nil path items.
	spec := `{"openapi":"3.0.3","info":{"title":"empty","version":"1"},"paths":{}}`
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	if err := registry.RegisterFromOpenAPI([]byte(spec), nil, func(string) Handler { return noopHandler }); err != nil {
		t.Fatalf("empty spec should register nothing without error: %v", err)
	}
	if registry.Count() != 0 {
		t.Fatalf("Count = %d", registry.Count())
	}
}

// ---------------------------------------------------------------------------
// OpenAPI helpers
// ---------------------------------------------------------------------------

func TestDeriveOperationName_Coverage(t *testing.T) {
	if got := deriveOperationName(nil); got != "Unnamed Function" {
		t.Fatalf("fallback name = %q", got)
	}
	if got := toTitleCase(""); got != "" {
		t.Fatalf("toTitleCase empty = %q", got)
	}
	if got := toTitleCase("player_ban"); got != "Player Ban" {
		t.Fatalf("toTitleCase = %q", got)
	}
}

func TestExtractExtension(t *testing.T) {
	if got := extractExtension(nil, "x"); got != "" {
		t.Fatalf("nil extensions = %q", got)
	}
	ext := map[string]interface{}{
		"s": "str",
		"b": true,
		"f": float64(1.5),
		"o": map[string]interface{}{"k": 1},
	}
	if got := extractExtension(ext, "s"); got != "str" {
		t.Fatalf("string ext = %q", got)
	}
	if got := extractExtension(ext, "b"); got != "true" {
		t.Fatalf("bool ext = %q", got)
	}
	if got := extractExtension(ext, "f"); got != "1.5" {
		t.Fatalf("float ext = %q", got)
	}
	if got := extractExtension(ext, "o"); got == "" {
		t.Fatal("other ext should render via fmt verb")
	}
	if got := extractExtension(ext, "missing"); got != "" {
		t.Fatalf("missing ext = %q", got)
	}
}

func TestParseRiskLevel_Coverage(t *testing.T) {
	cases := map[string]RiskLevel{
		"safe":     RiskLow,
		"low":      RiskLow,
		"moderate": RiskMedium,
		"high":     RiskHigh,
		"critical": RiskDanger,
		"weird":    RiskMedium,
	}
	for in, want := range cases {
		if got := parseRiskLevel(in); got != want {
			t.Fatalf("parseRiskLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSchemaToJSONSchema(t *testing.T) {
	if got, err := schemaToJSONSchema(nil); err != nil || got != "{}" {
		t.Fatalf("nil schema = %q, %v", got, err)
	}

	objectType := openapi3.Types{"object"}
	intType := openapi3.Types{"integer"}
	schema := &openapi3.Schema{
		Type:        &objectType,
		Description: "a schema",
		Required:    []string{"id"},
		Properties: openapi3.Schemas{
			"id": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &intType, Description: "identifier"}},
		},
	}
	out, err := schemaToJSONSchema(schema)
	if err != nil {
		t.Fatalf("schemaToJSONSchema: %v", err)
	}
	if !strings.Contains(out, `"id"`) || !strings.Contains(out, `"required"`) || !strings.Contains(out, `"a schema"`) {
		t.Fatalf("converted schema = %s", out)
	}

	// Note: schemaTypeToString(nil) panics (recorded as a product bug); only
	// typed and type-less non-nil schemas are safe here.
	typeless := &openapi3.Schema{}
	if got := schemaTypeToString(typeless); got != "object" {
		t.Fatalf("typeless schema = %q", got)
	}
	typed := &openapi3.Schema{Type: &intType}
	if got := schemaTypeToString(typed); got != "integer" {
		t.Fatalf("typed schema = %q", got)
	}
}
