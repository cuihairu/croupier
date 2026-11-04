package provision

import (
	"fmt"
	"net/url"
	"strings"
	"text/template"
)

// ProvisionLabelKey 是 agent 自注册标签键（等待注册按此标签匹配——agent 的
// AgentID 由 agent 自身决定，不与 terraform instance_id 等同，须以供给运行
// 专属标签关联，见包注释 §自注册）。
const ProvisionLabelKey = "croupier-provisioned"

// CloudInitOptions 是 cloud-init 渲染输入。
type CloudInitOptions struct {
	// HubAddr 是 agent 拨入的 hub 地址（host:port）。
	HubAddr string
	// BootstrapToken 是 agent 注册 token（敏感）。
	BootstrapToken string
	// AgentInstallURL 是 agent 安装脚本地址；空则跳过安装（仅写配置）。
	AgentInstallURL string
	// RunID 是供给运行 id；以 ProvisionLabelKey 标签注入 agent，供等待注册。
	RunID string
	// Extra 是追加到 runcmd 的自定义片段（每条一行）。
	Extra string
}

// shellMetachars 是 cloud-init runcmd 注入原语：命令替换（$()/反引号）、命令
// 分隔（;）、管道（|）、串联/后台（&）、重定向（<>）、子 shell（()）。cloud-init
// 对字符串形式的 runcmd 条目以 sh -c 求值，这些字符会让条目变成任意 shell，
// 故逐行拒绝（C-002）。列表形式 [ cmd, arg ] 不经 shell，仅受命令白名单约束。
const shellMetachars = "`$;|&<>()"

// shellInterpTokens 是会被当作解释器求值的命令：以 -c/-e 求值字符串即注入
// 载体（列表 [ sh, -c, '...' ] 或字符串 sh -c '...' 都绕过元字符检查）。
var shellInterpTokens = map[string]bool{
	"sh": true, "bash": true, "dash": true, "zsh": true, "ksh": true,
	"csh": true, "ash": true, "python": true, "python2": true, "python3": true,
	"perl": true, "php": true, "ruby": true, "node": true, "nodejs": true,
}

// evalFlags 是解释器「把下一个参数当脚本求值」的开关（-c/-e 及组合）。
func hasEvalFlag(line string) bool {
	for _, tok := range strings.Fields(line) {
		tok = strings.TrimRight(tok, ",]")
		switch tok {
		case "-c", "-e", "-ec", "-ce", "-lc", "-cl", "--command":
			return true
		}
	}
	return false
}

// firstCommand 取 runcmd 条目的命令 token：列表形式 [ cmd, args ] 取首个元素，
// 字符串形式取首个空白分隔字段。
func firstCommand(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "[") {
		inner := strings.TrimSpace(strings.TrimPrefix(line, "["))
		if idx := strings.Index(inner, ","); idx >= 0 {
			return strings.TrimSpace(inner[:idx])
		}
		return inner
	}
	if fields := strings.Fields(line); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// validateCloudInitExtraLine 校验单行 runcmd 条目。空行跳过；含 shell 元字符、
// 以 '-' 开头（YAML 嵌套）、或「解释器 + eval 标志」注入的条目被拒。
func validateCloudInitExtraLine(line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if strings.HasPrefix(line, "-") {
		return fmt.Errorf("provision: cloudInitExtra line must not start with '-': %q", line)
	}
	if strings.ContainsAny(line, shellMetachars) {
		return fmt.Errorf("provision: cloudInitExtra line contains shell metacharacters: %q", line)
	}
	if shellInterpTokens[firstCommand(line)] && hasEvalFlag(line) {
		return fmt.Errorf("provision: cloudInitExtra line invokes shell interpreter with eval flag: %q", line)
	}
	return nil
}

// validateCloudInitExtra 校验整个 CloudInitExtra（逐行）。
func validateCloudInitExtra(extra string) error {
	for _, line := range strings.Split(extra, "\n") {
		if err := validateCloudInitExtraLine(line); err != nil {
			return err
		}
	}
	return nil
}

