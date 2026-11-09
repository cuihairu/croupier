package agent

import (
	"context"
	"os"
	"strings"
	"time"
)

func (a *App) startMaintenance(ctx context.Context) {
	if a == nil || a.store == nil {
		return
	}

	pruneInterval := parseDurationEnv("CROUPIER_AGENTLOCAL_PRUNE_INTERVAL", 30*time.Second)
	maxAge := parseDurationEnv("CROUPIER_AGENTLOCAL_MAX_AGE", 2*time.Minute)
	taskResultMaxAge := parseDurationEnv("CROUPIER_AGENTLOCAL_TASKRESULT_MAX_AGE", 10*time.Minute)
	if pruneInterval <= 0 || maxAge <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(pruneInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.store.Prune(maxAge)
				if taskResultMaxAge > 0 {
					a.store.CleanupOldTaskResults(taskResultMaxAge)
				}
			}
		}
	}()
}

func parseDurationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func (a *App) startExtensionSync(ctx context.Context) {
	if a == nil || a.extensionRuntime == nil {
		return
	}
	baseURL := strings.TrimSpace(os.Getenv("CROUPIER_EXTENSION_SYNC_API"))
	if baseURL == "" {
		return
	}
	interval := parseDurationEnv("CROUPIER_EXTENSION_SYNC_INTERVAL", 30*time.Second)
	// ARCH-021：受限机器身份 token（CROUPIER_EXTENSION_SYNC_TOKEN）；空=不
	// 携带，服务端 401 拒绝（不静默降级为未认证访问）。
	syncToken := strings.TrimSpace(os.Getenv("CROUPIER_EXTENSION_SYNC_TOKEN"))
	a.extensionPuller = NewExtensionSyncPuller(baseURL, a.agentID, syncToken, interval, a.extensionRuntime)
	a.extensionPuller.Start(ctx)
}
