package provision

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-exec/tfexec"
)

// Runner 抽象 terraform/tofu 生命周期。生产用 TfexecRunner；测试用 FakeRunner
// 走同一编排代码路径（真实 provider 集成需云凭据 + 二进制，见包注释）。
type Runner interface {
	// Init 在 workDir 内 terraform init（下载 provider 插件）。
	Init(ctx context.Context, workDir string) error
	// Apply 在 workDir 内 terraform apply，vars 以 TF_VAR_* 注入；返回 apply 输出。
	Apply(ctx context.Context, workDir string, vars map[string]string) (map[string]string, error)
	// Destroy 在 workDir 内 terraform destroy（回收资源，防漂移）。
	Destroy(ctx context.Context, workDir string) error
}

// TfexecRunner 用 hashicorp terraform-exec 驱动二进制（tofu 或 terraform）。
type TfexecRunner struct {
	// ExecPath 是二进制路径（如 /usr/local/bin/tofu）。
	ExecPath string
}

// NewTfexecRunner 构造 TfexecRunner。
func NewTfexecRunner(execPath string) *TfexecRunner {
	return &TfexecRunner{ExecPath: execPath}
}

func (r *TfexecRunner) newTF(workDir string) (*tfexec.Terraform, error) {
	return tfexec.NewTerraform(workDir, r.ExecPath)
}

// Init 实现 Runner。
func (r *TfexecRunner) Init(ctx context.Context, workDir string) error {
	tf, err := r.newTF(workDir)
	if err != nil {
		return fmt.Errorf("provision: new terraform in %s: %w", workDir, err)
	}
	return tf.Init(ctx)
}

// Apply 实现 Runner。
func (r *TfexecRunner) Apply(ctx context.Context, workDir string, vars map[string]string) (map[string]string, error) {
	tf, err := r.newTF(workDir)
	if err != nil {
		return nil, fmt.Errorf("provision: new terraform in %s: %w", workDir, err)
	}
	opts := make([]tfexec.ApplyOption, 0, len(vars))
	for k, v := range vars {
		opts = append(opts, tfexec.Var(fmt.Sprintf("%s=%s", k, v)))
	}
	if err := tf.Apply(ctx, opts...); err != nil {
		return nil, fmt.Errorf("provision: apply in %s: %w", workDir, err)
	}
	out, err := tf.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("provision: output in %s: %w", workDir, err)
	}
	res := make(map[string]string, len(out))
	for k, v := range out {
		res[k] = string(v.Value)
	}
	return res, nil
}

// Destroy 实现 Runner。
func (r *TfexecRunner) Destroy(ctx context.Context, workDir string) error {
	tf, err := r.newTF(workDir)
	if err != nil {
		return fmt.Errorf("provision: new terraform in %s: %w", workDir, err)
	}
	return tf.Destroy(ctx)
}
