package function

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// ---------------------------------------------------------------------------
// Builders
// ---------------------------------------------------------------------------

func TestMetadataBuilder2FullChain(t *testing.T) {
	behavior := NewBehaviorBuilder().SetCommand().SetIdempotent(true).Build()
	risk := NewRiskBuilder().SetDanger().Build()

	metadata, err := NewMetadataBuilder().
		SetID("player.ban").
		SetVersion("2.0.0").
		SetResource("player").
		SetOperation("ban").
		SetPermission("player.ban").
		SetTags("gm", "risk").
		AddTag("extra").
		SetName("Ban Player").
		SetSummary("Bans a player").
		SetDescription("Bans a player account").
		SetInputSchema(`{"type":"object"}`).
		SetOutputSchema(`{"type":"object"}`).
		SetBehavior(behavior).
		SetRisk(risk).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(metadata.Tags) != 3 || metadata.Tags[2] != "extra" {
		t.Fatalf("tags = %v", metadata.Tags)
	}
	if metadata.Behavior.Mode != ModeCommand || !metadata.Behavior.Idempotent {
		t.Fatalf("behavior = %+v", metadata.Behavior)
	}
	if metadata.Risk.Level.String() != "danger" {
		t.Fatalf("risk = %v", metadata.Risk.Level)
	}
}

func TestMetadataBuilder2MissingIDFails(t *testing.T) {
	if _, err := NewMetadataBuilder().SetName("no id").Build(); err == nil {
		t.Fatal("missing ID must fail")
	} else if !strings.Contains(err.Error(), "ID is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMetadataBuilder2FillsDefaults(t *testing.T) {
	metadata := NewMetadataBuilder().SetID("fn").MustBuild()
	if metadata.Behavior == nil {
		t.Fatal("behavior default must be filled")
	}
	if metadata.Risk == nil || metadata.Risk.Level != RiskMedium {
		t.Fatalf("risk default = %+v", metadata.Risk)
	}
}

func TestMetadataBuilder2MustBuildPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustBuild should panic on validation failure")
		}
	}()
	NewMetadataBuilder().MustBuild()
}

func TestBehaviorBuilder2Defaults(t *testing.T) {
	behavior := NewBehaviorBuilder().Build()
	if behavior.Mode != ModeQuery || behavior.TimeoutMs != 30000 {
		t.Fatalf("unexpected defaults: %+v", behavior)
	}
	if behavior.RouteStrategy != RouteLB || behavior.Idempotent || behavior.Cacheable {
		t.Fatalf("unexpected defaults: %+v", behavior)
	}
}

func TestModeString2(t *testing.T) {
	if ModeQuery.String() == "" || ModeCommand.String() == "" {
		t.Fatal("mode strings must not be empty")
	}
	if ModeUnknown.String() == ModeQuery.String() {
		t.Fatal("modes must be distinguishable")
	}
}

// ---------------------------------------------------------------------------
// OpenAPI import behaviours
// ---------------------------------------------------------------------------

const allMethodsSpec = `
openapi: 3.0.3
info:
  title: Methods API
  version: 1.0.0
paths:
  /thing:
    get:
      operationId: thing_get
      x-resource: thing
      x-risk: low
      responses:
        '200':
          description: ok
    put:
      operationId: thing_put
      x-risk: critical
      responses:
        '200':
          description: ok
    post:
      operationId: thing_post
      responses:
        '200':
          description: ok
    delete:
      operationId: thing_del
      responses:
        '200':
          description: ok
    patch:
      operationId: thing_patch
      x-risk: danger
      responses:
        '200':
          description: ok
    head:
      operationId: thing_head
      responses:
        '200':
          description: ok
    options:
      operationId: thing_options
      responses:
        '200':
          description: ok
    trace:
      operationId: thing_trace
      responses:
        '200':
          description: ok
`

