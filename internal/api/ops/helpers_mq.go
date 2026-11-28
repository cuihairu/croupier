package ops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

func opsMQ(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMQRequest) (*OpsMQResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return &OpsMQResponse{
			Result: map[string]interface{}{},
		}, nil
	}

	state := svcCtx.OpsStateStore.Snapshot()
	result := map[string]interface{}{
		"type":      state.MQ.Type,
		"updatedAt": utils.FormatTimestamp(state.MQ.UpdatedAt),
	}

	if state.MQ.Redis != nil {
		result["redis"] = map[string]interface{}{
			"url":     state.MQ.Redis.URL,
			"streams": state.MQ.Redis.Streams,
		}
	}
	if state.MQ.Kafka != nil {
		result["kafka"] = map[string]interface{}{
			"brokers": state.MQ.Kafka.Brokers,
			"topics":  state.MQ.Kafka.Topics,
		}
	}
	if len(state.MQ.Lengths) > 0 {
		result["lengths"] = state.MQ.Lengths
	}
	if len(state.MQ.Groups) > 0 {
		groups := make([]map[string]interface{}, 0, len(state.MQ.Groups))
		for _, g := range state.MQ.Groups {
			groups = append(groups, map[string]interface{}{
				"stream":    g.Stream,
				"name":      g.Name,
				"consumers": g.Consumers,
				"pending":   g.Pending,
				"lag":       g.Lag,
			})
		}
		result["groups"] = groups
	}

	return &OpsMQResponse{
		Result: result,
	}, nil
}

func opsServices(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsServicesRequest) (*OpsServicesResponse, error) {
	if svcCtx == nil {
		return &OpsServicesResponse{
			Services: []OpsServiceItem{},
			Total:    0,
		}, nil
	}

	items := make([]OpsServiceItem, 0)
	serverAddr := fmt.Sprintf("%s:%d", svcCtx.Config.Server.Host, svcCtx.Config.Server.Port)
	if svcCtx.Config.Server.Host == "" || svcCtx.Config.Server.Host == "0.0.0.0" {
		serverAddr = fmt.Sprintf("localhost:%d", svcCtx.Config.Server.Port)
	}
	lastSeen := ""
	if !svcCtx.StartTime.IsZero() {
		lastSeen = svcCtx.StartTime.Format(time.RFC3339)
	}
	items = append(items, OpsServiceItem{
		ID:       "server",
		Name:     "croupier-server",
		Type:     "server",
		Status:   "running",
		Address:  serverAddr,
		Version:  svcCtx.ServerVersion,
		Region:   svcCtx.Config.Region,
		Zone:     svcCtx.Config.Zone,
		Labels:   svcCtx.Config.Labels,
		LastSeen: lastSeen,
	})

	if svcCtx.RegistryStore != nil {
		svcCtx.RegistryStore.Mu().RLock()
		for _, sess := range svcCtx.RegistryStore.AgentsUnsafe() {
			if sess == nil || strings.TrimSpace(sess.AgentID) == "" {
				continue
			}

			status := "healthy"
			if !sess.ExpireAt.IsZero() && time.Now().After(sess.ExpireAt) {
				status = "expired"
			}

			var metadata *OpsServiceMetadata
			if len(sess.Providers) > 0 {
				processes := make([]OpsServiceProcess, 0, len(sess.Providers))
				for _, p := range sess.Providers {
					processes = append(processes, OpsServiceProcess{
						ServiceID:    p.ProviderID,
						Addr:         p.Addr,
						Version:      p.Version,
						LastSeenUnix: p.LastSeenUnix,
						FunctionIDs:  p.FunctionIDs,
						Functions:    len(p.FunctionIDs),
					})
				}
				metadata = &OpsServiceMetadata{
					Processes:      processes,
					ProcessesCount: len(processes),
				}
			}

			labels := sess.Labels
			if labels == nil {
				labels = map[string]string{}
			}

			items = append(items, OpsServiceItem{
				ID:             sess.AgentID,
				Name:           sess.AgentID,
				Type:           "agent",
				Status:         status,
				Address:        sess.Addr,
				GameID:         sess.GameID,
				Env:            sess.Env,
				Version:        sess.Version,
				Region:         sess.Region,
				Zone:           sess.Zone,
				Labels:         labels,
				FunctionsCount: utils.CountEnabledFunctions(sess.Functions),
				LastSeen: func() string {
					if !sess.LastSeen.IsZero() {
						return sess.LastSeen.Format(time.RFC3339)
					}
					return ""
				}(),
				Metadata: metadata,
			})
		}
		svcCtx.RegistryStore.Mu().RUnlock()
	}

	return &OpsServicesResponse{
		Services: items,
		Total:    len(items),
	}, nil
}

func opsServicesLegacyCompatible(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsServicesRequest) (*OpsServicesResponse, error) {
	resp, err := opsServicesFn(ctx, svcCtx, req)
	if err != nil {
		return nil, err
	}

	legacy := make([]OpsServiceItem, 0, len(resp.Services))
	for _, svc := range resp.Services {
		var metadata *OpsServiceMetadata
		if svc.Metadata != nil {
			processes := make([]OpsServiceProcess, 0, len(svc.Metadata.Processes))
			for _, p := range svc.Metadata.Processes {
				processes = append(processes, OpsServiceProcess{
					ServiceID:    p.ServiceID,
					Addr:         p.Addr,
					Version:      p.Version,
					LastSeenUnix: p.LastSeenUnix,
					FunctionIDs:  p.FunctionIDs,
					Functions:    p.Functions,
				})
			}
			metadata = &OpsServiceMetadata{
				Processes:      processes,
				ProcessesCount: svc.Metadata.ProcessesCount,
			}
		}

		legacy = append(legacy, OpsServiceItem{
			ID:             svc.ID,
			Name:           svc.Name,
			Type:           svc.Type,
			Status:         svc.Status,
			Address:        svc.Address,
			GameID:         svc.GameID,
			Env:            svc.Env,
			Version:        svc.Version,
			Region:         svc.Region,
			Zone:           svc.Zone,
			Labels:         svc.Labels,
			FunctionsCount: svc.FunctionsCount,
			LastSeen:       svc.LastSeen,
			Metadata:       metadata,
		})
	}

	return &OpsServicesResponse{
		Services: legacy,
		Total:    resp.Total,
	}, nil
}
