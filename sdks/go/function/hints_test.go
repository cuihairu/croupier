package function

import (
	"encoding/json"
	"strings"
	"testing"
)

// F14：SetFieldHint/SetFieldWidget 向 InputSchema 注入 x-ui 呈现 hints。
func hintsOf(t *testing.T, schema string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &out); err != nil {
		t.Fatalf("schema not valid JSON: %v", err)
	}
	return out
}

func TestMetadataBuilder_SetFieldHint(t *testing.T) {
	t.Run("merge into existing schema", func(t *testing.T) {
		metadata, err := NewMetadataBuilder().
			SetID("player.ban").
			SetInputSchema(`{"type":"object","properties":{"id":{"type":"string","title":"玩家 ID"}}}`).
			SetFieldHint("id", "x-widget", "Select").
			SetFieldHint("id", "x-options-source", map[string]interface{}{
				"functionId": "player.list",
				"labelPath":  "/items/*/name",
				"valuePath":  "/items/*/id",
			}).
			Build()
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		schema := hintsOf(t, metadata.InputSchema)
		props := schema["properties"].(map[string]interface{})
		id := props["id"].(map[string]interface{})
		if id["x-widget"] != "Select" {
			t.Fatalf("x-widget = %v", id["x-widget"])
		}
		source := id["x-options-source"].(map[string]interface{})
		if source["functionId"] != "player.list" {
			t.Fatalf("x-options-source = %v", source)
		}
		// 既有 title 保留
		if id["title"] != "玩家 ID" {
			t.Fatalf("title lost: %v", id["title"])
		}
	})

	t.Run("empty schema creates object skeleton", func(t *testing.T) {
		metadata, err := NewMetadataBuilder().
			SetID("f").
			SetFieldWidget("level", "Slider").
			Build()
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		schema := hintsOf(t, metadata.InputSchema)
		if schema["type"] != "object" {
			t.Fatalf("type = %v", schema["type"])
		}
		props := schema["properties"].(map[string]interface{})
		level := props["level"].(map[string]interface{})
		if level["x-widget"] != "Slider" {
			t.Fatalf("x-widget = %v", level["x-widget"])
		}
	})

	t.Run("override previous hint", func(t *testing.T) {
		metadata, err := NewMetadataBuilder().
			SetID("f").
			SetFieldWidget("a", "Input").
			SetFieldWidget("a", "TextArea").
			Build()
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		schema := hintsOf(t, metadata.InputSchema)
		a := schema["properties"].(map[string]interface{})["a"].(map[string]interface{})
		if a["x-widget"] != "TextArea" {
			t.Fatalf("x-widget = %v (expected override)", a["x-widget"])
		}
	})

	t.Run("x_ prefix normalized to x-", func(t *testing.T) {
		metadata, err := NewMetadataBuilder().
			SetID("f").
			SetFieldHint("a", "x_widget", "Input").
			Build()
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		schema := hintsOf(t, metadata.InputSchema)
		a := schema["properties"].(map[string]interface{})["a"].(map[string]interface{})
		if _, ok := a["x-widget"]; !ok {
			t.Fatalf("expected normalized x-widget key, got %v", a)
		}
	})

	t.Run("invalid hint rejected", func(t *testing.T) {
		_, err := NewMetadataBuilder().
			SetID("f").
			SetFieldHint("a", "widget", "Input").
			Build()
		if err == nil || !strings.Contains(err.Error(), "x- extension key") {
			t.Fatalf("expected hint validation error, got %v", err)
		}
	})

	t.Run("empty field rejected", func(t *testing.T) {
		_, err := NewMetadataBuilder().
			SetID("f").
			SetFieldHint("", "x-widget", "Input").
			Build()
		if err == nil || !strings.Contains(err.Error(), "field key is required") {
			t.Fatalf("expected field validation error, got %v", err)
		}
	})

	t.Run("empty widget rejected", func(t *testing.T) {
		_, err := NewMetadataBuilder().
			SetID("f").
			SetFieldWidget("a", "  ").
			Build()
		if err == nil || !strings.Contains(err.Error(), "widget is required") {
			t.Fatalf("expected widget validation error, got %v", err)
		}
	})

	t.Run("invalid existing schema rejected", func(t *testing.T) {
		_, err := NewMetadataBuilder().
			SetID("f").
			SetInputSchema(`not-json`).
			SetFieldHint("a", "x-widget", "Input").
			Build()
		if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
			t.Fatalf("expected schema parse error, got %v", err)
		}
	})
}
