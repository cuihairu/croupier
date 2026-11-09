package settings

import (
	"context"
	"log/slog"
	"sync"

	"github.com/cuihairu/croupier/internal/model"
)

func (l *Layered) hasL3(key string) bool {
	if l == nil {
		return false
	}
	_, ok := l.l3[key]
	return ok
}

// Reload re-reads L3 after a Set/Clear so consumers converge without restart.
func (l *Layered) Reload(ctx context.Context, store *model.PlatformSettingModel) {
	if l == nil || store == nil {
		return
	}
	overrides, err := store.List(ctx)
	if err != nil {
		slog.Warn("settings: reload L3 failed", "err", err)
		return
	}
	l.mu.Lock()
	l.l3 = overrides
	l.l3Loaded = true
	l.mu.Unlock()
}

// ResetForTest clears the singleton (test-only hook, exported for
// cross-package handler tests).
func ResetForTest() {
	resetForTest()
}

// resetForTest clears the singleton (test-only).
func resetForTest() {
	layeredOnce = sync.Once{}
	layered = nil
	secretResolver = nil
}
