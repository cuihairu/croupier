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
	// node.Resources 为 datatypes.JSONMap（命名 map 类型）：即便其值为 nil，
	// 装入 interface{} 后携带类型信息，`interface == nil` 恒为 false——原先
	// 的空值替换分支从未生效，予以删除；nil JSONMap 的 MarshalJSON 会输出
	// "null"，序列化行为保持与删除前逐字节一致。
	return Node{
		Id:        node.NodeID,
		Name:      node.Name,
		Type:      node.Type,
		Status:    node.Status,
		IP:        node.IP,
		Port:      node.Port,
		Resources: node.Resources,
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
