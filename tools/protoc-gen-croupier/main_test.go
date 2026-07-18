package main

import (
	"testing"

	componentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/component/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

func TestParseFunctionOptions_Extension(t *testing.T) {
	mo := &descriptorpb.MethodOptions{}
	proto.SetExtension(mo, componentv1.E_Function, &componentv1.FunctionOptions{
		FunctionId:     "player.ban",
		Version:        "1.2.0",
		Category:       "player",
		Risk:           "high",
		Route:          "lb",
		Timeout:        "30s",
		TwoPersonRule:  true,
		Placement:      "agent",
		Mode:           "command",
		IdempotencyKey: true,
		Labels:         map[string]string{"team": "gm"},
		// UI fields (display_name, summary, tags, menu, permissions) are
		// deprecated and no longer extracted by the plugin.
	})

	out := parseFunctionOptions(mo)
	require.Equal(t, "player.ban", out.FunctionID)
	require.Equal(t, "1.2.0", out.Version)
	require.Equal(t, "player", out.Category)
	require.Equal(t, "high", out.Risk)
	require.Equal(t, "lb", out.Route)
	require.Equal(t, "30s", out.Timeout)
	require.True(t, out.TwoPersonRuleSet)
	require.True(t, out.TwoPersonRule)
	require.Equal(t, "agent", out.Placement)
	require.Equal(t, "command", out.Mode)
	require.True(t, out.IdempotencyKeySet)
	require.True(t, out.IdempotencyKey)
	require.Equal(t, map[string]string{"team": "gm"}, out.Labels)
}

func TestFieldToJSONSchema_RepeatedString(t *testing.T) {
	f := &descriptorpb.FieldDescriptorProto{
		Name:  proto.String("tags"),
		Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
		Type:  descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
	}
	sch, _ := fieldToJSONSchema("p", map[string]*descriptorpb.DescriptorProto{}, map[string]*descriptorpb.EnumDescriptorProto{}, f)
	require.Equal(t, "array", sch["type"])
	items := sch["items"].(map[string]any)
	require.Equal(t, "string", items["type"])
}
