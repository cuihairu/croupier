package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 覆盖散落的 0% 辅助函数（ValidateBugLinks/MaskedDSN/ValidateDBSource/
// NormalizeChannel/RandomSeedHex/BucketHit/WritableConfigSourceType 等）。

func TestValidateDBSource(t *testing.T) {
	// 合法
	require.NoError(t, ValidateDBSource(&DBSource{Name: "x", Driver: "postgres", Kind: "self", DSN: "postgres://user@host/db", GameID: "demo", Env: "prod"}))
	// 缺字段
	assert.Error(t, ValidateDBSource(&DBSource{}))
	assert.Error(t, ValidateDBSource(&DBSource{Name: "x", Driver: "bogus", Kind: "self", DSN: "user@host", GameID: "g", Env: "e"}))
	assert.Error(t, ValidateDBSource(&DBSource{Name: "x", Driver: "postgres", Kind: "bogus", DSN: "user@host", GameID: "g", Env: "e"}))
}

func TestMaskedDSN(t *testing.T) {
	dsn := &DBSource{DSN: "postgres://user:secret@host:5432/db"}
	masked := dsn.MaskedDSN()
	assert.NotContains(t, masked, "secret", "密码必须被掩码")

	// 无 @ 符号的 DSN → 全掩码
	dsn2 := &DBSource{DSN: "host=/var/run/postgresql db=app"}
	assert.NotEmpty(t, dsn2.MaskedDSN())
}

func TestNormalizeChannel(t *testing.T) {
	assert.Equal(t, "official", NormalizeChannel("Official"))
	assert.Equal(t, "official", NormalizeChannel(" OFFICIAL "))
	assert.Equal(t, "official", NormalizeChannel(" Official "))
}

func TestRandomSeedHex(t *testing.T) {
	s1 := RandomSeedHex()
	s2 := RandomSeedHex()
	assert.NotEmpty(t, s1)
	assert.NotEqual(t, s1, s2, "每次随机")
}

func TestBucketHit(t *testing.T) {
	rel := &GameRelease{GraySeed: "seed", GrayPercent: 100}
	rel0 := &GameRelease{GraySeed: "seed", GrayPercent: 0}
	// 100% 必中
	assert.True(t, rel.BucketHit("any-device"))
	// 0% 必不中
	assert.False(t, rel0.BucketHit("any-device"))
	// 同 seed+device 确定性
	rel50 := &GameRelease{GraySeed: "seed", GrayPercent: 50}
	r1 := rel50.BucketHit("dev-1")
	r2 := rel50.BucketHit("dev-1")
	assert.Equal(t, r1, r2, "同 seed+device 确定性")
}

func TestWritableConfigSourceType(t *testing.T) {
	assert.True(t, WritableConfigSourceType(ConfigSourceTypeRedis))
	assert.True(t, WritableConfigSourceType(ConfigSourceTypeNacos))
	assert.False(t, WritableConfigSourceType(ConfigSourceTypeGit))
	assert.False(t, WritableConfigSourceType(ConfigSourceTypeDB))
}

func TestValidateBugLinks(t *testing.T) {
	require.NoError(t, ValidateBugLinks([]BugLink{{URL: "https://x", Kind: "github_issue"}}))
	assert.Error(t, ValidateBugLinks([]BugLink{{URL: "", Kind: "github_issue"}}))
	assert.Error(t, ValidateBugLinks([]BugLink{{URL: "https://x", Kind: "bogus"}}))
	require.NoError(t, ValidateBugLinks(nil))
}

func TestNormalizeConfigNamespace(t *testing.T) {
	ns, ok := NormalizeConfigNamespace("")
	assert.True(t, ok)
	assert.Equal(t, "runtime", ns) // default = runtime
	ns, ok = NormalizeConfigNamespace("gameplay")
	assert.True(t, ok)
	assert.Equal(t, "gameplay", ns)
	ns, ok = NormalizeConfigNamespace("bogus")
	assert.False(t, ok)
	_ = ns
}
