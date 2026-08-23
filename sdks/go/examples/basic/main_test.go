package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// recordingClient captures RegisterFunction calls without opening connections.
type recordingClient struct {
	registered map[string]croupier.FunctionDescriptor
	failOn     map[string]bool
}

func newRecordingClient() *recordingClient {
	return &recordingClient{
		registered: make(map[string]croupier.FunctionDescriptor),
		failOn:     make(map[string]bool),
	}
}

func (r *recordingClient) RegisterFunction(desc croupier.FunctionDescriptor, handler croupier.FunctionHandler) error {
	if r.failOn[desc.ID] {
		return errors.New("rejected")
	}
	r.registered[desc.ID] = desc
	return nil
}
func (r *recordingClient) Connect(ctx context.Context) error { return nil }
func (r *recordingClient) Serve(ctx context.Context) error   { return nil }
func (r *recordingClient) Stop() error                       { return nil }
func (r *recordingClient) Close() error                      { return nil }

func TestBasicRegisterFunctionsRegistersBoth(t *testing.T) {
	client := newRecordingClient()
	if err := registerFunctions(client); err != nil {
		t.Fatalf("registerFunctions: %v", err)
	}

	for _, id := range []string{"player.ban", "item.create"} {
		if _, ok := client.registered[id]; !ok {
			t.Fatalf("function %s should be registered", id)
		}
	}
}

func TestBasicDescriptorsAreValid(t *testing.T) {
	client := newRecordingClient()
	if err := registerFunctions(client); err != nil {
		t.Fatalf("registerFunctions: %v", err)
	}

	ban := client.registered["player.ban"]
	if ban.Risk != "high" || ban.Resource != "player" || ban.Operation != "ban" {
		t.Fatalf("unexpected player.ban descriptor: %+v", ban)
	}
	if ban.Permission != "player:ban" || !ban.Enabled {
		t.Fatalf("unexpected player.ban contract: %+v", ban)
	}
	if err := croupier.ValidateFunctionDescriptor(&ban); err != nil {
		t.Fatalf("player.ban invalid: %v", err)
	}

	item := client.registered["item.create"]
	if item.Risk != "low" || item.Resource != "item" {
		t.Fatalf("unexpected item.create descriptor: %+v", item)
	}
	if err := croupier.ValidateFunctionDescriptor(&item); err != nil {
		t.Fatalf("item.create invalid: %v", err)
	}
}

func TestBasicRegisterFunctionsPropagatesFailure(t *testing.T) {
	client := newRecordingClient()
	client.failOn["player.ban"] = true
	if err := registerFunctions(client); err == nil || !strings.Contains(err.Error(), "player.ban") {
		t.Fatalf("expected player.ban failure, got %v", err)
	}

	client2 := newRecordingClient()
	client2.failOn["item.create"] = true
	if err := registerFunctions(client2); err == nil || !strings.Contains(err.Error(), "item.create") {
		t.Fatalf("expected item.create failure, got %v", err)
	}
}

func TestBasicGetenv(t *testing.T) {
	t.Setenv("EXAMPLE_BASIC_KEY", "value")
	if got := getenv("EXAMPLE_BASIC_KEY", "fallback"); got != "value" {
		t.Fatalf("getenv set = %q", got)
	}
	if got := getenv("EXAMPLE_BASIC_UNSET", "fallback"); got != "fallback" {
		t.Fatalf("getenv unset = %q", got)
	}
	t.Setenv("EXAMPLE_BASIC_EMPTY", "")
	if got := getenv("EXAMPLE_BASIC_EMPTY", "fallback"); got != "fallback" {
		t.Fatalf("getenv empty must fall back, got %q", got)
	}
}
