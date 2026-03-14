
package ops

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
	
)

type OpsServicesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取服务列表
func NewOpsServicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsServicesLogic {
	return &OpsServicesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsServicesLogic) OpsServices(_ *OpsServicesRequest) (*OpsServicesResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看服务列表", "admin:all", "ops:read", "registry:read"); err != nil {
		return nil, err
	}

	services := make([]OpsServiceItem, 0)

	// Add server itself
	serverAddr := fmt.Sprintf("%s:%d", l.svcCtx.Config.Server.Host, l.svcCtx.Config.Server.Port)
	if l.svcCtx.Config.Server.Host == "0.0.0.0" {
		serverAddr = fmt.Sprintf("localhost:%d", l.svcCtx.Config.Server.Port)
	}

	// 收集系统标签并合并配置中的 labels
	labels := collectServerLabels()
	for k, v := range l.svcCtx.Config.Labels {
		labels[k] = v
	}

	services = append(services, OpsServiceItem{
		ID:       "server",
		Name:     "croupier-server",
		Type:     "server",
		Status:   "running",
		Address:  serverAddr,
		GameID:   "",
		Env:      "",
		Version:  l.svcCtx.ServerVersion,
		Region:   l.svcCtx.Config.Region,
		Zone:     l.svcCtx.Config.Zone,
		Labels:   labels,
		LastSeen: l.svcCtx.StartTime.Format(time.RFC3339),
	})

	// Add agents from registry
	if store := l.svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if sess == nil || strings.TrimSpace(sess.AgentID) == "" {
				continue
			}

			ttl, healthy := ttlAndHealth(sess)
			status := "healthy"
			if ttl <= 0 {
				status = "expired"
			} else if !healthy {
				status = "unhealthy"
			}

			labels := sess.Labels
			if labels == nil {
				labels = make(map[string]string)
			}

			// 转换进程信息
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

			services = append(services, OpsServiceItem{
				ID:             sess.AgentID,
				Name:           sess.AgentID,
				Type:           "agent",
				Status:         status,
				Address:        sess.RPCAddr,
				GameID:         sess.GameID,
				Env:            sess.Env,
				Version:        sess.Version,
				Region:         sess.Region,
				Zone:           sess.Zone,
				Labels:         labels,
				FunctionsCount: utils.CountEnabledFunctions(sess.Functions),
				LastSeen:       formatLastSeen(sess.LastSeen, sess.ExpireAt),
				Metadata:       metadata,
			})
		}
		store.Mu().RUnlock()
	}

	return &OpsServicesResponse{
		Services: services,
		Total:    len(services),
	}, nil
}

// formatLastSeen 格式化最后活跃时间，优先使用 LastSeen，为零值时回退到 ExpireAt
func formatLastSeen(lastSeen, expireAt time.Time) string {
	if !lastSeen.IsZero() {
		return lastSeen.Format(time.RFC3339)
	}
	// 兼容旧数据：如果没有 LastSeen，使用 ExpireAt - 30秒
	return expireAt.Add(-time.Second * 30).Format(time.RFC3339)
}

func ttlAndHealth(sess *registry.AgentSession) (int, bool) {
	if sess == nil || sess.ExpireAt.IsZero() {
		return 0, false
	}
	ttl := int(time.Until(sess.ExpireAt).Seconds())
	if ttl < 0 {
		ttl = 0
	}
	return ttl, ttl > 0
}

// collectServerLabels 收集 Server 系统信息作为标签
func collectServerLabels() map[string]string {
	labels := make(map[string]string)

	// 操作系统
	labels["os"] = runtime.GOOS
	// CPU 架构
	labels["arch"] = runtime.GOARCH

	// 主机名
	if hostname, err := os.Hostname(); err == nil {
		labels["hostname"] = hostname
	}

	// 获取第一个非回环 IP 地址
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					labels["ip"] = ipnet.IP.String()
					break
				}
			}
		}
	}

	// CPU 核心数
	labels["cpu_count"] = fmt.Sprintf("%d", runtime.NumCPU())

	// Go 版本
	labels["go_version"] = runtime.Version()

	return labels
}
