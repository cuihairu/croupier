package pg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

// PostgresConnector SourceConnector 的 PostgreSQL 逻辑复制实现（todo 4306
// 增量二：pglogrepl 握手 + 槽管理 + keepalive → SourceConnector 组装；
// 解码真值在 Decoder，本层只做流式接线）。
//
// 双连接形态（PG 复制协议约束）：普通连接承载 Discover/槽管理 SQL；
// START_REPLICATION 要求 replication 启动参数，Poll 时按 Open 解析的配置
// 克隆一条复制连接（RuntimeParams replication=database）。槽管理走 SQL
// 函数（pg_create_logical_replication_slot）——复制命令版
// CREATE_REPLICATION_SLOT 在普通连接上报语法错，SQL 函数版无需复制连接。
//
// 前置条件（缺一 Open/Poll 报错）：
//   - 源端 wal_level=logical（否则 START_REPLICATION 被拒）；
//   - publication 已预建（CREATE PUBLICATION … FOR TABLE …，Options.publication
//     指名）——连接器不自动建发布：pgoutput 只投递发布内表的变更，自动建
//     FOR ALL TABLES 会把全库变更卷进槽，属源端副作用，须运维显式决策；
//   - DSNEnv 指向的环境变量已设置（明文连接串不进契约/日志，凭据红线）。
//
// 位点语义：Checkpoint = 最近提交的 commit EndLSN（十进制）；从该位点续读
// 是 at-least-once（边界事务可能重放），与 MySQL 源同口径，下游幂等消费。
type PostgresConnector struct {
	cfg connector.SourceConfig
	log *slog.Logger

	pcc    *pgconn.Config // Open 时解析的连接配置（复制连接克隆源）
	conn   *pgconn.PgConn // 普通连接：Discover / 槽管理 SQL
	repl   *pgconn.PgConn // 复制连接：START_REPLICATION（Poll 懒建；pump 退出时关闭）
	closed atomic.Bool    // Close 置位；pump 收敛判定与接口拒绝共用（atomic 免锁）

	pollMu sync.Mutex // Poll 互斥（同一连接只承载一条流）

	// pump 生命周期句柄（Poll 内 pollMu 下赋值，Close 内 pollMu 下消费）：
	// pumpCancel 解除 pump 阻塞，pumpDone 等 pump 退出后 Close 才能安全收尾
	// ——pgconn 不允许 Close 与 ReceiveMessage 并发，复制连接由 pump 侧独占
	// 关闭（close-join 模式，见 Close 注释）。
	pumpCancel context.CancelFunc
	pumpDone   chan struct{}

	cpMu sync.Mutex
	cp   connector.Position // 最近提交位点（pump 写，Checkpoint 读）
}

// NewPostgresConnector 构造 PG 连接器（不立即连接，Open 时建立）。
func NewPostgresConnector(cfg connector.SourceConfig, log *slog.Logger) *PostgresConnector {
	if log == nil {
		log = slog.Default()
	}
	return &PostgresConnector{cfg: cfg, log: log}
}

// 复制标识符闭集：slot/publication 名进 SQL 字符串拼接（复制协议命令不支持
// 参数绑定），白名单校验杜绝注入面。
var replicationIdentRe = regexp.MustCompile(`^[a-z0-9_]{1,63}$`)

func validateReplicationIdent(kind, name string) error {
	if !replicationIdentRe.MatchString(name) {
		return fmt.Errorf("pg: %s %q invalid (want [a-z0-9_]{1,63})", kind, name)
	}
	return nil
}

// resolveStartLSN 起点 LSN 决策（纯逻辑，单测覆盖）：
//   - From 是合法 lsn 位点 → 从该处续读；
//   - From 空 → slot 的 confirmed_flush_lsn（存量槽）/ 建槽 consistent point
//     （新槽），由调用方查询后传入 confirmed；契约要求起点显式记日志。
func resolveStartLSN(from connector.Position, confirmed string) (pglogrepl.LSN, error) {
	switch from.Kind {
	case "":
		if confirmed == "" {
			return 0, errors.New("pg: empty start: slot confirmed position unknown")
		}
		lsn, err := pglogrepl.ParseLSN(confirmed)
		if err != nil {
			return 0, fmt.Errorf("pg: parse slot confirmed position %q: %w", confirmed, err)
		}
		return lsn, nil
	case "lsn":
		// 位点值是 Decoder 落的十进制串；ParseLSN 双形态兼容（十进制/X-Y hex）。
		lsn, err := ParseLSN(from.Value)
		if err != nil {
			return 0, fmt.Errorf("pg: parse checkpoint lsn %q: %w", from.Value, err)
		}
		return pglogrepl.LSN(lsn), nil
	default:
		return 0, fmt.Errorf("pg: position kind %q not applicable to postgres source", from.Kind)
	}
}

