package handler

import (
	"net/http"
	"strings"
)

func resolveAnalyticsScope(r *http.Request, game, env string) (string, string) {
	game = strings.TrimSpace(game)
	env = strings.TrimSpace(env)
	if game == "" {
		game = strings.TrimSpace(r.Header.Get("X-Game-ID"))
	}
	if env == "" {
		env = strings.TrimSpace(r.Header.Get("X-Env"))
	}
	return game, env
}
