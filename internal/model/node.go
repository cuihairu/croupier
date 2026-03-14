package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Node stores managed node metadata.
type Node struct {
	gorm.Model
	NodeID    string `gorm:"size:64;uniqueIndex"`
	Name      string `gorm:"size:128"`
	Type      string `gorm:"size:32;index"`
	Status    string `gorm:"size:32;index"`
	IP        string `gorm:"size:64"`
	Port      int
	Resources datatypes.JSONMap `gorm:"type:json"`
	Meta      datatypes.JSONMap `gorm:"type:json"`
}

// NodeCommand describes an available agent command.
type NodeCommand struct {
	gorm.Model
	Name        string `gorm:"size:64;uniqueIndex"`
	Description string `gorm:"size:255"`
}

func (Node) TableName() string {
	return "nodes"
}

func (NodeCommand) TableName() string {
	return "node_commands"
}
