package extension

// 覆盖率巡检第二十八轮（wt-api）：PackImport 的 manifest 非 JSON 对象翼
// （service.go:508-509 的 Unmarshal 失败/nil 半边）——内层 "manifest" 值
// 为数组或 null 时：extensionPackManifest.Manifest 是 json.RawMessage，
// 解包期原样透传，落库前的对象校验在此拦截。另一边 service.go:513 的
// json.Marshal(manifestObj) 失败翼由 catalog_writes_paths_test.go 登记
// （解码产物再序列化无失败路径）。

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
