package provision

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderCloudInit_ContainsTokenAndHub(t *testing.T) {
	out, err := renderCloudInit(CloudInitOptions{
		HubAddr:         "hub.example.com:19090",
		BootstrapToken:  "secret-token-abc",
		AgentInstallURL: "https://dl.example.com/agent.sh",
		Extra:           "[ systemctl, enable, --now, croupier-agent ]",
	})
	require.NoError(t, err)
	assert.Contains(t, out, "#cloud-config")
	assert.Contains(t, out, "hubAddr: hub.example.com:19090")
	assert.Contains(t, out, "bootstrapToken: secret-token-abc")
	assert.Contains(t, out, "https://dl.example.com/agent.sh")
	assert.Contains(t, out, "croupier-agent")
}

func TestRenderCloudInit_NoInstallURLSkipsDownload(t *testing.T) {
	out, err := renderCloudInit(CloudInitOptions{HubAddr: "h:1", BootstrapToken: "t"})
	require.NoError(t, err)
	assert.NotContains(t, out, "curl")
	assert.Contains(t, out, "bootstrapToken: t")
}

// C-002：CloudInitExtra 注入面校验。元字符（$()/;/|/&/重定向/子 shell）与
// 「解释器 + eval 标志」注入逐行拒绝；合法单条命令与安全列表形式放行。
func TestValidateCloudInitExtra_RejectsInjection(t *testing.T) {
	cases := []string{
		"$(rm -rf /)",             // 命令替换
		"`rm -rf /`",              // 反引号替换
		"curl x | bash",           // 管道
		"curl x; rm -rf /",        // 命令分隔
		"curl x && rm -rf /",      // 串联
		"curl x > /etc/passwd",    // 重定向
		"(rm -rf /)",              // 子 shell
		"[ sh, -c, 'rm -rf /' ]",  // 列表形式解释器求值
		"sh -c 'rm -rf /'",        // 字符串形式解释器求值
		"bash -e 'curl x | bash'", // 解释器 eval
		"- rm -rf /",              // YAML 嵌套
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			err := validateCloudInitExtra(c)
			require.Error(t, err, "should reject: %s", c)
		})
	}
	// 逐行校验：多行中一行非法即拒。
	err := validateCloudInitExtra("curl x -o /tmp/y\n; rm -rf /\n")
	require.Error(t, err)
}

func TestValidateCloudInitExtra_AllowsSafe(t *testing.T) {
	cases := []string{
		"[ systemctl, enable, --now, croupier-agent ]",
		"curl -fsSL https://dl.example.com/agent.sh -o /tmp/a.sh",
		"bash /tmp/croupier-agent-install.sh",
		"systemctl enable --now croupier-agent",
		"mkdir -p /etc/croupier",
		"",       // 空行跳过
		"  \n  ", // 空白行跳过
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			require.NoError(t, validateCloudInitExtra(c), "should allow: %q", c)
		})
	}
}

func TestRenderCloudInit_RejectsInjectionExtra(t *testing.T) {
	_, err := renderCloudInit(CloudInitOptions{
		HubAddr:        "h:1",
		BootstrapToken: "t",
		Extra:          "curl x; rm -rf /",
	})
	require.Error(t, err)
}

// 各 provider 模板必须：含 provider 块、含对应 resource、含 user_data 注入、
// 含 instance_id 输出。这是模板渲染正确性的真值面（无云凭据不可 apply，
// 真实 apply 验证登记为部署后人工项）。
func TestRenderTemplate_PerProviderResources(t *testing.T) {
	cases := []struct {
		p        Provider
		resource string
	}{
		{ProviderAliyun, "alicloud_instance"},
		{ProviderTencent, "tencentcloud_instance"},
		{ProviderAWS, "aws_instance"},
		{ProviderHetzner, "hcloud_server"},
	}
	req := Request{
		Region:         "test-region",
		Count:          2,
		Spec:           Spec{InstanceType: "t3.small"},
		GameID:         "demo",
		Env:            "prod",
		HubAddr:        "hub:19090",
		BootstrapToken: "tok",
	}
	for _, c := range cases {
		t.Run(string(c.p), func(t *testing.T) {
			main, err := renderTemplate(c.p, req)
			require.NoError(t, err)
			assert.Contains(t, main, "variable \"region\"")
			assert.Contains(t, main, c.resource)
			assert.Contains(t, main, "user_data")
			assert.Contains(t, main, "instance_id")
			assert.Contains(t, main, "var.name_prefix") // 值经 apply vars 注入
		})
	}
}