// Open 建立复制连接（识别系统不在此步——START_REPLICATION 时服务端才要求
// 复制形态；普通连接即可完成 Discover/槽管理）。DSN 从 DSNEnv 指向的环境
// 变量解析；错误消息只提环境变量名，不回显取值。
func (c *PostgresConnector) Open(_ context.Context, cfg connector.SourceConfig) error {
	c.pollMu.Lock()
	defer c.pollMu.Unlock()
	if c.closed.Load() {
		return errors.New("pg: connector closed")
	}
	if c.conn != nil {
		return errors.New("pg: connector already open")
	}
	if cfg.DSNEnv == "" {
		return errors.New("pg: dsnEnv missing (source config must reference a DSN environment variable)")
	}
	dsn := os.Getenv(cfg.DSNEnv)
	if dsn == "" {
		return fmt.Errorf("pg: environment variable %s not set (postgres DSN)", cfg.DSNEnv)
	}
	pcc, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("pg: parse dsn from %s: %w", cfg.DSNEnv, err)
	}
	conn, err := pgconn.ConnectConfig(context.Background(), pcc)
	if err != nil {
		return fmt.Errorf("pg: connect via %s: %w", cfg.DSNEnv, err)
	}
	c.cfg = cfg
	c.pcc = pcc
	c.conn = conn
	return nil
}

// Discover information_schema 摄取表/列/PK 元数据（ColumnsMapped 契约的
// 数据源）。Tables 过滤用 "schema.table" 全名；能力声明与元数据同源产出。
func (c *PostgresConnector) Discover(ctx context.Context, req connector.DiscoveryRequest) (*connector.DiscoveryResult, error) {
	c.pollMu.Lock()
	defer c.pollMu.Unlock()
	if c.closed.Load() || c.conn == nil {
		return nil, errors.New("pg: connector not open")
	}
	filter := make(map[string]bool, len(req.Tables))
	for _, t := range req.Tables {
		filter[t] = true
	}
	const colsSQL = `SELECT table_schema, table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema NOT IN ('pg_catalog','information_schema')
ORDER BY table_schema, table_name, ordinal_position`
	colRows, err := c.conn.Exec(ctx, colsSQL).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("pg: discover columns: %w", err)
	}
	const pkSQL = `SELECT tc.table_schema, tc.table_name, kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
WHERE tc.constraint_type = 'PRIMARY KEY'`
	pkRows, err := c.conn.Exec(ctx, pkSQL).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("pg: discover primary keys: %w", err)
	}

	pk := map[string][]string{}
	for _, r := range pkRows {
		for _, row := range r.Rows {
			if len(row) != 3 {
				continue
			}
			full := string(row[0]) + "." + string(row[1])
			pk[full] = append(pk[full], string(row[2]))
		}
	}
	byTable := map[string]*connector.TableMeta{}
	var order []string
	for _, r := range colRows {
		for _, row := range r.Rows {
			if len(row) != 4 {
				continue
			}
			full := string(row[0]) + "." + string(row[1])
			if len(filter) > 0 && !filter[full] {
				continue
			}
			tm, ok := byTable[full]
			if !ok {
				tm = &connector.TableMeta{Name: full}
				byTable[full] = tm
				order = append(order, full)
			}
			tm.Columns = append(tm.Columns, connector.ColumnMeta{
				Name: string(row[2]),
				Type: string(row[3]),
			})
			if keys, ok := pk[full]; ok {
				tm.PrimaryKey = keys
			}
		}
	}
	result := &connector.DiscoveryResult{Capabilities: PostgresCapabilities()}
	for _, name := range order {
		result.Tables = append(result.Tables, *byTable[name])
	}
	return result, nil
}

// Snapshot 能力缺失显式拒绝（契约要求，不静默回空流）。
func (c *PostgresConnector) Snapshot(_ context.Context, _ connector.SnapshotRequest) (<-chan connector.ChangeEvent, error) {
	return nil, connector.ErrUnsupportedSnapshot
}

