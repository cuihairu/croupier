package workspace

import (
	"testing"

	"github.com/cuihairu/croupier/services/server/internal/types"
)

func TestValidateWorkspaceForPublish(t *testing.T) {
	tests := []struct {
		name    string
		cfg     types.WorkspaceConfig
		wantErr bool
	}{
		{
			name: "valid tabs layout",
			cfg: types.WorkspaceConfig{
				Layout: map[string]interface{}{
					"type": "tabs",
					"tabs": []interface{}{
						map[string]interface{}{
							"layout": map[string]interface{}{"type": "form"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid timeline layout",
			cfg: types.WorkspaceConfig{
				Layout: map[string]interface{}{
					"type": "tabs",
					"tabs": []interface{}{
						map[string]interface{}{
							"layout": map[string]interface{}{"type": "timeline"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid dashboard layout",
			cfg: types.WorkspaceConfig{
				Layout: map[string]interface{}{
					"type": "tabs",
					"tabs": []interface{}{
						map[string]interface{}{
							"layout": map[string]interface{}{
								"type":  "dashboard",
								"stats": []interface{}{map[string]interface{}{"key": "kpi", "value": 1}},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid custom layout",
			cfg: types.WorkspaceConfig{
				Layout: map[string]interface{}{
					"type": "tabs",
					"tabs": []interface{}{
						map[string]interface{}{
							"layout": map[string]interface{}{
								"type":      "custom",
								"component": "CustomPanel",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "legacy single should fail",
			cfg: types.WorkspaceConfig{
				Layout: map[string]interface{}{
					"type": "tabs",
					"tabs": []interface{}{
						map[string]interface{}{
							"layout": map[string]interface{}{"type": "single"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "split without panels should fail",
			cfg: types.WorkspaceConfig{
				Layout: map[string]interface{}{
					"type": "tabs",
					"tabs": []interface{}{
						map[string]interface{}{
							"layout": map[string]interface{}{"type": "split"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "wizard without steps should fail",
			cfg: types.WorkspaceConfig{
				Layout: map[string]interface{}{
					"type": "tabs",
					"tabs": []interface{}{
						map[string]interface{}{
							"layout": map[string]interface{}{"type": "wizard"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid without items should fail",
			cfg: types.WorkspaceConfig{
				Layout: map[string]interface{}{
					"type": "tabs",
					"tabs": []interface{}{
						map[string]interface{}{
							"layout": map[string]interface{}{"type": "grid"},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkspaceForPublish(tt.cfg)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
		})
	}
}
