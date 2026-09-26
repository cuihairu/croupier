// buildProviders 实例元数据回归：Instance.Metadata 中的用户多 KV（serverId 等）
// 必须经 UserMetadata 剥离保留键后随 AgentProcess.Metadata 上报。
package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/platform/agentlocal"
)

func TestBuildProviders_CarriesUserInstanceMetadata(t *testing.T) {
	now := time.Now()
	localData := map[string][]agentlocal.Instance{
		"fn.echo": {
			{
				ProviderID: "svc-meta",
				LastSeen:   now,
				Metadata: map[string]string{
					"sdkLanguage": "go",
					"sdkVersion":  "1.4.0",
					"sdkName":     "croupier-go-sdk",
					"gameId":      "default",
					"env":         "dev",
					"serverId":    "s1",
					"pod":         "game-7c4d",
				},
			},
		},
	}

	procs := buildProviders(localData, nil)
	require.Len(t, procs, 1)
	p := procs[0]
	assert.Equal(t, "svc-meta", p.ServiceId)
	assert.Equal(t, "go", p.SdkLanguage)
	assert.Equal(t, "1.4.0", p.SdkVersion)
	assert.Equal(t, "default", p.GameId)
	assert.Equal(t, "dev", p.Env)

	// 用户元数据保留，平台固定键不混入
	assert.Equal(t, map[string]string{"serverId": "s1", "pod": "game-7c4d"}, p.Metadata)
}

func TestBuildProviders_NoUserMetadataLeavesFieldNil(t *testing.T) {
	now := time.Now()
	localData := map[string][]agentlocal.Instance{
		"fn.echo": {
			{
				ProviderID: "svc-plain",
				LastSeen:   now,
				Metadata: map[string]string{
					"sdkLanguage": "js",
					"sdkVersion":  "1.5.0",
					"gameId":      "default",
					"env":         "dev",
				},
			},
		},
	}

	procs := buildProviders(localData, nil)
	require.Len(t, procs, 1)
	assert.Nil(t, procs[0].Metadata, "无用户元数据时 Metadata 应为 nil（wire 不发空 map）")
	assert.Equal(t, "js", procs[0].SdkLanguage)
}