// Poll 增量变更流：复制连接（懒建，DSN 克隆加 replication=database 启动
// 参数）→ 确保逻辑槽存在 → START_REPLICATION（pgoutput v1，指名
// publication）→ 接收循环（XLogData → Decoder → 白名单过滤 → 事件通道；
// keepalive 与周期 standby 状态上报防服务端掐空闲连接）。ctx 取消即流关闭。
func (c *PostgresConnector) Poll(ctx context.Context, req connector.PollRequest) (<-chan connector.ChangeEvent, error) {
	c.pollMu.Lock()
	defer c.pollMu.Unlock()
	if c.closed.Load() || c.conn == nil {
		return nil, errors.New("pg: connector not open")
	}
	slotName, pub, err := c.replicationOptions()
	if err != nil {
		return nil, err
	}
	repl, err := c.ensureReplConn()
	if err != nil {
		return nil, err
	}
	// 启动序列失败即弃连：START_REPLICATION 前的任何错误（起点解析、槽被
	// 占 55006 等）都必须关掉 repl 并复位缓存——失败连接以 startup walsender
	// 形态挂在 PG 侧占 max_wal_senders 配额，重开循环逐轮泄漏一个直至耗尽
	// （55006 级联全员无法重连）。本调用持有 pollMu，可直接改缓存。
	abort := func(cause error) error {
		if c.repl == repl {
			c.repl = nil
		}
		_ = repl.Close(context.Background())
		return cause
	}
	startLSN, err := c.resolveStart(ctx, slotName, req.From)
	if err != nil {
		return nil, abort(err)
	}
	c.log.Info("postgres poll: starting replication",
		"sourceId", c.cfg.SourceID, "slot", slotName, "publication", pub,
		"startLsn", pglogrepl.LSN(startLSN).String())

	err = pglogrepl.StartReplication(ctx, repl, slotName, startLSN, pglogrepl.StartReplicationOptions{
		// 插件参数每项是完整 "name 'value'" 对（repl_gram 语法：选项名与值
		// 空格分隔、值单引号；逗号只分隔不同选项）。pub 已过白名单校验，
		// 无引号注入面。
		PluginArgs: []string{"proto_version '1'", "publication_names '" + pub + "'"},
	})
	if err != nil {
		return nil, abort(fmt.Errorf("pg: start replication slot %s: %w", slotName, err))
	}

	out := make(chan connector.ChangeEvent, 64)
	whitelist := make(map[string]bool, len(req.Tables))
	for _, t := range req.Tables {
		whitelist[t] = true
	}
	// pump 包装：复制连接的关闭权归 pump 侧（pgconn.Close 与 ReceiveMessage
	// 并发会在 pgconn 内部数据上竞态）——pump 退出后本包装统一关 repl，Close
	// 只取消 + join，不直接动 repl。
	pumpCtx, cancel := context.WithCancel(ctx)
	c.pumpCancel = cancel
	c.pumpDone = make(chan struct{})
	go func() {
		defer close(c.pumpDone)
		defer func() { _ = repl.Close(context.Background()) }()
		c.pump(pumpCtx, repl, out, whitelist)
	}()
	return out, nil
}

// publicationPrematureHint 识别「槽位重启点早于发布创建」的流中报错并给处置
// 提示。pgoutput 的 change 回调按**槽位起点的历史目录快照**解析发布归属：槽
// 位起点（confirmed_flush/restart_lsn）停在发布创建之前的 WAL 上时，
// START_REPLICATION 仍能成功（启动校验只读当前目录），但解码到该处变更立即
// 报 publication "X" does not exist，且每次重连都重读同一旧位点 = 永久重连
// 死循环。识别面：message 形如 publication "name" does not exist。命中返回
// 处置提示（推进槽位越过该处 / 重建槽位）。
func publicationPrematureHint(message string) (string, bool) {
	const marker = "publication \""
	if !strings.HasPrefix(message, marker) {
		return "", false
	}
	rest := message[len(marker):]
	if !strings.HasSuffix(rest, `" does not exist`) {
		return "", false
	}
	name := rest[:len(rest)-len(`" does not exist`)]
	if name == "" || strings.ContainsAny(name, "\n\r\t\"") {
		return "", false
	}
	return fmt.Sprintf(
		"slot restart position predates publication %q creation; "+
			"decode cannot see it in the historic catalog snapshot. "+
			"Remediate once: SELECT pg_replication_slot_advance('<slot>', pg_current_wal_lsn()) "+
			"or drop and recreate the slot (position advances past the stale change, then the stream resumes)",
		name), true
}

