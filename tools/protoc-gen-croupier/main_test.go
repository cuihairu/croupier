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
		Resource:       "player",
		Operation:      "ban",
		Risk:           "high",
		Route:          "lb",
		Timeout:        "30s",
		TwoPersonRule:  true,
		Mode:           "command",
		IdempotencyKey: true,
		Summary:        "Ban Player",
		Description:    "Ban a player",
		Tags:           []string{"player", "moderation"},
		Permission:     "player.ban.invoke",
	})

	out := parseFunctionOptions(mo)
	require.Equal(t, "player.ban", out.FunctionID)
	require.Equal(t, "1.2.0", out.Version)
	require.Equal(t, "player", out.Resource)
	require.Equal(t, "ban", out.Operation)
	require.Equal(t, "high", out.Risk)
	require.Equal(t, "lb", out.Route)
	require.Equal(t, "30s", out.Timeout)
	require.True(t, out.TwoPersonRuleSet)
	require.True(t, out.TwoPersonRule)
	require.Equal(t, "command", out.Mode)
	require.True(t, out.IdempotencyKeySet)
	require.True(t, out.IdempotencyKey)
	require.Equal(t, "Ban Player", out.Summary)
	require.Equal(t, "Ban a player", out.Description)
	require.Equal(t, []string{"player", "moderation"}, out.Tags)
	require.Equal(t, "player.ban.invoke", out.Permission)
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
