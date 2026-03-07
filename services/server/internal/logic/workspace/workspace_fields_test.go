package workspace

import (
	"testing"

	"github.com/cuihairu/croupier/services/server/internal/types"
)

func TestResolveWorkspaceStatus(t *testing.T) {
	tests := []struct {
		name   string
		config *types.WorkspaceConfig
		want   string
	}{
		{
			name:   "nil config defaults draft",
			config: nil,
			want:   workspaceStatusDraft,
		},
		{
			name: "published has highest priority",
			config: &types.WorkspaceConfig{
				Published: true,
				Status:    workspaceStatusArchived,
			},
			want: workspaceStatusPublished,
		},
		{
			name: "archived is preserved",
			config: &types.WorkspaceConfig{
				Published: false,
				Status:    workspaceStatusArchived,
			},
			want: workspaceStatusArchived,
		},
		{
			name: "invalid status falls back to draft",
			config: &types.WorkspaceConfig{
				Published: false,
				Status:    "unknown_status",
			},
			want: workspaceStatusDraft,
		},
		{
			name: "published status text without flag falls back to draft",
			config: &types.WorkspaceConfig{
				Published: false,
				Status:    workspaceStatusPublished,
			},
			want: workspaceStatusDraft,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveWorkspaceStatus(tt.config); got != tt.want {
				t.Fatalf("resolveWorkspaceStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
