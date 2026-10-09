package source

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
)

// MySQLConfig MySQL binlog CDC 连接配置。前置条件（设计 §3.3，缺一源端报错）：
// binlog_format=ROW、binlog_row_image=FULL（前像）、REPLICATION SLAVE 权限、
// server_id 与源实例及其他订阅者不冲突。
type MySQLConfig struct {
	Host     string
	Port     uint16
	User     string
	Password string
	ServerID uint32
	// Tables 订阅白名单（"库.表"，只订道具相关表，噪声与带宽双控）。
	Tables map[string]bool
}

// MySQLSource MySQL binlog 变更源（go-mysql replication，canal 同源库）。
// GTID 模式续读，位点经 XID/Query 事务边界推进，Bookmark() 随时可快照。
type MySQLSource struct {
	cfg    MySQLConfig
	log    *slog.Logger
	probe  GTIDProbe // 空位点时的起点探测（默认查 @@GLOBAL.gtid_executed）
	syncer *replication.BinlogSyncer

	gtidMu sync.Mutex
	gtid   string // 当前 GTID 集合（Bookmark 快照源）
}

// NewMySQLSource 构造 MySQL 变更源（不立即连接，Open 时建立）。
func NewMySQLSource(cfg MySQLConfig, log *slog.Logger) *MySQLSource {
	if log == nil {
		log = slog.Default()
	}
	s := &MySQLSource{cfg: cfg, log: log, probe: defaultGTIDProbe(cfg)}
	return s
}

// Open 建立 binlog 复制流。bookmark 有效则从该 GTID 集合续读（崩溃恢复），
// 否则探测源当前起点（gtid_executed——位点失效的「可得起点重扫」降级路径，
// 不全量重放历史 binlog）。返回的事件已过白名单过滤。
func (s *MySQLSource) Open(ctx context.Context, bookmark Bookmark) (<-chan ChangeEvent, error) {
	var gset mysql.GTIDSet
	if bookmark.Valid() {
		parsed, err := mysql.ParseGTIDSet("mysql", bookmark.Position)
		if err != nil {
			return nil, fmt.Errorf("source: parse mysql bookmark %q: %w", bookmark.Position, err)
		}
		gset = parsed
	} else {
		pos, err := s.probe(ctx)
		if err != nil {
			return nil, fmt.Errorf("source: probe current gtid_executed: %w", err)
		}
		parsed, err := mysql.ParseGTIDSet("mysql", pos)
		if err != nil {
			return nil, fmt.Errorf("source: parse probed gtid %q: %w", pos, err)
		}
		gset = parsed
		s.log.Info("mysql source: no valid bookmark, starting from current position", "gtid", pos)
	}
	s.setGTID(gset.String())

	syncerCfg := replication.BinlogSyncerConfig{
		ServerID: s.cfg.ServerID,
		Flavor:   "mysql",
		Host:     s.cfg.Host,
		Port:     s.cfg.Port,
		User:     s.cfg.User,
		Password: s.cfg.Password,
		// ParseTime: DATETIME/TIMESTAMP 译成 time.Time（JSON RFC3339）；
		// UseDecimal: DECIMAL 译成 decimal.Decimal 字符串语义，不走 float 丢精度。
		ParseTime:            true,
		UseDecimal:           true,
		MaxReconnectAttempts: 10,
	}
	s.syncer = replication.NewBinlogSyncer(syncerCfg)
	streamer, err := s.syncer.StartSyncGTID(gset)
	if err != nil {
		s.syncer.Close()
		s.syncer = nil
		return nil, fmt.Errorf("source: start gtid sync: %w", err)
	}

	out := make(chan ChangeEvent, 64)
	go s.pump(ctx, streamer, out)
	return out, nil
}

// Bookmark 当前位点快照（runner 周期落盘 Store）。
func (s *MySQLSource) Bookmark() Bookmark {
	s.gtidMu.Lock()
	defer s.gtidMu.Unlock()
	return Bookmark{Position: s.gtid}
}

// Close 断开复制流（幂等）。
func (s *MySQLSource) Close() error {
	if s.syncer != nil {
		s.syncer.Close()
		s.syncer = nil
	}
	return nil
}

func (s *MySQLSource) setGTID(g string) {
	s.gtidMu.Lock()
	s.gtid = g
	s.gtidMu.Unlock()
}

func (s *MySQLSource) gtidSnapshot() string {
	s.gtidMu.Lock()
	defer s.gtidMu.Unlock()
	return s.gtid
}
