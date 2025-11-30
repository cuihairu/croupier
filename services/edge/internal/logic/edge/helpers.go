package edge

import (
	"fmt"
	"strings"
	"time"
)

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func generateTunnelID(agentID string) string {
	return fmt.Sprintf("tunnel-%s-%d", strings.TrimSpace(agentID), time.Now().UnixNano())
}