// validateAgentInstallURL 校验 agent 安装脚本 URL。要求非空时是 http(s) URL
// 且不含空白/控制字符（换行会破坏 cloud-init YAML、空白会拆 shell 词）。
// shell 元字符（;|&$`() 等）本身经 shellSingleQuote 在渲染边界加引号中和，
// 故不在此拒绝；仅做 URL 形态约束（#673）。
func validateAgentInstallURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if strings.IndexFunc(raw, func(r rune) bool {
		return r <= ' ' || r == 0x7f
	}) >= 0 {
		return fmt.Errorf("provision: agentInstallUrl must not contain whitespace or control chars: %q", raw)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("provision: agentInstallUrl must be an http(s) URL: %q", raw)
	}
	return nil
}

// shellSingleQuote 把字符串包装为单引号 shell 字面量（'...'），内嵌单引号
// 以 '\” 转义。用于把 URL 作为单一参数传入 sh -c 脚本，防命令注入。
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// renderCloudInit 渲染 cloud-config 文本：写 /etc/croupier/agent.yaml（hub 地址 +
// bootstrap token + 供给标签），下载安装 agent，启用服务。cloud-init 经
// base64encode 注入 user_data，内容不落 terraform state（敏感 token 只出现
// 在实例用户数据，不入 provision_runs 表）。Extra 逐行经 shell 注入校验
// （C-002），非法返回 error（Validate 边界已拦，此处防御纵深）。
func renderCloudInit(o CloudInitOptions) (string, error) {
	if err := validateCloudInitExtra(o.Extra); err != nil {
		return "", err
	}
	if err := validateAgentInstallURL(o.AgentInstallURL); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("#cloud-config\n")
	b.WriteString("write_files:\n")
	b.WriteString("  - path: /etc/croupier/agent.yaml\n")
	b.WriteString("    content: |\n")
	fmt.Fprintf(&b, "      hubAddr: %s\n", o.HubAddr)
	fmt.Fprintf(&b, "      bootstrapToken: %s\n", o.BootstrapToken)
	if o.RunID != "" {
		fmt.Fprintf(&b, "      labels:\n        %s: %s\n", ProvisionLabelKey, o.RunID)
	}
	b.WriteString("runcmd:\n")
	if o.AgentInstallURL != "" {
		installCmd := fmt.Sprintf(
			"curl -fsSL %s -o /tmp/croupier-agent-install.sh && bash /tmp/croupier-agent-install.sh",
			shellSingleQuote(o.AgentInstallURL),
		)
		fmt.Fprintf(&b, "  - [ sh, -c, '%s' ]\n",
			strings.ReplaceAll(installCmd, "'", "''"))
	}
	if o.Extra != "" {
		for _, line := range strings.Split(o.Extra, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				fmt.Fprintf(&b, "  - %s\n", line)
			}
		}
	}
	return b.String(), nil
}

// TemplateVars 是 provider 模板的公共变量（HCL 内以 var.xxx 引用）。
type TemplateVars struct {
	Region       string
	InstanceType string
	Image        string
	DiskSizeGB   int
	UserData     string
	Count        int
	NamePrefix   string
}