// ensureReplConn 复制连接（幂等）：Open 解析的配置克隆 + replication=database
// 启动参数（逻辑复制必须挂库；replication=true 是物理复制语义，不适用）。
func (c *PostgresConnector) ensureReplConn() (*pgconn.PgConn, error) {
	if c.repl != nil {
		return c.repl, nil
	}
	rc := *c.pcc
	rc.RuntimeParams = make(map[string]string, len(c.pcc.RuntimeParams)+1)
	for k, v := range c.pcc.RuntimeParams {
		rc.RuntimeParams[k] = v
	}
	rc.RuntimeParams["replication"] = "database"
	conn, err := pgconn.ConnectConfig(context.Background(), &rc)
	if err != nil {
		return nil, fmt.Errorf("pg: connect replication connection: %w", err)
	}
	c.repl = conn
	return conn, nil
}

// replicationOptions 槽/发布名取值与校验（Options 契约面：slotName、
// publication 必填）。
func (c *PostgresConnector) replicationOptions() (slot, pub string, err error) {
	slot = c.cfg.Options["slotName"]
	pub = c.cfg.Options["publication"]
	if slot == "" {
		return "", "", errors.New("pg: options.slotName required (logical slot name)")
	}
	if pub == "" {
		return "", "", errors.New("pg: options.publication required (pre-created publication name)")
	}
	if err := validateReplicationIdent("slot name", slot); err != nil {
		return "", "", err
	}
	if err := validateReplicationIdent("publication name", pub); err != nil {
		return "", "", err
	}
	return slot, pub, nil
}

// resolveStart 起点 LSN：From 位点优先；空 From 查槽的 confirmed_flush_lsn
// （槽不存在则先建槽，用建槽返回的 consistent point）。
func (c *PostgresConnector) resolveStart(ctx context.Context, slot string, from connector.Position) (pglogrepl.LSN, error) {
	if from.Kind != "" || from.Value != "" {
		return resolveStartLSN(from, "")
	}
	confirmed, err := c.slotConfirmedLSN(ctx, slot)
	if err != nil {
		return 0, err
	}
	lsn, err := resolveStartLSN(connector.Position{}, confirmed)
	if err != nil {
		return 0, err
	}
	return lsn, nil
}

// slotConfirmedLSN 查询槽位点；槽不存在时经 SQL 函数创建（pgoutput，持久
// 槽——位点恢复语义依赖槽在服务端保留未消费 WAL）。
func (c *PostgresConnector) slotConfirmedLSN(ctx context.Context, slot string) (string, error) {
	rows, err := c.conn.Exec(ctx,
		`SELECT confirmed_flush_lsn FROM pg_replication_slots WHERE slot_name = '`+slot+`'`).ReadAll()
	if err != nil {
		return "", fmt.Errorf("pg: query slot %s: %w", slot, err)
	}
	if len(rows) > 0 && len(rows[0].Rows) > 0 {
		return string(rows[0].Rows[0][0]), nil
	}
	createRows, err := c.conn.Exec(ctx,
		`SELECT lsn FROM pg_create_logical_replication_slot('`+slot+`', 'pgoutput')`).ReadAll()
	if err != nil {
		return "", fmt.Errorf("pg: create slot %s: %w", slot, err)
	}
	if len(createRows) == 0 || len(createRows[0].Rows) == 0 {
		return "", fmt.Errorf("pg: create slot %s: empty result", slot)
	}
	lsn := string(createRows[0].Rows[0][0])
	c.log.Info("postgres poll: created logical slot", "slot", slot, "consistentPoint", lsn)
	return lsn, nil
}

