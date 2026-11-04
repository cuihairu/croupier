package provision

import (
	"strconv"
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

// #673：AgentInstallURL 校验——空白/控制字符与非法 scheme 拒；shell 元字符
// 本身经渲染边界单引号中和（见 TestRenderCloudInit_SingleQuotesInstallURL）。
func TestValidateAgentInstallURL_RejectsBad(t *testing.T) {
	cases := []string{
		"https://dl.example.com/agent.sh; rm -rf /", // 空白拆词
		"https://dl.example.com/agent.sh  --extra",  // 空白拆词
		"https://dl.example.com/agent.sh # comment", // 空白+注释
		"https://dl.example.com/agent.sh\nrm -rf /", // 换行破坏 YAML
		"file:///etc/passwd",
		"ftp://dl.example.com/agent.sh",
		"javascript:alert(1)",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			err := validateAgentInstallURL(c)
			require.Error(t, err, "should reject: %q", c)
		})
	}
}

func TestValidateAgentInstallURL_AllowsSafe(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"https://dl.example.com/agent.sh",
		"http://10.0.0.5:8080/install",
		"https://dl.example.com/install?token=abc-123&x=1",
		"https://dl.example.com/agent.sh?ver=1.2.3&src=croupier",
		"https://dl.example.com/agent.sh;echo%20pwned", // 元字符将由渲染引号中和
		"https://dl.example.com/$(id).sh",
		"https://dl.example.com/agent.sh`id`",
		"https://dl.example.com/agent.sh|bash",
		"https://dl.example.com/agent.sh'x",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			require.NoError(t, validateAgentInstallURL(c), "should allow: %q", c)
		})
	}
}

func TestRenderCloudInit_RejectsInjectionInstallURL(t *testing.T) {
	_, err := renderCloudInit(CloudInitOptions{
		HubAddr:         "h:1",
		BootstrapToken:  "t",
		AgentInstallURL: "https://dl.example.com/agent.sh rm -rf /", // 空白拒
	})
	require.Error(t, err)
}

// #673：渲染输出把 URL 包为单引号 shell 字面量（内嵌 ' 以 '\” 转义），
// 元字符成为字面量，不改变 sh -c 脚本解析。
func TestRenderCloudInit_SingleQuotesInstallURL(t *testing.T) {
	out, err := renderCloudInit(CloudInitOptions{
		HubAddr:         "h:1",
		BootstrapToken:  "t",
		AgentInstallURL: "https://dl.example.com/agent.sh;echo%20pwned",
	})
	require.NoError(t, err)
	assert.Contains(t, out, "curl -fsSL 'https://dl.example.com/agent.sh;echo%20pwned' -o /tmp/croupier-agent-install.sh")
	assert.NotContains(t, out, "';echo%20pwned' &&")
}

// #658：tfVarAssignment 把字符串变量编码为 HCL 字符串（strconv.Quote），
// 多行 cloud-init YAML/#cloud-config 注释不再被 terraform 当作 HCL 解析；
// 数字变量（count/disk_size）保持裸 number；非数字值报错。
func TestTfVarAssignment(t *testing.T) {
	// user_data（多行，含 # 注释）必须编码为合法 HCL string。
	ud := "#cloud-config\nwrite_files:\n  - path: /etc/x\n"
	got, err := tfVarAssignment("user_data", ud)
	require.NoError(t, err)
	require.Equal(t, "user_data="+strconv.Quote(ud), got)

	// 数字变量保持裸 number。
	got, err = tfVarAssignment("count", "2")
	require.NoError(t, err)
	require.Equal(t, "count=2", got)
	got, err = tfVarAssignment("disk_size", "40")
	require.NoError(t, err)
	require.Equal(t, "disk_size=40", got)

	// 普通字符串变量也编码（region 无特殊字符也不受影响）。
	got, err = tfVarAssignment("region", "us-east-1")
	require.NoError(t, err)
	require.Equal(t, `region="us-east-1"`, got)

	// 数字变量传非数字报错。
	_, err = tfVarAssignment("count", "abc")
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
	t.Run("count over cap", func(t *testing.T) {
		r := base()
		r.Count = maxCountPerRequest + 1
		_, err := Validate(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "count must be <=")
	})
	t.Run("count at cap", func(t *testing.T) {
		r := base()
		r.Count = maxCountPerRequest
		_, err := Validate(r)
		require.NoError(t, err)
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
