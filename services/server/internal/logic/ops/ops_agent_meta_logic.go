// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/datatypes"
)

type OpsAgentMetaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新代理元数据
func NewOpsAgentMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentMetaLogic {
	return &OpsAgentMetaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentMetaLogic) OpsAgentMeta(req *types.OpsAgentMetaUpdateRequest) (*types.OpsAgentMetaResponse, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}
	agentID, err := utils.ValidateNodeID(req.AgentID)
	if err != nil {
		return nil, err
	}

	metaPayload, ok := req.Meta.(map[string]interface{})
	if !ok {
		return nil, errors.New("meta 必须是对象")
	}
	metaValues := map[string]string{}
	for k, v := range metaPayload {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		value := strings.TrimSpace(fmt.Sprint(v))
		metaValues[key] = value
	}

	if len(metaValues) == 0 {
		return nil, errors.New("meta 内容不能为空")
	}

	if err := l.updateNodeMeta(agentID, metaValues); err != nil {
		return nil, err
	}
	l.updateRegistry(agentID, metaValues)

	return &types.OpsAgentMetaResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"agentId": agentID,
			"meta":    metaValues,
		},
	}, nil
}

func (l *OpsAgentMetaLogic) updateNodeMeta(agentID string, metaValues map[string]string) error {
	var node *model.Node
	var err error
	if node, err = l.svcCtx.NodeModel.FindByNodeID(l.ctx, agentID); err != nil {
		node = &model.Node{
			NodeID: agentID,
			Name:   agentID,
			Type:   "agent",
			Status: "unknown",
		}
	}

	var merged datatypes.JSONMap
	if node.Meta != nil {
		merged = cloneJSONMap(node.Meta)
	} else {
		merged = datatypes.JSONMap{}
	}
	for k, v := range metaValues {
		merged[k] = v
	}

	if node.ID == 0 {
		node.Meta = merged
		return l.svcCtx.NodeModel.Upsert(l.ctx, node)
	}

	return l.svcCtx.NodeModel.UpdateMeta(l.ctx, agentID, map[string]interface{}{
		"meta":       merged,
		"updated_at": time.Now(),
	})
}

func (l *OpsAgentMetaLogic) updateRegistry(agentID string, metaValues map[string]string) {
	store := l.svcCtx.RegistryStore
	if store == nil {
		return
	}
	store.Mu().Lock()
	defer store.Mu().Unlock()
	sess := store.AgentsUnsafe()[agentID]
	if sess == nil {
		return
	}
	if sess.Labels == nil {
		sess.Labels = map[string]string{}
	}
	if region, ok := metaValues["region"]; ok {
		sess.Region = region
	}
	if zone, ok := metaValues["zone"]; ok {
		sess.Zone = zone
	}
	for k, v := range metaValues {
		sess.Labels["meta."+k] = v
	}
}

func cloneJSONMap(src datatypes.JSONMap) datatypes.JSONMap {
	if src == nil {
		return datatypes.JSONMap{}
	}
	dst := datatypes.JSONMap{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
