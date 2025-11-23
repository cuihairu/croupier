package logic

import (
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/ports"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

var allowedGameStatuses = map[string]struct{}{
	"dev":         {},
	"test":        {},
	"running":     {},
	"online":      {},
	"offline":     {},
	"maintenance": {},
}

func normalizeGameStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return "dev"
	}
	if _, ok := allowedGameStatuses[status]; ok {
		return status
	}
	return "dev"
}

func gameToInfo(g *ports.Game, envRecs []*ports.GameEnvDef) types.GameInfo {
	items := make([]types.GameEnvItem, 0, len(envRecs))
	for _, e := range envRecs {
		if e == nil {
			continue
		}
		items = append(items, types.GameEnvItem{
			Env:         e.Env,
			Description: e.Description,
			Color:       e.Color,
		})
	}
	created := ""
	updated := ""
	if g != nil {
		created = formatGameTime(g.CreatedAt)
		updated = formatGameTime(g.UpdatedAt)
	}
	return types.GameInfo{
		ID:          g.ID,
		Name:        g.Name,
		Icon:        g.Icon,
		Description: g.Description,
		Enabled:     g.Enabled,
		AliasName:   g.AliasName,
		Homepage:    g.Homepage,
		Status:      g.Status,
		GameType:    g.GameType,
		GenreCode:   g.GenreCode,
		Envs:        items,
		CreatedAt:   created,
		UpdatedAt:   updated,
	}
}

func formatGameTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
