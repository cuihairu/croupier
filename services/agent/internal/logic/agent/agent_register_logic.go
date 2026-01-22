// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentRegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentRegisterLogic {
	return &AgentRegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// collectSystemLabels 收集系统信息作为标签
func collectSystemLabels() map[string]string {
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

// mergeLabels 合并请求中的 metadata 和配置中的 labels，系统信息优先级最高
func mergeLabels(reqMetadata map[string]string, cfgLabels map[string]string) map[string]string {
	result := make(map[string]string)

	// 先添加配置文件中的 labels
	for k, v := range cfgLabels {
		result[k] = v
	}

	// 请求中的 metadata 覆盖配置
	for k, v := range reqMetadata {
		result[k] = v
	}

	// 系统信息覆盖所有（自动检测的信息优先级最高）
	sysLabels := collectSystemLabels()
	for k, v := range sysLabels {
		result[k] = v
	}

	return result
}

func (l *AgentRegisterLogic) AgentRegister(req *types.AgentRegisterRequest) (*types.AgentRegisterResponse, error) {
	if req == nil {
		return nil, errors.New("missing request payload")
	}

	if l.svcCtx.Core == nil {
		return nil, errors.New("agent core is not running")
	}

	configuredID := strings.TrimSpace(l.svcCtx.Config.Agent.ID)
	agentID := strings.TrimSpace(req.AgentId)
	if agentID == "" {
		agentID = configuredID
	}
	if agentID == "" {
		return nil, errors.New("agent_id 不能为空")
	}
	if configuredID != "" && agentID != configuredID {
		return nil, errors.New("agent_id mismatch")
	}

	// 合并 labels：请求 metadata + 配置 labels + 系统信息
	labels := mergeLabels(req.Metadata, l.svcCtx.Config.Agent.Labels)

	meta := agentcore.UpstreamMetadata{
		GameID:  defaultString(strings.TrimSpace(req.GameId), strings.TrimSpace(l.svcCtx.Config.Agent.GameID)),
		Env:     defaultString(strings.TrimSpace(req.Env), strings.TrimSpace(l.svcCtx.Config.Agent.Env)),
		Version: defaultString(strings.TrimSpace(req.Version), ""),
		RPCAddr: defaultString(strings.TrimSpace(req.RpcAddr), strings.TrimSpace(l.svcCtx.Config.Agent.LocalAddr)),
		Region:  defaultString(strings.TrimSpace(req.Region), strings.TrimSpace(l.svcCtx.Config.Agent.Region)),
		Zone:    defaultString(strings.TrimSpace(req.Zone), strings.TrimSpace(l.svcCtx.Config.Agent.Zone)),
		Labels:  labels,
	}
	l.svcCtx.Core.WithUpstreamMetadata(meta)

	syncCtx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()
	if err := l.svcCtx.Core.SyncUpstream(syncCtx); err != nil {
		l.Errorf("upstream sync skipped/failed: %v", err)
	}

	token := fmt.Sprintf("%s:%d", agentID, time.Now().Unix())
	l.Infof("注册代理成功: %s (%s/%s) region=%s zone=%s labels=%v",
		agentID, req.GameId, req.Env, meta.Region, meta.Zone, labels)

	// Set registration timestamp
	l.svcCtx.SetRegisteredAt(time.Now())

	return &types.AgentRegisterResponse{
		Success: true,
		Message: "agent registered",
		Token:   token,
	}, nil
}
