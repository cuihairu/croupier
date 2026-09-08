// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package function

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// TestRegisterFromOpenAPIMissingHandlerContinueOnError covers the
// ContinueOnError branch: operations without a matching handler are skipped
// instead of aborting the whole import.
func TestRegisterFromOpenAPIMissingHandlerContinueOnError(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.0.3",
		"info": {"title": "Gap API", "version": "1.0.0"},
		"paths": {
			"/present": {"post": {"operationId": "gap.present", "summary": "Present", "responses": {"200": {"description": "OK"}}}},
			"/absent": {"post": {"operationId": "gap.absent", "summary": "Absent", "responses": {"200": {"description": "OK"}}}}
		}
	}`)

	reg := NewRegistry(newMockClient())
	err := reg.RegisterFromOpenAPI(spec, &ImportOptions{ContinueOnError: true}, func(operationID string) Handler {
		if operationID == "gap.present" {
			return func(ctx context.Context, input []byte) ([]byte, error) { return input, nil }
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterFromOpenAPI: %v", err)
	}
	if _, ok := reg.metadata["gap.present"]; !ok {
		t.Fatal("gap.present should be registered")
	}
	if _, ok := reg.metadata["gap.absent"]; ok {
		t.Fatal("gap.absent should be skipped via ContinueOnError")
	}

	// Without ContinueOnError a missing handler must abort the import.
	reg2 := NewRegistry(newMockClient())
	if err := reg2.RegisterFromOpenAPI(spec, nil, func(string) Handler { return nil }); err == nil {
		t.Fatal("expected error when handler missing and ContinueOnError disabled")
	}
}

// TestOpenAPIToMetadataSkipsNilPathItem covers the nil path item guard in
// openAPIToMetadata: nil entries must be ignored without error.
func TestOpenAPIToMetadataSkipsNilPathItem(t *testing.T) {
	reg := NewRegistry(newMockClient())
	doc := &openapi3.T{Paths: openapi3.NewPaths()}
	doc.Paths.Set("/nil-item", nil)
	doc.Paths.Set("/alive", &openapi3.PathItem{
		Post: &openapi3.Operation{OperationID: "gap.alive", Summary: "Alive"},
	})

	metadatas, err := reg.openAPIToMetadata(doc, nil)
	if err != nil {
		t.Fatalf("openAPIToMetadata: %v", err)
	}
	if len(metadatas) != 1 {
		t.Fatalf("expected 1 metadata, got %d", len(metadatas))
	}
	if metadatas[0].ID != "gap.alive" {
		t.Fatalf("unexpected metadata ID: %s", metadatas[0].ID)
	}
}
