package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewService(t *testing.T) {
	svc := NewService(nil, nil)
	assert.NotNil(t, svc)
}

func TestMatchesAgentTarget(t *testing.T) {
	tests := []struct {
		name       string
		targetType string
		targetID   string
		agentID    string
		want       bool
	}{
		{"agent type matches", "agent", "agent-1", "agent-1", true},
		{"agent type no match", "agent", "agent-1", "agent-2", false},
		{"agent type empty target", "agent", "", "agent-1", false},
		{"group default", "agent_group", "default", "any", true},
		{"group all", "group", "all", "any", true},
		{"group star", "agent_group", "*", "any", true},
		{"group specific", "agent_group", "group-1", "any", false},
		{"global type", "global", "", "any", true},
		{"all type", "all", "", "any", true},
		{"any type", "any", "", "any", true},
		{"broadcast type", "broadcast", "", "any", true},
		{"empty type", "", "", "any", true},
		{"unknown type", "unknown", "", "any", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAgentTarget(tt.targetType, tt.targetID, tt.agentID)
			assert.Equal(t, tt.want, got)
		})
	}
}