func TestRegisterFromOpenAPI2AllHttpMethods(t *testing.T) {
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	handlers := map[string]Handler{}
	for _, id := range []string{"thing_get", "thing_put", "thing_post", "thing_del", "thing_patch", "thing_head", "thing_options", "thing_trace"} {
		handlers[id] = noopHandler
	}

	if err := registry.RegisterFromOpenAPIWithHandlers([]byte(allMethodsSpec), nil, handlers); err != nil {
		t.Fatalf("RegisterFromOpenAPIWithHandlers: %v", err)
	}
	if registry.Count() != 8 {
		t.Fatalf("registered %d functions, want 8", registry.Count())
	}

	getMeta, _ := registry.GetMetadata("thing_get")
	if getMeta.Resource != "thing" || getMeta.Risk.Level != RiskLow {
		t.Fatalf("GET metadata = %+v", getMeta)
	}
	putMeta, _ := registry.GetMetadata("thing_put")
	if putMeta.Risk.Level != RiskDanger {
		t.Fatalf("critical should map to danger, got %v", putMeta.Risk.Level)
	}
}

func TestRegisterFromOpenAPI2NonJsonContentIgnored(t *testing.T) {
	spec := `
openapi: 3.0.3
info:
  title: Content API
  version: 1.0.0
paths:
  /upload:
    post:
      operationId: upload_file
      requestBody:
        content:
          text/plain:
            schema:
              type: string
      responses:
        '200':
          description: ok
`
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	if err := registry.RegisterFromOpenAPIWithHandlers([]byte(spec), nil, map[string]Handler{"upload_file": noopHandler}); err != nil {
		t.Fatalf("register: %v", err)
	}
	metadata, _ := registry.GetMetadata("upload_file")
	if metadata.InputSchema != "" {
		t.Fatalf("non-JSON content must not produce an input schema, got %q", metadata.InputSchema)
	}
	if metadata.OutputSchema != "" {
		t.Fatalf("no JSON 200 response must not produce an output schema, got %q", metadata.OutputSchema)
	}
}

func TestRegisterFromOpenAPI2InvalidSpecFails(t *testing.T) {
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	if err := registry.RegisterFromOpenAPI([]byte("not: valid: yaml:"), nil, func(string) Handler { return noopHandler }); err == nil {
		t.Fatal("invalid spec must fail")
	}
}

func TestRegisterFromOpenAPI2MissingHandlerFails(t *testing.T) {
	spec := `
openapi: 3.0.3
info:
  title: X
  version: 1.0.0
paths:
  /a:
    get:
      operationId: need_handler
      responses:
        '200':
          description: ok
`
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	err := registry.RegisterFromOpenAPI([]byte(spec), nil, func(string) Handler { return nil })
	if err == nil || !strings.Contains(err.Error(), "no handler provided") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Registry list behaviour
// ---------------------------------------------------------------------------

func TestRegistry2ListMetadataIsIndependent(t *testing.T) {
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	if err := registry.Register(&FunctionMetadata{ID: "a", Risk: &FunctionRisk{Level: RiskLow}}, noopHandler); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if err := registry.Register(&FunctionMetadata{ID: "b"}, noopHandler); err != nil {
		t.Fatalf("register b: %v", err)
	}

	list := registry.ListMetadata()
	if len(list) != 2 {
		t.Fatalf("list length = %d", len(list))
	}
	// Mutating the returned copies must not affect the registry.
	for _, metadata := range list {
		metadata.ID = "mutated"
	}
	if again := registry.ListMetadata(); again[0].ID == "mutated" {
		t.Fatal("ListMetadata must return cloned copies")
	}
}

func TestRegistry2UnregisterUnknownFails(t *testing.T) {
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	if err := registry.Unregister("ghost"); err == nil {
		t.Fatal("unregistering an unknown function must fail")
	}
}

func TestOpenAPI2EmptyOperationExtensionsDefaultRisk(t *testing.T) {
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	paths := &openapi3.Paths{}
	item := &openapi3.PathItem{}
	item.Get = &openapi3.Operation{OperationID: "plain"}
	paths.Set("/p", item)
	metadatas, err := registry.openAPIToMetadata(&openapi3.T{Paths: paths}, nil)
	if err != nil {
		t.Fatalf("openAPIToMetadata: %v", err)
	}
	if len(metadatas) != 1 {
		t.Fatalf("metadata count = %d", len(metadatas))
	}
	if metadatas[0].Risk.Level != RiskMedium {
		t.Fatalf("default risk = %v", metadatas[0].Risk.Level)
	}
	if metadatas[0].Behavior.TimeoutMs != 30000 {
		t.Fatalf("default timeout = %d", metadatas[0].Behavior.TimeoutMs)
	}
}
