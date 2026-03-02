package utils

import (
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

// ValidateNodeID ensures node ID is provided.
func ValidateNodeID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", errorx.NewBadRequest("节点ID不能为空")
	}
	return trimmed, nil
}

// BuildNode converts model.Node to API Node type.
func BuildNode(node *model.Node) types.Node {
	var resources interface{} = node.Resources
	if resources == nil {
		resources = map[string]interface{}{}
	}
	return types.Node{
		Id:        node.NodeID,
		Name:      node.Name,
		Type:      node.Type,
		Status:    node.Status,
		IP:        node.IP,
		Port:      node.Port,
		Resources: resources,
		UpdatedAt: FormatTimestamp(node.UpdatedAt),
	}
}
