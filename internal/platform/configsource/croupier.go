package configsource

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
)

// croupierSource browses croupier's own ConfigVersion store:
// 目录 = namespace，文件 = key。可写：应急编辑 = 注册新版本（版本单调、
// 审批/审计复用既有链路）。
//
// Config: {"namespaces": ["runtime", ...]}（空 = 全部 namespace）。
type croupierSource struct {
	versionModel *model.ConfigVersionModel
	gameID       string
	env          string
	namespaces   map[string]struct{} // 空 = 不限制
}

var croupierVersionModel *model.ConfigVersionModel

// SetCroupierVersionModel wires the shared ConfigVersionModel (由 svc 层注入，
// 避免适配器直接持有 *gorm.DB)。
func SetCroupierVersionModel(m *model.ConfigVersionModel) {
	croupierVersionModel = m
}

func newCroupierSource(cfg map[string]interface{}, gameID, env string) (Source, error) {
	if croupierVersionModel == nil {
		return nil, fmt.Errorf("croupier source not initialized")
	}
	s := &croupierSource{
		versionModel: croupierVersionModel,
		gameID:       gameID,
		env:          env,
	}
	if list := configStrings(cfg, "namespaces"); len(list) > 0 {
		s.namespaces = map[string]struct{}{}
		for _, ns := range list {
			s.namespaces[ns] = struct{}{}
		}
	}
	return s, nil
}

func (s *croupierSource) Type() string { return "croupier" }

func (s *croupierSource) nsAllowed(ns string) bool {
	if s.namespaces == nil {
		return true
	}
	_, ok := s.namespaces[ns]
	return ok
}

func (s *croupierSource) List(ctx context.Context, dir string) ([]Entry, error) {
	dir, err := cleanPath(dir)
	if err != nil {
		return nil, err
	}
	if dir == "" {
		// 根目录 = namespace 列表（只出现实际有配置的 ns）
		latest, err := s.versionModel.ListLatest(ctx, model.ConfigListOptions{
			GameID: s.gameID, Env: s.env,
		})
		if err != nil {
			return nil, err
		}
		seen := map[string]struct{}{}
		out := []Entry{}
		for _, rec := range latest {
			ns := rec.Namespace
			if ns == "" {
				ns = model.ConfigNamespaceDefault
			}
			if !s.nsAllowed(ns) {
				continue
			}
			if _, dup := seen[ns]; dup {
				continue
			}
			seen[ns] = struct{}{}
			out = append(out, Entry{Name: ns, Path: ns, Dir: true})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out, nil
	}

	// 子目录 = namespace 下的 key 文件
	ns := dir
	if !s.nsAllowed(ns) {
		return nil, fmt.Errorf("namespace not allowed: %s", ns)
	}
	latest, err := s.versionModel.ListLatest(ctx, model.ConfigListOptions{
		GameID: s.gameID, Env: s.env,
	})
	if err != nil {
		return nil, err
	}
	out := []Entry{}
	for _, rec := range latest {
		recNS := rec.Namespace
		if recNS == "" {
			recNS = model.ConfigNamespaceDefault
		}
		if recNS != ns {
			continue
		}
		name := rec.Key
		if rec.Format != "" {
			name += "." + rec.Format
		}
		out = append(out, Entry{
			Name:    name,
			Path:    ns + "/" + name,
			Size:    int64(len(rec.Value)),
			ModTime: rec.UpdatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// splitKey splits "namespace/key[.format]" into key; format 后缀仅展示用，
// 定位仍按 namespace+key。
func (s *croupierSource) splitKey(path string) (ns, key string, err error) {
	path, err = cleanPath(path)
	if err != nil {
		return "", "", err
	}
	i := strings.Index(path, "/")
	if i <= 0 {
		return "", "", fmt.Errorf("path must be <namespace>/<key>")
	}
	ns, key = path[:i], path[i+1:]
	// 剥掉展示用的 format 后缀（key 本身不含 '.')
	if dot := strings.LastIndex(key, "."); dot > 0 {
		suffix := key[dot+1:]
		if _, isFormat := map[string]struct{}{
			"json": {}, "csv": {}, "yaml": {}, "yml": {}, "ini": {}, "xml": {},
			"lua": {}, "python": {}, "py": {},
		}[suffix]; isFormat {
			key = key[:dot]
		}
	}
	if !s.nsAllowed(ns) {
		return "", "", fmt.Errorf("namespace not allowed: %s", ns)
	}
	return ns, key, nil
}

func (s *croupierSource) Read(ctx context.Context, path string) ([]byte, error) {
	_, key, err := s.splitKey(path)
	if err != nil {
		return nil, err
	}
	rec, err := s.versionModel.FindLatestByScope(ctx, key, s.gameID, s.env)
	if err != nil {
		return nil, fmt.Errorf("config not found: %s", path)
	}
	return []byte(rec.Value), nil
}

// Write implements emergency edit: 注册新版本（ConfigVersion 单调递增，
// 发布/审计走既有链路）。
func (s *croupierSource) Write(ctx context.Context, path string, content []byte, message string) error {
	ns, key, err := s.splitKey(path)
	if err != nil {
		return err
	}
	if len(content) > 8<<20 {
		return fmt.Errorf("content too large (>8MiB)")
	}
	// 保留原 format
	format := ""
	if old, err := s.versionModel.FindLatestByScope(ctx, key, s.gameID, s.env); err == nil {
		format = old.Format
	}
	_, err = s.versionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:       key,
		Content:   string(content),
		Format:    format,
		GameID:    s.gameID,
		Env:       s.env,
		Namespace: ns,
		Message:   message,
	}, "config-explorer")
	return err
}
