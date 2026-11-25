package extension

// Coverage sweep round 28 (wt-api): the non-JSON-object manifest arm of
// PackImport (the Unmarshal failure/nil half at service.go:508-509) -- when
// the inner "manifest" value is an array or null:
// extensionPackManifest.Manifest is a json.RawMessage, so it passes through
// untouched during unpacking, and the object validation before landing in the
// database intercepts it. The other arm, the json.Marshal(manifestObj)
// failure at service.go:513, is registered by catalog_writes_paths_test.go
// (a decoded product has no failure path when re-serialized).

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackImport_ManifestNotObject(t *testing.T) {
	env := newPackEnv(t)

	cases := []struct {
		name     string
		manifest string
	}{
		{"array", `"manifest": [1,2]`},
		{"null", `"manifest": null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packJSON := fmt.Sprintf(`{
  "extensionId": "com.example.demo",
  "name": "demo",
  "displayName": "Demo Extension",
  "vendor": "example",
  "kind": "integration",
  "summary": "demo pack",
  "version": "1.0.0",
  "releaseChannel": "stable",
  "minCoreVersion": "0.0.1",
  "changelog": "first release",
  %s
}`, tc.manifest)
			pack := buildPack(t, map[string]string{"./manifest.json": packJSON})

			resp, err := env.service.PackImport(env.ctx, bytes.NewReader(pack), int64(len(pack)), "ext_tester")
			require.Error(t, err)
			assert.Nil(t, resp)
			assert.Contains(t, err.Error(), "manifest 必须为 JSON 对象")
		})
	}
}
