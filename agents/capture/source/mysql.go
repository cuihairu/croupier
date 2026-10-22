package source

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
)

// MySQLConfig MySQL/MariaDB binlog CDC 连接配置。前置条件（设计 §3.3，缺一
// 源端报错）：binlog_format=ROW、binlog_row_image=FULL（前像）、REPLICATION
// SLAVE 权限、server_id 与源实例及其他订阅者不冲突。
type MySQLConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     uint16 `yaml:"port" json:"port"`
	User     string `yaml:"user" json:"user"`
	Password string `yaml:"password" json:"password"`
	ServerID uint32 `yaml:"serverId" json:"serverId"`
	// Flavor 源方言（todo 4307）："mysql" | "mariadb"，缺省 mysql。差异面：
	// GTID 变量与格式（mysql gtid_executed=UUID 集合 / mariadb
	// gtid_current_pos=domain-server-seq）+ go-mysql syncer/GTIDSet flavor；
	// ROW binlog 事件解析同库无分叉。
	Flavor string `yaml:"flavor" json:"flavor"`
	// Tables 订阅白名单（"库.表"，只订道具相关表，噪声与带宽双控）。
	Tables map[string]bool `yaml:"tables" json:"tables"`
}

// normalizeFlavor 归一方言：空值兜底 mysql；未知值报错（不静默按 mysql 跑——
// 方言错配的失败形态是连接后 GTID 乱套，越早暴露越好）。
func (c MySQLConfig) normalizeFlavor() (string, error) {
	switch c.Flavor {
	case "", "mysql":
		return "mysql", nil
	case "mariadb":
		return "mariadb", nil
	default:
		return "", fmt.Errorf("source: unknown mysql flavor %q (want \"mysql\" or \"mariadb\")", c.Flavor)
	}
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

// MySQLCapabilities 本源的能力自报（connector 简档 §3 契约类型的 MySQL
// log 订阅实现面；注册 labels captureConnectorCapabilities 的数据源，
// server 发布校验能力感知 lint 消费）。诚实口径：afterImage / resumableCheckpoint
// （位点周期落盘）具备；事务边界虽用于推进 GTID，但行事件
// 未携带事务 ID 和结束标记，transactionMetadata 必须关闭；beforeImage
// 依赖源端 binlog_row_image=FULL、agent 不可靠感知，从严声明 false；
// snapshot 未实现（历史回补走独立流程，过滤策略简档 §4）；pushdown 无
// （tables 白名单是 agent 侧代码过滤，非源端下推）。MariaDB 使用相同的保守
// 事务边界声明，GTID 方言差异不改变恢复能力结论。
func MySQLCapabilities() connector.Capabilities {
	return connector.Capabilities{
		AfterImage:          true,
		TransactionMetadata: false,
		ResumableCheckpoint: true,
	}
}

// Open 建立 binlog 复制流。bookmark 有效则从该 GTID 集合续读（崩溃恢复），
// 否则探测源当前起点（gtid_executed——位点失效的「可得起点重扫」降级路径，
// 不全量重放历史 binlog）。返回的事件已过白名单过滤。
func (s *MySQLSource) Open(ctx context.Context, bookmark Bookmark) (<-chan ChangeEvent, error) {
	flavor, err := s.cfg.normalizeFlavor()
	if err != nil {
		return nil, err
	}
	var gset mysql.GTIDSet
	if bookmark.Valid() {
		parsed, err := mysql.ParseGTIDSet(flavor, bookmark.Position)
		if err != nil {
			return nil, fmt.Errorf("source: parse %s bookmark %q: %w", flavor, bookmark.Position, err)
		}
		gset = parsed
	} else {
		pos, err := s.probe(ctx)
		if err != nil {
			return nil, fmt.Errorf("source: probe current %s position: %w", flavor, err)
		}
		parsed, err := mysql.ParseGTIDSet(flavor, pos)
		if err != nil {
			return nil, fmt.Errorf("source: parse probed %s gtid %q: %w", flavor, pos, err)
		}
		gset = parsed
		s.log.Info("mysql source: no valid bookmark, starting from current position", "flavor", flavor, "gtid", pos)
	}
	s.setGTID(gset.String())

	syncerCfg := replication.BinlogSyncerConfig{
		ServerID: s.cfg.ServerID,
		Flavor:   flavor,
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
