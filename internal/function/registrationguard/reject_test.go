package registrationguard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForbiddenPresentationField(t *testing.T) {
	t.Run("rejects presentation keys in all spellings", func(t *testing.T) {
		for _, key := range []string{
			"ui", "x-ui", "menu", "x-menu", "layout", "x-layout",
			"table", "x-table", "tableColumns", "x-table-columns",
			"pagination", "x-pagination", "route", "routes", "x-route",
			"placement", "x-placement", "formily", "x-components",
			"inputMapping", "x-input-mapping", "output_mapping",
			"page-contract", "x-page-schema", "x-page",
			"displayName", "entityDisplay", "categoryDisplay", "operationKind",
			"x-title", "x-columns",
		} {
			field, ok := ForbiddenPresentationField(key)
			assert.True(t, ok, "expected %q to be forbidden", key)
			assert.NotEmpty(t, field)
		}
	})

	t.Run("accepts capability contract keys", func(t *testing.T) {
		for _, key := range []string{
			"x-resource", "x-operation", "x-capability", "x-execution",
			"x-risk", "x-permission", "x-approval", "resource", "operation",
			"capability", "execution", "risk", "permission", "version",
			"summary", "description", "tags",
		} {
			_, ok := ForbiddenPresentationField(key)
			assert.False(t, ok, "expected %q to be allowed", key)
		}
	})
}

func TestFindPresentationViolation(t *testing.T) {
	violation, found := FindPresentationViolation(
		map[string]string{"x-title": "Players"},
		`{"type":"object","x-menu":"Players"}`,
		``,
	)
	assert.True(t, found)
	assert.Equal(t, "x-title", violation.Field)
	assert.Equal(t, "extensions.x-title", violation.Location)

	violation, found = FindPresentationViolation(nil, `{"type":"object","x-menu":"Players"}`, ``)
	assert.True(t, found)
	assert.Equal(t, "x-menu", violation.Field)
	assert.Equal(t, "inputSchema.x-menu", violation.Location)
}

func TestForbiddenRegistrationExtensionField(t *testing.T) {
	for _, key := range []string{"title", "labels", "columns"} {
		field, forbidden := ForbiddenRegistrationExtensionField(key)
		assert.True(t, forbidden, "expected %q to be forbidden", key)
		assert.Equal(t, key, field)
	}

	_, forbidden := ForbiddenPresentationField("title")
	assert.False(t, forbidden, "standard OpenAPI title must remain valid")
}

func TestScanJSON(t *testing.T) {
	t.Run("empty and non-JSON input reports nothing", func(t *testing.T) {
		for _, raw := range []string{"", "   ", "not-json", "{broken"} {
			field, path, found := ScanJSON(raw)
			assert.False(t, found, "input %q", raw)
			assert.Empty(t, field)
			assert.Empty(t, path)
		}
	})

	t.Run("clean capability schema passes", func(t *testing.T) {
		raw := `{
			"type": "object",
			"properties": {
				"player_id": {"type": "string"},
				"reason": {"type": "string", "title": "Reason"},
				"menu": {"type": "string"},
				"table": {"type": "integer"}
			},
			"required": ["player_id"]
		}`
		// "menu"/"table" here are payload property names, not presentation
		// extensions — schema scanning only enforces extension-style keys.
		_, _, found := ScanJSON(raw)
		assert.False(t, found)
	})

	t.Run("rejects dashboard extension at schema root", func(t *testing.T) {
		raw := `{"type": "object", "x-menu": "Players"}`
		field, path, found := ScanJSON(raw)
		assert.True(t, found)
		assert.Equal(t, "x-menu", field)
		assert.Equal(t, "$.x-menu", path)
	})

	t.Run("rejects nested dashboard extension with full path", func(t *testing.T) {
		raw := `{
			"type": "object",
			"properties": {
				"player_id": {"type": "string", "x-table-columns": ["name"]}
			}
		}`
		field, path, found := ScanJSON(raw)
		assert.True(t, found)
		assert.Equal(t, "x-table-columns", field)
		assert.Equal(t, "$.properties.player_id.x-table-columns", path)
	})

	t.Run("rejects component tree and page path extensions", func(t *testing.T) {
		cases := map[string]string{
			`{"x-components": {"Form": {}}}`:     "x-components",
			`{"x-route": "/console/players"}`:    "x-route",
			`{"x-pagination": {"pageSize": 20}}`: "x-pagination",
			`{"formily": {"schema": {}}}`:        "formily",
			`{"x-input-mapping": {"a": "b"}}`:    "x-input-mapping",
		}
		for raw, want := range cases {
			field, _, found := ScanJSON(raw)
			assert.True(t, found, "input %s", raw)
			assert.Equal(t, want, field)
		}
	})

	t.Run("rejects extension inside arrays", func(t *testing.T) {
		raw := `{"anyOf": [{"type": "string"}, {"type": "object", "x-placement": "sidebar"}]}`
		field, path, found := ScanJSON(raw)
		assert.True(t, found)
		assert.Equal(t, "x-placement", field)
		assert.Equal(t, "$.anyOf[1].x-placement", path)
	})

	t.Run("ignores unknown x- extensions", func(t *testing.T) {
		raw := `{"type": "object", "x-custom-vendor-flag": true}`
		_, _, found := ScanJSON(raw)
		assert.False(t, found)
	})
}

func TestScanJSONValueDeterministic(t *testing.T) {
	value := map[string]interface{}{
		"x-route": "/a",
		"x-menu":  "Players",
	}
	// Keys are visited in sorted order: x-menu sorts before x-route.
	field, _, found := ScanJSONValue(value, "$")
	assert.True(t, found)
	assert.Equal(t, "x-menu", field)
}