// pump 接收循环（Poll 独占复制连接；ctx 取消/流错误 → 事件通道关闭收敛）。
// standby 心跳采用 pglogrepl 官方 demo 形态：ReceiveMessage 挂 10s 截止，
// 超时即发一次状态上报（服务端 wal_sender_timeout 默认 60s，10s 上报有
// 4 倍裕量）。
func (c *PostgresConnector) pump(ctx context.Context, repl *pgconn.PgConn, out chan<- connector.ChangeEvent, whitelist map[string]bool) {
	defer close(out)
	dec := NewDecoder(c.cfg.SourceID)
	clientXLogPos := pglogrepl.LSN(0)
	nextStandbyDeadline := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(nextStandbyDeadline) {
			err := pglogrepl.SendStandbyStatusUpdate(ctx, repl, pglogrepl.StandbyStatusUpdate{
				WALWritePosition: clientXLogPos,
			})
			if err != nil {
				c.log.Error("postgres poll: standby status update failed", "error", err)
				return
			}
			nextStandbyDeadline = time.Now().Add(10 * time.Second)
		}
		recvCtx, cancel := context.WithDeadline(ctx, nextStandbyDeadline)
		rawMsg, err := repl.ReceiveMessage(recvCtx)
		cancel()
		if err != nil {
			if ctx.Err() != nil || c.closed.Load() {
				return // 上层取消 / Close 断流：正常收敛，不计错误
			}
			if pgconn.Timeout(err) {
				continue // 心跳截止到点，回循环发状态
			}
			c.log.Error("postgres poll: receive failed", "error", err)
			return
		}
		if errMsg, ok := rawMsg.(*pgproto3.ErrorResponse); ok {
			args := []any{"severity", errMsg.Severity, "message", errMsg.Message}
			if hint, ok := publicationPrematureHint(errMsg.Message); ok {
				// 流建立后才到的 ErrorResponse 是 change 回调里的历史快照
				// 解码失败，与启动校验是两条路；给运维一条可执行的处置提示。
				args = append(args, "hint", hint)
			}
			c.log.Error("postgres poll: server error response", args...)
			return
		}
		msg, ok := rawMsg.(*pgproto3.CopyData)
		if !ok {
			continue
		}
		switch msg.Data[0] {
		case pglogrepl.PrimaryKeepaliveMessageByteID:
			pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
			if err != nil {
				c.log.Error("postgres poll: parse keepalive", "error", err)
				return
			}
			if pkm.ServerWALEnd > clientXLogPos {
				clientXLogPos = pkm.ServerWALEnd
			}
			if pkm.ReplyRequested {
				nextStandbyDeadline = time.Time{} // 立即回状态上报
			}
		case pglogrepl.XLogDataByteID:
			xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
			if err != nil {
				c.log.Error("postgres poll: parse xlog data", "error", err)
				return
			}
			events, err := dec.Feed(xld.WALData)
			if err != nil {
				c.log.Error("postgres poll: decode", "error", err)
				return
			}
			if xld.WALStart > clientXLogPos {
				clientXLogPos = xld.WALStart
			}
			for i := range events {
				if len(whitelist) > 0 && !whitelist[events[i].QualifiedTable()] {
					continue
				}
				select {
				case out <- events[i]:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (c *PostgresConnector) storeCheckpoint(p connector.Position) {
	c.cpMu.Lock()
	c.cp = p
	c.cpMu.Unlock()
}

// Checkpoint 最近提交位点（pump 循环并发读写，锁内取）。
func (c *PostgresConnector) Checkpoint(_ context.Context) (connector.Position, error) {
	c.cpMu.Lock()
	defer c.cpMu.Unlock()
	return c.cp, nil
}

// AckCheckpoint marks a consumer-delivered commit position as resumable.
func (c *PostgresConnector) AckCheckpoint(_ context.Context, position connector.Position) error {
	if c.closed.Load() {
		return errors.New("pg: connector closed")
	}
	if position.Kind != "lsn" || !position.Valid() {
		return errors.New("pg: invalid lsn checkpoint")
	}
	c.storeCheckpoint(position)
	return nil
}

// Close 断开连接（幂等；槽为持久槽，不随连接删除——位点恢复依赖）。
//
// close-join 模式：置位 closed → 取消 pumpCtx（解除 ReceiveMessage 阻塞）→
// 等 pump 退出（repl 由 pump 包装 goroutine 关闭）→ 最后关普通连接。不得在
// 此直接 Close repl：与 pump 在途的 ReceiveMessage 并发关同一 pgconn 是
// 数据竞态（-race 实证），且 ReceiveMessage 只能靠 ctx 取消或连接对端断开
// 解除阻塞，前者归 pumpCancel，后者归 pump 自己的 repl.Close。
func (c *PostgresConnector) Close(_ context.Context) error {
	c.pollMu.Lock()
	defer c.pollMu.Unlock()
	if c.closed.Load() {
		return nil
	}
	c.closed.Store(true)
	if c.pumpCancel != nil {
		c.pumpCancel()
		select {
		case <-c.pumpDone:
		case <-time.After(5 * time.Second):
			// 防御：pump 极端滞留时不无限挂死 Close；repl 留给 pump 退出时关。
			c.log.Error("pg: close timed out waiting for pump exit")
		}
		c.pumpCancel = nil
		c.pumpDone = nil
		// 流已由 pump 包装关闭，这里只复位缓存（不动已关连接）。
		c.repl = nil
	} else if c.repl != nil {
		// pump 从未启动（Poll 在启动序列失败后遗留）的兜底：无人负责关闭，
		// Close 收尾。持有 pollMu，与 Poll 的弃连路径互斥，无双关竞态。
		_ = c.repl.Close(context.Background())
		c.repl = nil
	}
	var err error
	if c.conn != nil {
		err = c.conn.Close(context.Background())
		c.conn = nil
	}
	return err
}
