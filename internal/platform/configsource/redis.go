package configsource

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisSource browses Redis keys as a directory tree (skynet 配置总线惯例：
// key 前缀即目录，如 cfg:gameplay/item → 目录 gameplay/ 下的文件 item）。
// 可写：应急编辑 = SET 回 Redis（游戏服键空间通知/轮询版本号的消费方式不变）。
//
// Config: {"addr", "password", "db", "prefix", "delimiter"}.
type redisSource struct {
	client *redis.Client
	prefix string
	sep    string
}

func newRedisSource(cfg map[string]interface{}) (Source, error) {
	addr := configString(cfg, "addr", "")
	if addr == "" {
		return nil, fmt.Errorf("redis source requires addr")
	}
	sep := configString(cfg, "delimiter", "/")
	return &redisSource{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: configString(cfg, "password", ""),
			DB:       configInt(cfg, "db", 0),
		}),
		prefix: configString(cfg, "prefix", ""),
		sep:    sep,
	}, nil
}

func (s *redisSource) Type() string { return "redis" }

// scanKeys scans keys under prefix+dir with the delimiter, collapsing into
// direct children (dirs vs files).
func (s *redisSource) List(ctx context.Context, dir string) ([]Entry, error) {
	dir, err := cleanPath(dir)
	if err != nil {
		return nil, err
	}
	base := s.prefix + dir
	if dir != "" {
		base += s.sep
	}
	pattern := base + "*"

	dirs := map[string]struct{}{}
	files := map[string]int64{}
	var cursor uint64
	for i := 0; i < 100; i++ { // 上限保护：最多 100 批 SCAN
		var keys []string
		keys, cursor, err = s.client.Scan(ctx, cursor, pattern, 500).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan: %w", err)
		}
		for _, key := range keys {
			rest := strings.TrimPrefix(key, base)
			if rest == "" || rest == key {
				continue
			}
			if i := strings.Index(rest, s.sep); i >= 0 {
				dirs[rest[:i]] = struct{}{}
			} else {
				size, _ := s.client.StrLen(ctx, key).Result()
				files[rest] = size
			}
		}
		if cursor == 0 {
			break
		}
	}

	out := make([]Entry, 0, len(dirs)+len(files))
	for name := range dirs {
		out = append(out, Entry{Name: name, Path: joinSub(dir, name), Dir: true})
	}
	for name, size := range files {
		out = append(out, Entry{Name: name, Path: joinSub(dir, name), Size: size})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *redisSource) Read(ctx context.Context, path string) ([]byte, error) {
	path, err := cleanPath(path)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("path required")
	}
	val, err := s.client.Get(ctx, s.prefix+path).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("key not found: %s", path)
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}
	return val, nil
}

// Write implements emergency edit: SET the key value back to Redis.
// 游戏服消费侧（订阅通知/轮询版本）无需任何改动。
func (s *redisSource) Write(ctx context.Context, path string, content []byte, _ string) error {
	path, err := cleanPath(path)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("path required")
	}
	if len(content) > 8<<20 {
		return fmt.Errorf("content too large (>8MiB)")
	}
	if err := s.client.Set(ctx, s.prefix+path, content, 0).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	// 键空间通知依赖 Redis notify-keyspace-events 配置；主动 publish 一个
	// 变更事件作为补充通道（skynet 惯例的版本 key 由项目自己维护则不强制）。
	_ = s.client.Publish(ctx, s.prefix+"__notify__",
		fmt.Sprintf("%s:%d", path, time.Now().Unix())).Err()
	return nil
}
