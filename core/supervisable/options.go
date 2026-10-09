package supervisable

import "time"

type config struct {
	gracefulTimeout   time.Duration
	heartbeatPath     string
	heartbeatInterval time.Duration
}

// Option Main 可选参数。
type Option func(*config)

// WithGracefulTimeout 覆盖优雅退出等待（默认 10s）：超时强退按异常退出码。
func WithGracefulTimeout(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.gracefulTimeout = d
		}
	}
}

// WithHeartbeat 开启周期心跳打点：path 为本地打点文件（监管方采样 mtime 得
// last_heartbeat_unix）；interval<=0 用默认 10s。
func WithHeartbeat(path string, interval time.Duration) Option {
	return func(c *config) {
		if path != "" {
			c.heartbeatPath = path
		}
		if interval > 0 {
			c.heartbeatInterval = interval
		}
	}
}
