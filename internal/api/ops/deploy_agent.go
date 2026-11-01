package ops

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// DeployAgent 是部署编排的 agent 投递/健康适配器（OPS-004）。生产实现复用
// 既有 AgentService：投递=对象存储签名 URL → agent 本地下载（经 ExecCommand，
// 宿主机需有下载工具且在命令白名单）；换包=UpdateProcess；健康=探针/资源门禁。
type DeployAgent struct {
	ctx         *svc.ServiceContext
	agentSvc    *AgentService
	downloadCmd string
}

// NewDeployAgent 构造生产适配器（供 handler 装配注入 svc.Deployments）。
func NewDeployAgent(ctx *svc.ServiceContext) *DeployAgent {
	return &DeployAgent{
		ctx:      ctx,
		agentSvc: NewAgentService(ctx),
		// 默认 curl；宿主机仅有 wget 时由调用方经 DownloadCmd 设置。
		downloadCmd: "curl",
	}
}

// DownloadCmd 设置下载命令（curl|wget）。
func (a *DeployAgent) DownloadCmd(cmd string) { a.downloadCmd = cmd }

// StagePackage 生成对象存储签名 URL 并让 agent 下载到本地 staging。
func (a *DeployAgent) StagePackage(ctx context.Context, agentID string, pkg *model.DeployPackage) (string, error) {
	if a.ctx == nil || a.ctx.ObjectStore == nil {
		return "", fmt.Errorf("object store not configured")
	}
	url, err := a.ctx.ObjectStore.SignedURL(ctx, pkg.ObjectKey, "GET", 5*time.Minute)
	if err != nil {
		return "", fmt.Errorf("signed url: %w", err)
	}
	staged := fmt.Sprintf("/tmp/croupier-deploy-%s-%s", pkg.PackageID, pkg.Version)
	var cmd string
	var args []string
	if a.downloadCmd == "wget" {
		cmd, args = "wget", []string{"-O", staged, url}
	} else {
		cmd, args = "curl", []string{"-fsSL", url, "-o", staged}
	}
	res, err := a.agentSvc.ExecCommand(ctx, agentID, cmd, args, 120)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("download exit %d: %s", res.ExitCode, res.Stdout)
	}
	return staged, nil
}

// ApplyUpdate 换包。
func (a *DeployAgent) ApplyUpdate(ctx context.Context, agentID, processName, stagedPath, expectedSha256 string) (string, error) {
	resp, err := a.agentSvc.UpdateProcess(ctx, agentID, processName, stagedPath, expectedSha256)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Message)
	}
	return resp.NewFingerprint, nil
}

// Healthy 用 MetricsStore 最新探针判定（资源门禁简化为探针失败即不健康）。
func (a *DeployAgent) Healthy(ctx context.Context, agentID string) (bool, string) {
	if a.ctx == nil || a.ctx.MetricsStore == nil {
		return true, "no metrics store; assume healthy"
	}
	entry, ok := a.ctx.MetricsStore.GetLatest(agentID)
	if !ok || entry.Report == nil {
		return false, "no recent metrics"
	}
	for _, p := range entry.Report.GetProbeStats() {
		if p.GetFailed() > 0 && p.GetChecked() > 0 {
			return false, "probe failed: " + p.GetTarget()
		}
	}
	return true, ""
}
