package utils

import (
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
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
func BuildNode(node *model.Node) Node {
	var resources interface{} = node.Resources
	if resources == nil {
		resources = map[string]interface{}{}
	}
	return Node{
		Id:        node.NodeID,
		Name:      node.Name,
		Type:      node.Type,
		Status:    node.Status,
		IP:        node.IP,
		Port:      node.Port,
		Resources: resources,
		UpdatedAt: helper.FormatTimestamp(node.UpdatedAt),
	}
}

// Local types for backward compatibility
type Node struct {
	Id        string      `json:"id"`
	Name      string      `json:"name"`
	Type      string      `json:"type"`
	Status    string      `json:"status"`
	IP        string      `json:"ip"`
	Port      int         `json:"port"`
	Resources interface{} `json:"resources"`
	UpdatedAt string      `json:"updatedAt"`
}
