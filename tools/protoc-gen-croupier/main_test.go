package main

import (
	"testing"

	commonv1 "github.com/cuihairu/croupier/generated/croupier/common/v1"
	optionsv1 "github.com/cuihairu/croupier/generated/croupier/options/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

func TestParseFunctionOptions_Extension(t *testing.T) {
	mo := &descriptorpb.MethodOptions{}
	proto.SetExtension(mo, optionsv1.E_Function, &optionsv1.FunctionOptions{
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
		DisplayName:    &commonv1.I18NText{Zh: "封禁玩家", En: "Ban Player"},
		Summary:        &commonv1.I18NText{Zh: "封禁指定玩家", En: "Ban a player"},
		Tags:           []string{"player", "moderation"},
		Menu:           &commonv1.Menu{Section: "Function Management", Group: "Player", Path: "/functions/invoke", Order: 10, Icon: "StopOutlined", Badge: "beta", Hidden: false},
		Permissions:    &commonv1.PermissionSpec{Verbs: []string{"read", "invoke"}, Scopes: []string{"game", "env", "function_id"}, Defaults: []*commonv1.RoleBinding{{Role: "operator", Verbs: []string{"invoke"}}}},
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
	require.Equal(t, map[string]string{"zh": "封禁玩家", "en": "Ban Player"}, out.DisplayName)
	require.Equal(t, map[string]string{"zh": "封禁指定玩家", "en": "Ban a player"}, out.Summary)
	require.Equal(t, []string{"player", "moderation"}, out.Tags)
	require.Equal(t, "Function Management", out.Menu["section"])
	require.Equal(t, []string{"read", "invoke"}, out.Permissions["verbs"])
}

func TestCollectUIFieldHints_Extension(t *testing.T) {
	fo := &descriptorpb.FieldOptions{}
	proto.SetExtension(fo, optionsv1.E_Ui, &optionsv1.UIFieldOptions{
		Widget:      "input",
		Label:       "玩家ID",
		Placeholder: "请输入玩家ID",
		Sensitive:   true,
		EnumMap:     map[string]string{"A": "Alpha", "B": "Beta"},
		ShowIf:      "x == 1",
		RequiredIf:  "y == 2",
	})
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("Req"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("player_id"),
				JsonName: proto.String("playerId"),
				Options:  fo,
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			},
		},
	}

	hints := collectUIFieldHints(msg)
	require.Contains(t, hints.Fields, "playerId")
	cfg := hints.Fields["playerId"]
	require.Equal(t, "input", cfg["widget"])
	require.Equal(t, "玩家ID", cfg["label"])
	require.Equal(t, "请输入玩家ID", cfg["placeholder"])
	require.Equal(t, true, cfg["sensitive"])
	require.Equal(t, "x == 1", cfg["show_if"])
	require.Equal(t, "y == 2", cfg["required_if"])
	require.ElementsMatch(t, []string{"A", "B"}, cfg["enum"].([]string))
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
