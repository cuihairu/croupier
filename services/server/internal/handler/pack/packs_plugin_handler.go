// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var packIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// PacksPluginHandler serves pack web plugins for the dashboard.
//
// Query params:
// - pack: pack id (directory under packs root)
// - path: relative path under pack, must be within web-plugin/
func PacksPluginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		packID := strings.TrimSpace(r.URL.Query().Get("pack"))
		relPath := strings.TrimSpace(r.URL.Query().Get("path"))
		if packID == "" || relPath == "" {
			httpx.ErrorCtx(r.Context(), w, fmt.Errorf("missing pack/path"))
			return
		}
		if !packIDPattern.MatchString(packID) {
			httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid pack id"))
			return
		}

		relPath = filepath.ToSlash(relPath)
		if strings.HasPrefix(relPath, "/") || strings.Contains(relPath, "\x00") {
			httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid plugin path"))
			return
		}
		clean := filepath.Clean(relPath)
		if clean == "." || strings.HasPrefix(clean, "..") {
			httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid plugin path"))
			return
		}
		if !strings.HasPrefix(filepath.ToSlash(clean), "web-plugin/") {
			httpx.ErrorCtx(r.Context(), w, fmt.Errorf("plugin must be under web-plugin/"))
			return
		}

		packsDir := resolvePacksDir(svcCtx.Config)
		packRoot := filepath.Join(packsDir, packID)
		full := filepath.Join(packRoot, clean)

		packRootAbs, err := filepath.Abs(packRoot)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		fullAbs, err := filepath.Abs(full)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		packRootNorm := filepath.ToSlash(packRootAbs)
		fullNorm := filepath.ToSlash(fullAbs)
		sep := "/"
		if fullNorm != packRootNorm && !strings.HasPrefix(fullNorm, packRootNorm+sep) {
			httpx.ErrorCtx(r.Context(), w, fmt.Errorf("invalid plugin path"))
			return
		}

		content, err := os.ReadFile(fullAbs)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		sum := sha256.Sum256(content)
		etag := fmt.Sprintf("\"%x\"", sum[:])
		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}
}

func resolvePacksDir(cfg config.Config) string {
	dir := strings.TrimSpace(cfg.Packs.Dir)
	if dir == "" {
		return "packs"
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}
