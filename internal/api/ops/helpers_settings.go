package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/svc"
)

func opsMaintenanceGet(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMaintenanceGetRequest) (*OpsMaintenanceGetResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return &OpsMaintenanceGetResponse{
			Windows: []OpsMaintenanceWindow{},
		}, nil
	}

	state := svcCtx.OpsStateStore.Snapshot()
	windows := make([]OpsMaintenanceWindow, 0, len(state.Maintenance.Windows))
	for _, w := range state.Maintenance.Windows {
		windows = append(windows, OpsMaintenanceWindow{
			ID:          w.ID,
			GameID:      w.GameID,
			Env:         w.Env,
			Start:       w.Start,
			End:         w.End,
			Message:     w.Message,
			BlockWrites: w.BlockWrites,
		})
	}

	return &OpsMaintenanceGetResponse{
		Windows:   windows,
		UpdatedAt: utils.FormatTimestamp(state.Maintenance.UpdatedAt),
	}, nil
}

func opsMaintenanceUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMaintenanceUpdateRequest) (*OpsMaintenanceUpdateResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return nil, errors.New("ops state store unavailable")
	}

	windows := make([]svc.OpsMaintenanceWindow, 0, len(req.Windows))
	for _, w := range req.Windows {
		id := strings.TrimSpace(w.ID)
		if id == "" {
			id = fmt.Sprintf("mw-%d", time.Now().UnixNano())
		}
		windows = append(windows, svc.OpsMaintenanceWindow{
			ID:          id,
			GameID:      strings.TrimSpace(w.GameID),
			Env:         strings.TrimSpace(w.Env),
			Start:       strings.TrimSpace(w.Start),
			End:         strings.TrimSpace(w.End),
			Message:     strings.TrimSpace(w.Message),
			BlockWrites: w.BlockWrites,
		})
	}

	updated, err := svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
		state.Maintenance.Windows = windows
		state.Maintenance.UpdatedAt = time.Now()
	})
	if err != nil {
		return nil, err
	}
	_ = updated
	recordOpsAudit(ctx, svcCtx, audit.AuditEventType("maintenance.update"), "", "success", map[string]interface{}{
		"windowsCount": len(windows),
	})

	return &OpsMaintenanceUpdateResponse{
		Windows: windows,
	}, nil
}

// Services and functions implementations

func opsFunctions(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsFunctionsRequest) (*OpsFunctionsResponse, error) {
	if svcCtx == nil {
		return &OpsFunctionsResponse{
			Functions: map[string][]string{},
		}, nil
	}
	store := svcCtx.RegistryStore
	if store == nil {
		return &OpsFunctionsResponse{
			Functions: map[string][]string{},
		}, nil
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	result := map[string][]string{}
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}

		for fid := range sess.Functions {
			result[fid] = append(result[fid], sess.AgentID)
		}
	}

	return &OpsFunctionsResponse{
		Functions: result,
	}, nil
}

// Config and notifications implementations

// opsConfig resolves observability integration URLs. L3 (platform_settings
// obs.*) is authoritative; the legacy in-memory OpsStateStore is consulted
// as a fallback so pre-migration values keep working until re-saved, and
// env vars remain the bootstrap default.
func opsConfig(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsConfigRequest) (*OpsConfigResponse, error) {
	if l := settings.Current(); l != nil {
		snap := l.ObsSnapshot()
		resp := &OpsConfigResponse{
			AlertmanagerURL:   snap.AlertmanagerURL,
			GrafanaExploreURL: snap.GrafanaExploreURL,
			JaegerURL:         snap.JaegerURL,
		}
		// 兼容期：L3 未配置时回落旧内存 store（重启丢失）再回落 env。
		if svcCtx != nil && svcCtx.OpsStateStore != nil {
			state := svcCtx.OpsStateStore.Snapshot()
			if resp.AlertmanagerURL == "" {
				resp.AlertmanagerURL = state.Config.AlertmanagerURL
			}
			if resp.GrafanaExploreURL == "" {
				resp.GrafanaExploreURL = state.Config.GrafanaExploreURL
			}
			if resp.JaegerURL == "" {
				resp.JaegerURL = state.Config.JaegerURL
			}
		}
		if resp.AlertmanagerURL == "" {
			resp.AlertmanagerURL = os.Getenv("CROUPIER_ALERTMANAGER_URL")
		}
		if resp.GrafanaExploreURL == "" {
			resp.GrafanaExploreURL = os.Getenv("CROUPIER_GRAFANA_EXPLORE_URL")
		}
		if resp.JaegerURL == "" {
			resp.JaegerURL = os.Getenv("CROUPIER_JAEGER_URL")
		}
		return resp, nil
	}

	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return &OpsConfigResponse{
			AlertmanagerURL:   os.Getenv("CROUPIER_ALERTMANAGER_URL"),
			GrafanaExploreURL: os.Getenv("CROUPIER_GRAFANA_EXPLORE_URL"),
			JaegerURL:         os.Getenv("CROUPIER_JAEGER_URL"),
		}, nil
	}

	state := svcCtx.OpsStateStore.Snapshot()
	return &OpsConfigResponse{
		AlertmanagerURL:   state.Config.AlertmanagerURL,
		GrafanaExploreURL: state.Config.GrafanaExploreURL,
		JaegerURL:         state.Config.JaegerURL,
	}, nil
}