func TestRenderTemplate_UnknownProvider(t *testing.T) {
	_, err := renderTemplate("gcp", Request{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestRenderTemplate_Base64Aliyun(t *testing.T) {
	main, err := renderTemplate(ProviderAliyun, Request{Region: "cn-hangzhou", Count: 1, Spec: Spec{InstanceType: "ecs.s6"}, GameID: "g", Env: "e", HubAddr: "h", BootstrapToken: "t"})
	require.NoError(t, err)
	assert.Contains(t, main, "base64encode(var.user_data)")
}

func TestValidate_Matrix(t *testing.T) {
	base := func() *Request {
		return &Request{
			Provider: ProviderAWS,
			Region:   "us-east-1",
			Count:    1,
			Spec:     Spec{InstanceType: "t3.micro"},
			GameID:   "demo",
			Env:      "prod",
		}
	}
	t.Run("valid defaults policy direct", func(t *testing.T) {
		req, err := Validate(base())
		require.NoError(t, err)
		assert.Equal(t, PolicyDirect, req.Policy)
		assert.Equal(t, defaultDiskGB, req.Spec.DiskSizeGB)
	})
	t.Run("ticket policy kept", func(t *testing.T) {
		r := base()
		r.Policy = PolicyTicket
		req, err := Validate(r)
		require.NoError(t, err)
		assert.Equal(t, PolicyTicket, req.Policy)
	})
	t.Run("missing provider", func(t *testing.T) {
		r := base()
		r.Provider = ""
		_, err := Validate(r)
		require.Error(t, err)
	})
	t.Run("bad provider", func(t *testing.T) {
		r := base()
		r.Provider = "gcp"
		_, err := Validate(r)
		require.Error(t, err)
	})
	t.Run("missing region", func(t *testing.T) {
		r := base()
		r.Region = ""
		_, err := Validate(r)
		require.Error(t, err)
	})
	t.Run("count zero", func(t *testing.T) {
		r := base()
		r.Count = 0
		_, err := Validate(r)
		require.Error(t, err)
	})
	t.Run("missing instance type", func(t *testing.T) {
		r := base()
		r.Spec.InstanceType = ""
		_, err := Validate(r)
		require.Error(t, err)
	})
	t.Run("missing game", func(t *testing.T) {
		r := base()
		r.GameID = ""
		_, err := Validate(r)
		require.Error(t, err)
	})
	t.Run("bad policy", func(t *testing.T) {
		r := base()
		r.Policy = "chaos"
		_, err := Validate(r)
		require.Error(t, err)
	})
	t.Run("cloudInitExtra injection rejected", func(t *testing.T) {
		r := base()
		r.CloudInitExtra = "curl x; rm -rf /"
		_, err := Validate(r)
		require.Error(t, err)
	})
	t.Run("cloudInitExtra safe allowed", func(t *testing.T) {
		r := base()
		r.CloudInitExtra = "[ systemctl, enable, --now, croupier-agent ]"
		_, err := Validate(r)
		require.NoError(t, err)
	})
}

func TestNewRunID(t *testing.T) {
	a, b := NewRunID(), NewRunID()
	assert.NotEqual(t, a, b)
	assert.True(t, strings.HasPrefix(a, "prov-"))
}
