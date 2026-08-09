package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInferResourceAndCapability(t *testing.T) {
	tests := []struct {
		name             string
		functionID       string
		input            FunctionMeta
		expectedResource string
		expectedOp       string
		expectedCap      string
	}{
		{
			name:             "list infers collection_query",
			functionID:       "player.list",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "player",
			expectedOp:       "list",
			expectedCap:      "collection_query",
		},
		{
			name:             "get infers item_query",
			functionID:       "player.get",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "player",
			expectedOp:       "get",
			expectedCap:      "item_query",
		},
		{
			name:             "create infers create",
			functionID:       "order.create",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "order",
			expectedOp:       "create",
			expectedCap:      "create",
		},
		{
			name:             "update infers update",
			functionID:       "player.update",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "player",
			expectedOp:       "update",
			expectedCap:      "update",
		},
		{
			name:             "delete infers delete",
			functionID:       "player.delete",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "player",
			expectedOp:       "delete",
			expectedCap:      "delete",
		},
		{
			name:             "ban infers action",
			functionID:       "player.ban",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "player",
			expectedOp:       "ban",
			expectedCap:      "action",
		},
		{
			name:             "search infers collection_query",
			functionID:       "player.search",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "player",
			expectedOp:       "search",
			expectedCap:      "collection_query",
		},
		{
			name:             "detail infers item_query",
			functionID:       "order.detail",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "order",
			expectedOp:       "detail",
			expectedCap:      "item_query",
		},
		{
			name:             "add infers create",
			functionID:       "item.add",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "item",
			expectedOp:       "add",
			expectedCap:      "create",
		},
		{
			name:             "remove infers delete",
			functionID:       "item.remove",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "item",
			expectedOp:       "remove",
			expectedCap:      "delete",
		},
		{
			name:             "no dot does not infer",
			functionID:       "standalone_func",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "",
			expectedOp:       "",
			expectedCap:      "",
		},
		{
			name:             "explicit capability not overwritten",
			functionID:       "player.list",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0", Capability: "action"},
			expectedResource: "",
			expectedOp:       "",
			expectedCap:      "action",
		},
		{
			name:             "explicit resource not overwritten",
			functionID:       "player.list",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0", Resource: "custom"},
			expectedResource: "custom",
			expectedOp:       "list",
			expectedCap:      "collection_query",
		},
		{
			name:             "nested resource with dot",
			functionID:       "game.player.list",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "game.player",
			expectedOp:       "list",
			expectedCap:      "collection_query",
		},
		{
			name:             "unknown operation infers action",
			functionID:       "player.send",
			input:            FunctionMeta{Enabled: true, Version: "1.0.0"},
			expectedResource: "player",
			expectedOp:       "send",
			expectedCap:      "action",
		},
		{
			name:       "nil meta does not panic",
			functionID: "player.list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := tt.input
			InferResourceAndCapability(tt.functionID, &meta)
			if tt.expectedResource != "" || tt.expectedOp != "" || tt.expectedCap != "" {
				assert.Equal(t, tt.expectedResource, meta.Resource, "resource")
				assert.Equal(t, tt.expectedOp, meta.Operation, "operation")
				assert.Equal(t, tt.expectedCap, meta.Capability, "capability")
			}
		})
	}
}
