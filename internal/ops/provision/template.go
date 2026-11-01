package provision

import (
	"fmt"
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

// renderCloudInit 渲染 cloud-config 文本：写 /etc/croupier/agent.yaml（hub 地址 +
// bootstrap token + 供给标签），下载安装 agent，启用服务。cloud-init 经
// base64encode 注入 user_data，内容不落 terraform state（敏感 token 只出现
// 在实例用户数据，不入 provision_runs 表）。
func renderCloudInit(o CloudInitOptions) string {
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
		fmt.Fprintf(&b, "  - [ sh, -c, 'curl -fsSL %s -o /tmp/croupier-agent-install.sh && bash /tmp/croupier-agent-install.sh' ]\n", o.AgentInstallURL)
	}
	if o.Extra != "" {
		for _, line := range strings.Split(o.Extra, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				fmt.Fprintf(&b, "  - %s\n", line)
			}
		}
	}
	return b.String()
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
