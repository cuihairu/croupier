package funcplugin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin/declare"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

// Descriptor renders the validated declaration as an agent registration
// payload descriptor (todo.md:35 remaining wiring: plugin functions join
// the register snapshot). These tests lock the field mapping and the
// risk vocabulary normalization at the conversion boundary.
func TestRegisteredDescriptor_FromMetadata(t *testing.T) {
	md, err := declare.Metadata("httpget", "0.1.0", declare.Descriptor{
		ID:           "plugin.httpget.fetch",
		Summary:      "fetch text",
		ParamSchema:  `{"type":"object"}`,
		ResultSchema: `{"type":"object"}`,
		RiskLevel:    declare.RiskLow,
	})
	require.NoError(t, err)

	d := Registered{Plugin: "httpget", Version: "0.1.0", Metadata: md}.Descriptor()
	require.Equal(t, "plugin.httpget.fetch", d.Id)
	require.Equal(t, "0.1.0", d.Version)
	require.True(t, d.Enabled)
	require.Equal(t, "plugin.httpget", d.Resource)
	require.Equal(t, []string{"plugin", "httpget"}, d.Tags)
	require.Equal(t, "fetch text", d.Summary)
	require.Equal(t, `{"type":"object"}`, d.InputSchema)
	require.Equal(t, `{"type":"object"}`, d.OutputSchema)
	require.Equal(t, "sync", d.Execution)
	require.Equal(t, "safe", d.Risk)
}

func TestRegisteredDescriptor_RiskWireVocabulary(t *testing.T) {
	cases := map[string]string{
		declare.RiskLow:    "safe",
		declare.RiskMedium: "warning",
		declare.RiskHigh:   "high",
		declare.RiskDanger: "danger",
	}
	for declared, want := range cases {
		md, err := declare.Metadata("httpget", "0.1.0", declare.Descriptor{
			ID:           "plugin.httpget.fetch",
			Summary:      "s",
			ParamSchema:  `{"type":"object"}`,
			ResultSchema: `{"type":"object"}`,
			RiskLevel:    declared,
		})
		require.NoError(t, err)
		d := Registered{Plugin: "httpget", Version: "0.1.0", Metadata: md}.Descriptor()
		require.Equal(t, want, d.Risk, "declared risk %q", declared)
	}
}

func TestRegisteredDescriptor_NilMetadataFallsBackToDeclared(t *testing.T) {
	d := Registered{
		Plugin:  "httpget",
		Version: "1.2.3",
		Declared: declare.Descriptor{
			ID:           "plugin.httpget.fetch",
			Summary:      "fetch text",
			ParamSchema:  `{"type":"object"}`,
			ResultSchema: `{"type":"object"}`,
			RiskLevel:    declare.RiskHigh,
		},
	}.Descriptor()
	require.NotNil(t, d)
	require.Equal(t, "plugin.httpget.fetch", d.Id)
	require.Equal(t, "1.2.3", d.Version)
	require.True(t, d.Enabled)
	require.Equal(t, "plugin.httpget", d.Resource)
	require.Equal(t, "fetch text", d.Summary)
	require.Equal(t, `{"type":"object"}`, d.InputSchema)
	require.Equal(t, `{"type":"object"}`, d.OutputSchema)
	require.Equal(t, "high", d.Risk)
}

func TestRegisteredDescriptor_MetadataSecurityPassthrough(t *testing.T) {
	// functionv1 security fields the declare builder leaves unset stay
	// unset on the wire: approval/permission remain server-side policy,
	// the descriptor adds no bypass.
	md := &functionv1.FunctionMetadata{
		Id:        "plugin.httpget.fetch",
		Version:   "0.1.0",
		Resource:  "plugin.httpget",
		Execution: "sync",
		Security:  &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_DANGER},
	}
	d := Registered{Plugin: "httpget", Version: "0.1.0", Metadata: md}.Descriptor()
	require.Equal(t, "danger", d.Risk)
	require.False(t, d.ApprovalRequired)
	require.Empty(t, d.Permission)
}
