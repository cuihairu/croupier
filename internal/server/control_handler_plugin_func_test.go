package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

// Plugin-adapted functions (todo.md:35) enter through the same register
// snapshot as SDK functions, so the registration boundary must accept the
// `plugin.<name>.<fn>` namespace and carry their contract fields through
// into the session registry.
func TestValidateAndNormalizeFunctions_PluginNamespace(t *testing.T) {
	items := []*agentv1.FunctionDescriptor{
		{
			Id:           "plugin.httpget.fetch",
			Version:      "0.1.0",
			Enabled:      true,
			Resource:     "plugin.httpget",
			Risk:         "warning",
			Execution:    "sync",
			InputSchema:  `{"type":"object","properties":{"url":{"type":"string"}}}`,
			OutputSchema: `{"type":"object","properties":{"body":{"type":"string"}}}`,
		},
		{
			Id:                "plugin.httpget.banplayer",
			Version:           "0.1.0",
			Enabled:           true,
			Resource:          "plugin.httpget",
			Risk:              "danger",
			Execution:         "sync",
			ApprovalRequired:  true,
			ApprovalPolicyKey: "two_person",
			Permission:        "player.ban.invoke",
			InputSchema:       `{"type":"object","properties":{"playerId":{"type":"string"}}}`,
			OutputSchema:      `{"type":"object","additionalProperties":true}`,
		},
	}

	functions, warnings := validateAndNormalizeFunctions(items)

	require.Empty(t, warnings)
	require.Len(t, functions, 2)

	// Normalized output order follows the dedupe map; select by id.
	var danger *agentv1.FunctionDescriptor
	for _, f := range functions {
		if f.Id == "plugin.httpget.banplayer" {
			danger = f
		}
	}
	require.NotNil(t, danger)
	require.Equal(t, "danger", danger.Risk)
	require.True(t, danger.ApprovalRequired)
	require.Equal(t, "two_person", danger.ApprovalPolicyKey)
	require.Equal(t, "player.ban.invoke", danger.Permission)
	require.Equal(t, "sync", danger.Execution)
}

// A plugin declaration carrying a dashboard presentation extension in its
// schema is refused at the same boundary as any other function — plugin
// adapters must not smuggle UI hints into the executable contract.
func TestValidateAndNormalizeFunctions_PluginPresentationFieldRefused(t *testing.T) {
	items := []*agentv1.FunctionDescriptor{
		{
			Id:           "plugin.httpget.fetch",
			Version:      "0.1.0",
			InputSchema:  `{"type":"object","x-menu":{"label":"fetch"}}`,
			OutputSchema: `{"type":"object"}`,
		},
	}

	functions, warnings := validateAndNormalizeFunctions(items)

	require.Empty(t, functions)
	require.Len(t, warnings, 1)
	require.Equal(t, "function_presentation_field_not_allowed", warnings[0].Code)
}
