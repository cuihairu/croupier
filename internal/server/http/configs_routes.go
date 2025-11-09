package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// addConfigsRoutes registers /api/configs* endpoints (MVP: JSON/CSV validation, file-backed versions).
func (s *Server) addConfigsRoutes(r *gin.Engine) {
	// ensure store path
	if strings.TrimSpace(s.configsPath) == "" {
		s.configsPath = filepath.Join("data", "configs.json")
	}
	if s.configs == nil {
		s.configs = map[string]*configEntry{}
	}

	list := r.Group("/api/configs")

	list.GET("", func(c *gin.Context) {
		if _, _, ok := s.require(c, "configs:read"); !ok {
			return
		}
		gid := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		format := strings.TrimSpace(c.Query("format"))
		idLike := strings.TrimSpace(c.Query("id_like"))
		out := []gin.H{}
		for _, e := range s.configs {
			if gid != "" && e.GameID != gid {
				continue
			}
			if env != "" && e.Env != env {
				continue
			}
			if format != "" && !strings.EqualFold(format, e.Format) {
				continue
			}
			if idLike != "" && !strings.Contains(strings.ToLower(e.ID), strings.ToLower(idLike)) {
				continue
			}
			out = append(out, gin.H{"id": e.ID, "game_id": e.GameID, "env": e.Env, "format": e.Format, "latest_version": e.Latest})
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["id"].(string) < out[j]["id"].(string) })
		c.JSON(200, gin.H{"items": out})
	})

	list.GET(":id", func(c *gin.Context) {
		if _, _, ok := s.require(c, "configs:read"); !ok {
			return
		}
		id := c.Param("id")
		gid := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		key := cfgKey(id, gid, env)
		if e := s.configs[key]; e != nil {
			var ver *configVersion
			for i := range e.Versions {
				if e.Versions[i].Version == e.Latest {
					ver = &e.Versions[i]
					break
				}
			}
			if ver == nil && len(e.Versions) > 0 {
				ver = &e.Versions[len(e.Versions)-1]
			}
			out := gin.H{"id": e.ID, "game_id": e.GameID, "env": e.Env, "format": e.Format, "version": e.Latest, "content": ""}
			if ver != nil {
				out["content"] = ver.Content
			}
			c.JSON(200, out)
			return
		}
		s.respondError(c, 404, "not_found", "config not found")
	})

	list.POST(":id/validate", func(c *gin.Context) {
		if _, _, ok := s.require(c, "configs:read", "configs:write"); !ok {
			return
		}
		id := c.Param("id")
		_ = id
		var in struct {
			Format  string `json:"format"`
			Content string `json:"content"`
		}
		if err := c.BindJSON(&in); err != nil {
			s.respondError(c, 400, "bad_request", "invalid payload")
			return
		}
		errs := []string{}
		f := strings.ToLower(strings.TrimSpace(in.Format))
		switch f {
		case "json":
			var v any
			if err := json.Unmarshal([]byte(in.Content), &v); err != nil {
				errs = append(errs, err.Error())
			}
		case "csv":
			// naive CSV validation: each line split by comma; ensure same column count
			lines := strings.Split(strings.ReplaceAll(in.Content, "\r\n", "\n"), "\n")
			cols := -1
			for _, ln := range lines {
				if strings.TrimSpace(ln) == "" {
					continue
				}
				cur := len(strings.Split(ln, ","))
				if cols < 0 {
					cols = cur
				} else if cur != cols {
					errs = append(errs, "inconsistent column count")
					break
				}
			}
		case "xml":
			var v any
			if err := xml.Unmarshal([]byte(in.Content), &v); err != nil {
				errs = append(errs, err.Error())
			}
		case "ini":
			// basic INI validator: allow [section], key=value, ;comment or #comment
			lines := strings.Split(strings.ReplaceAll(in.Content, "\r\n", "\n"), "\n")
			inSection := false
			for idx, ln := range lines {
				s := strings.TrimSpace(ln)
				if s == "" || strings.HasPrefix(s, ";") || strings.HasPrefix(s, "#") {
					continue
				}
				if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
					inSection = true
					continue
				}
				// allow key=value (in or out of section)
				if i := strings.Index(s, "="); i > 0 {
					// key and value non-empty around '='
					if strings.TrimSpace(s[:i]) == "" {
						errs = append(errs, "line "+fmtInt(idx+1)+": empty key")
					}
					continue
				}
				// not matching any acceptable line
				_ = inSection // unused fallback
				errs = append(errs, "line "+fmtInt(idx+1)+": invalid ini syntax")
			}
		case "yaml", "yml":
			// Minimal YAML check: must not contain NUL and should have balanced indentation (heuristic)
			if strings.Contains(in.Content, "\x00") {
				errs = append(errs, "contains NUL byte")
			}
			// heuristic: non-empty, non-comment line should contain ':' or start with '-' (list)
			lines := strings.Split(strings.ReplaceAll(in.Content, "\r\n", "\n"), "\n")
			for idx, ln := range lines {
				s := strings.TrimSpace(ln)
				if s == "" || strings.HasPrefix(s, "#") {
					continue
				}
				if strings.HasPrefix(s, "-") || strings.Contains(s, ":") {
					continue
				}
				// allow JSON as YAML as well
				if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
					break
				}
				errs = append(errs, "line "+fmtInt(idx+1)+": suspicious yaml (no ':' or '-')")
				if len(errs) > 5 {
					break
				}
			}
		default:
			// accept as-is for other formats (yaml/ini/xml) in MVP
		}
		c.JSON(200, gin.H{"valid": len(errs) == 0, "errors": errs})
	})

	list.POST(":id", func(c *gin.Context) {
		user, _, ok := s.require(c, "configs:write")
		if !ok {
			return
		}
		id := c.Param("id")
		var in struct {
			GameID, Env, Format, Content, Message string
			BaseVersion                           int
		}
		if err := c.BindJSON(&in); err != nil || strings.TrimSpace(in.Format) == "" {
			s.respondError(c, 400, "bad_request", "invalid payload")
			return
		}
		key := cfgKey(id, in.GameID, in.Env)
		e := s.configs[key]
		if e == nil {
			e = &configEntry{ID: id, GameID: in.GameID, Env: in.Env, Format: strings.ToLower(strings.TrimSpace(in.Format))}
		}
		// optimistic check
		if e.Latest != 0 && in.BaseVersion != 0 && e.Latest != in.BaseVersion {
			s.respondError(c, 409, "conflict", "version mismatch")
			return
		}
		// compute etag
		sum := sha256.Sum256([]byte(in.Content))
		etag := hex.EncodeToString(sum[:])
		ver := configVersion{Version: e.Latest + 1, Content: in.Content, Message: strings.TrimSpace(in.Message), Editor: user, CreatedAt: time.Now(), ETag: etag, Size: len(in.Content)}
		e.Versions = append(e.Versions, ver)
		e.Latest = ver.Version
		s.configs[key] = e
		s.saveConfigsToFile()
		if s.audit != nil {
			_ = s.audit.Log("config.update", user, key, map[string]string{"ip": c.ClientIP(), "version": fmtInt(ver.Version), "size": fmtInt(ver.Size)})
		}
		c.JSON(200, gin.H{"ok": true, "version": ver.Version, "etag": ver.ETag})
	})

	list.GET(":id/versions", func(c *gin.Context) {
		if _, _, ok := s.require(c, "configs:read"); !ok {
			return
		}
		id := c.Param("id")
		gid := c.Query("game_id")
		env := c.Query("env")
		if e := s.configs[cfgKey(id, gid, env)]; e != nil {
			outs := []gin.H{}
			for _, v := range e.Versions {
				outs = append(outs, gin.H{"version": v.Version, "message": v.Message, "editor": v.Editor, "created_at": v.CreatedAt, "size": v.Size, "etag": v.ETag})
			}
			sort.Slice(outs, func(i, j int) bool { return outs[i]["version"].(int) > outs[j]["version"].(int) })
			c.JSON(200, gin.H{"versions": outs})
			return
		}
		s.respondError(c, 404, "not_found", "config not found")
	})

	list.GET(":id/versions/:ver", func(c *gin.Context) {
		if _, _, ok := s.require(c, "configs:read"); !ok {
			return
		}
		id := c.Param("id")
		gid := c.Query("game_id")
		env := c.Query("env")
		ver := atoiSafe(c.Param("ver"))
		if e := s.configs[cfgKey(id, gid, env)]; e != nil {
			for i := range e.Versions {
				if e.Versions[i].Version == ver {
					c.JSON(200, gin.H{"version": ver, "content": e.Versions[i].Content})
					return
				}
			}
			s.respondError(c, 404, "not_found", "version not found")
			return
		}
		s.respondError(c, 404, "not_found", "config not found")
	})
}

func (s *Server) saveConfigsToFile() {
	if strings.TrimSpace(s.configsPath) == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.configsPath), 0o755)
	// marshal
	type kv struct {
		Key   string
		Entry *configEntry
	}
	arr := []kv{}
	for k, v := range s.configs {
		arr = append(arr, kv{Key: k, Entry: v})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].Key < arr[j].Key })
	m := map[string]*configEntry{}
	for _, it := range arr {
		m[it.Key] = it.Entry
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	_ = os.WriteFile(s.configsPath, b, 0o644)
}

func cfgKey(id, gid, env string) string {
	return strings.TrimSpace(gid) + "|" + strings.TrimSpace(env) + "|" + strings.TrimSpace(id)
}
func atoiSafe(s string) int { n, _ := strconv.Atoi(strings.TrimSpace(s)); return n }
func fmtInt(n int) string   { return strconv.FormatInt(int64(n), 10) }
func returnContentJSON(c *gin.Context, content string) {
	// helper to write content in same response; used in GET :id
	// we attach content after header JSON keys to avoid double writes.
	// In MVP we simply write the full JSON in one call from caller.
}
