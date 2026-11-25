package ops

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// DeployAgent is the delivery/health adapter for deployment orchestration (OPS-004).
// The production implementation reuses the existing AgentService: delivery =
// object-store signed URL → agent downloads locally (via ExecCommand; the host
// must have a download tool and it must be on the command allowlist); package swap =
// UpdateProcess; health = probe / resource gate.
type DeployAgent struct {
	ctx         *svc.ServiceContext
	agentSvc    *AgentService
	downloadCmd string
}

// NewDeployAgent constructs the production adapter (for the handler to wire into svc.Deployments).
func NewDeployAgent(ctx *svc.ServiceContext) *DeployAgent {
	return &DeployAgent{
		ctx:      ctx,
		agentSvc: NewAgentService(ctx),
		// curl by default; if the host only has wget the caller sets it via DownloadCmd.
		downloadCmd: "curl",
	}
}

// DownloadCmd sets the download command (curl|wget).
func (a *DeployAgent) DownloadCmd(cmd string) { a.downloadCmd = cmd }

// StagePackage generates an object-store signed URL and makes the agent download it to local staging.
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

// ApplyUpdate swaps the package.
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

// Healthy decides using the MetricsStore's latest probe (the resource gate is
// simplified to: probe failure means unhealthy).
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