// templates 是各 provider 的 main.tf 模板。变量块统一，resource 依 provider
// 命名（alicloud_instance/tencentcloud_instance/aws_instance/hcloud_server），
// 输出 instance_id(s) + ip(s) 供编排等待注册与纳管。模板只做资源声明，
// 凭据全部走 provider 环境变量（阿里云 ALICLOUD_ACCESS_KEY_ID 等），不入模板。
var templates = map[Provider]string{
	ProviderAliyun: `variable "region" {}
variable "instance_type" {}
variable "image" { default = "" }
variable "disk_size" { default = 40 }
variable "user_data" { default = "" }
variable "count" { default = 1 }
variable "name_prefix" { default = "croupier" }

provider "alicloud" {
  region = var.region
}

resource "alicloud_security_group" "node" {
  count = 1
  name  = "${var.name_prefix}-sg"
  description = "croupier provisioned node (managed by croupier)"
  tags = { managed-by = "croupier" }
}

resource "alicloud_instance" "node" {
  count           = var.count
  instance_type   = var.instance_type
  image_id        = var.image
  system_disk_category = "cloud_efficiency"
  system_disk_size     = var.disk_size
  security_groups = alicloud_security_group.node[*].id
  user_data       = base64encode(var.user_data)
  instance_name   = "${var.name_prefix}-${count.index}"
  tags = { managed-by = "croupier" }
}

output "instance_id" { value = alicloud_instance.node[*].id }
output "public_ip"   { value = alicloud_instance.node[*].public_ip }
`,

	ProviderTencent: `variable "region" {}
variable "instance_type" {}
variable "image" { default = "" }
variable "disk_size" { default = 50 }
variable "user_data" { default = "" }
variable "count" { default = 1 }
variable "name_prefix" { default = "croupier" }

provider "tencentcloud" {
  region = var.region
}

resource "tencentcloud_instance" "node" {
  count           = var.count
  instance_type   = var.instance_type
  image_id        = var.image
  system_disk_type = "CLOUD_PREMIUM"
  system_disk_size = var.disk_size
  user_data       = base64encode(var.user_data)
  instance_name   = "${var.name_prefix}-${count.index}"
  tags = { managed-by = "croupier" }
}

output "instance_id" { value = tencentcloud_instance.node[*].id }
output "public_ip"   { value = tencentcloud_instance.node[*].public_ip }
`,

	ProviderAWS: `variable "region" {}
variable "instance_type" {}
variable "image" { default = "" }
variable "disk_size" { default = 20 }
variable "user_data" { default = "" }
variable "count" { default = 1 }
variable "name_prefix" { default = "croupier" }

provider "aws" {
  region = var.region
}

resource "aws_instance" "node" {
  count         = var.count
  ami           = var.image
  instance_type = var.instance_type
  user_data     = var.user_data
  root_block_device {
    volume_size = var.disk_size
  }
  tags = {
    Name       = "${var.name_prefix}-${count.index}"
    managed-by = "croupier"
  }
}

output "instance_id" { value = aws_instance.node[*].id }
output "public_ip"   { value = aws_instance.node[*].public_ip }
`,

	ProviderHetzner: `variable "region" {}
variable "instance_type" {}
variable "image" { default = "ubuntu-24.04" }
variable "disk_size" { default = 20 }
variable "user_data" { default = "" }
variable "count" { default = 1 }
variable "name_prefix" { default = "croupier" }

provider "hcloud" {}

resource "hcloud_server" "node" {
  count       = var.count
  name        = "${var.name_prefix}-${count.index}"
  server_type = var.instance_type
  image       = var.image
  location    = var.region
  user_data   = var.user_data
  labels = {
    managed-by = "croupier"
  }
}

output "instance_id" { value = hcloud_server.node[*].id }
output "public_ip"   { value = hcloud_server.node[*].ipv4_address }
`,
}

// renderTemplate 渲染 provider 的 main.tf。模板为纯 HCL（无 Go 模板占位符），
// 变量值经 apply 的 -var 注入（vars 传 TemplateVars 仅为校验/文档，实际值
// 由 Runner.Apply 以 TF_VAR_*/-var 传入）。cloud-init 不经模板内嵌——它作为
// user_data 变量在 apply 时传入，敏感 token 不落工作目录 main.tf。
func renderTemplate(p Provider, req Request) (string, error) {
	tpl, ok := templates[p]
	if !ok {
		return "", fmt.Errorf("provision: unsupported provider %q", p)
	}
	// 仅校验模板可解析（值注入在 apply，模板本身静态）。
	if _, err := template.New("main.tf").Parse(tpl); err != nil {
		return "", fmt.Errorf("provision: parse template for %s: %w", p, err)
	}
	_ = TemplateVars{
		Region: req.Region, InstanceType: req.Spec.InstanceType, Image: req.Spec.Image,
		DiskSizeGB: req.Spec.DiskSizeGB, Count: req.Count,
		NamePrefix: "croupier-" + req.GameID + "-" + req.Env,
	}
	return tpl, nil
}
