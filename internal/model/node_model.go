package model

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// NodeModel manages node metadata.
type NodeModel struct {
	db *gorm.DB
}

// NewNodeModel returns helper.
func NewNodeModel(db *gorm.DB) *NodeModel {
	return &NodeModel{db: db}
}

// ListNodesOptions controls filtering.
type ListNodesOptions struct {
	Type   string
	Status string
}

// Upsert stores node info.
func (m *NodeModel) Upsert(ctx context.Context, node *Node) error {
	return m.db.WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(node).Error
}

// List returns nodes for filters.
func (m *NodeModel) List(ctx context.Context, opts ListNodesOptions) ([]Node, error) {
	query := m.db.WithContext(ctx).Model(&Node{})
	if opts.Type != "" {
		query = query.Where("type = ?", opts.Type)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}

	var nodes []Node
	if err := query.Order("updated_at DESC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// UpdateMeta updates node metadata map.
func (m *NodeModel) UpdateMeta(ctx context.Context, nodeID string, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).
		Model(&Node{}).
		Where("node_id = ?", nodeID).
		Updates(updates).Error
}

// FindByNodeID fetches a node by its node_id.
func (m *NodeModel) FindByNodeID(ctx context.Context, nodeID string) (*Node, error) {
	var node Node
	if err := m.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("节点不存在")
		}
		return nil, err
	}
	return &node, nil
}

// UpdateStatus updates node status.
func (m *NodeModel) UpdateStatus(ctx context.Context, nodeID, status string) error {
	return m.db.WithContext(ctx).
		Model(&Node{}).
		Where("node_id = ?", nodeID).
		Update("status", status).Error
}

// ListCommands returns registered commands.
func (m *NodeModel) ListCommands(ctx context.Context) ([]NodeCommand, error) {
	var commands []NodeCommand
	if err := m.db.WithContext(ctx).
		Order("name ASC").
		Find(&commands).Error; err != nil {
		return nil, err
	}
	return commands, nil
}

// UpsertCommand stores a node command.
func (m *NodeModel) UpsertCommand(ctx context.Context, command *NodeCommand) error {
	return m.db.WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(command).Error
}
