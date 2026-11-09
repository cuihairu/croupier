package config

// config.go 补测：ProvisionConfig / TaskLogConfig / ClusterConfig 的
// Effective* 访问器两臂与 ValidateInterconnectTLS 的缺文件 + 跳过校验
// 累加臂（此前这些访问器无任何测试引用，仅默认侧被间接覆盖）。
//
// 诚实跳过：无——全部为纯构造/纯条件分支，可直测。

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEffectiveBinaryPathBothArms(t *testing.T) {
	require.Equal(t, "tofu", (ProvisionConfig{}).EffectiveBinaryPath())
	require.Equal(t, "/opt/custom/opentofu",
		(ProvisionConfig{BinaryPath: "/opt/custom/opentofu"}).EffectiveBinaryPath())
}

func TestEffectiveWorkDirBothArms(t *testing.T) {
	require.Equal(t, "data/provision", (ProvisionConfig{}).EffectiveWorkDir())
	require.Equal(t, "/srv/provision", (ProvisionConfig{WorkDir: "/srv/provision"}).EffectiveWorkDir())
}

func TestEffectiveBootstrapTokenEnvBothArms(t *testing.T) {
	require.Equal(t, "CROUPIER_PROVISION_TOKEN", (ProvisionConfig{}).EffectiveBootstrapTokenEnv())
	require.Equal(t, "MY_TOKEN_VAR",
		(ProvisionConfig{BootstrapTokenEnv: "MY_TOKEN_VAR"}).EffectiveBootstrapTokenEnv())
}

func TestEffectiveRegisterTimeoutBothArms(t *testing.T) {
	require.Equal(t, 300*time.Second, (ProvisionConfig{}).EffectiveRegisterTimeout())
	require.Equal(t, 90*time.Second,
		(ProvisionConfig{RegisterTimeoutSec: 90}).EffectiveRegisterTimeout())
}

func TestEffectiveReaperTimeoutArms(t *testing.T) {
	require.Equal(t, 5*time.Minute, (TaskLogConfig{}).EffectiveReaperTimeout())
	require.Equal(t, 5*time.Minute,
		(TaskLogConfig{ReaperTimeout: "not-a-duration"}).EffectiveReaperTimeout())
	require.Equal(t, 5*time.Minute,
		(TaskLogConfig{ReaperTimeout: "0s"}).EffectiveReaperTimeout())
	require.Equal(t, 2*time.Minute,
		(TaskLogConfig{ReaperTimeout: "2m"}).EffectiveReaperTimeout())
	// 带空白输入：TrimSpace 归一后解析。
	require.Equal(t, 3*time.Minute,
		(TaskLogConfig{ReaperTimeout: " 3m "}).EffectiveReaperTimeout())
}

// ValidateInterconnectTLS 的缺证书文件 + 跳过校验累加臂：interconnectTls
// 显式 enabled=true（EffectiveEnabled 走真分支）但 cert/key/ca 全空，且
// interconnectTls.insecureSkipVerify=true——诊断须同时列出缺失文件与
// 跳过校验开关（此前用例只覆盖 enabled=false 与证书齐备两翼）。
func TestValidateInterconnectTLS_MissingFilesAndSkipVerifyAccumulate(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`
cluster:
  enabled: true
  interconnectTls:
    enabled: true
    insecureSkipVerify: true
`), &cfg))

	err := cfg.Cluster.ValidateInterconnectTLS()
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "missing certFile/keyFile/caFile")
	require.Contains(t, msg, "interconnectTls.insecureSkipVerify=true")
	require.NotContains(t, msg, "insecureSkipTls=true")
}
