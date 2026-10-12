package source

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/agents/capture/connector"

	// go-sql-driver DSN 解析（凭据红线：明文连接串只从环境变量取值，
	// 契约面只携带环境变量名 DSNEnv）。
	drivermysql "github.com/go-sql-driver/mysql"
)

// MySQLConnector 把既有 MySQLSource（Provider 形态）包进统一 SourceConnector
// 契约（todo 4306 增量三：MySQL 迁移契约——runner 多 Connector 装配与
// 三闸消费面统一切换的前置件）。
//
// 契约映射口径（无损）：
//   - 行像：MySQL binlog ROW 事件是裸值数组（位置下标），契约的
//     ColumnsMapped=false 语义正是位置下标键——Before/After 按
//     {"0","1",…} 键化，不升列名（列名映射随 Discover 摄取在契约面落地）；
//   - 位点：source.GTID → connector.Position{Kind:"gtid"}；
//   - 能力：Discover 产出与 MySQLCapabilities 同源声明（afterImage/
//     transactionMetadata/resumableCheckpoint=true；beforeImage 依赖源端
//     binlog_row_image=FULL 从严 false；snapshot/pushdown=false）。
//
// 凭据只走契约通道：DSNEnv 指向的环境变量（go-sql-driver DSN 形态，
// ParseDSN 解析出 host/port/user/password）；Options 仅承载 serverId/
// flavor/tables 等非密参数。明文密码不进契约、不落日志（红线）。
type MySQLConnector struct {
	cfg connector.SourceConfig
	log *slog.Logger
	src *MySQLSource
}

// NewMySQLConnector 构造 MySQL 契约连接器（不立即连接，Open 时解析配置）。
func NewMySQLConnector(cfg connector.SourceConfig, log *slog.Logger) *MySQLConnector {
	if log == nil {
		log = slog.Default()
	}
	return &MySQLConnector{cfg: cfg, log: log}
}

// Open 解析 DSNEnv → MySQLConfig → 构造 MySQLSource（不建连接）。
func (c *MySQLConnector) Open(_ context.Context, cfg connector.SourceConfig) error {
	if cfg.DSNEnv == "" {
		return errors.New("mysql connector: dsnEnv missing (credential channel)")
	}
	dsn := os.Getenv(cfg.DSNEnv)
	if dsn == "" {
		return fmt.Errorf("mysql connector: environment variable %s not set (mysql DSN)", cfg.DSNEnv)
	}
	dsnCfg, err := drivermysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("mysql connector: parse dsn from %s: %w", cfg.DSNEnv, err)
	}
	host, port := dsnCfg.Addr, uint16(3306)
	if i := strings.LastIndex(dsnCfg.Addr, ":"); i >= 0 {
		host = dsnCfg.Addr[:i]
		p, perr := strconv.Atoi(dsnCfg.Addr[i+1:])
		if perr != nil || p <= 0 || p > 65535 {
			return fmt.Errorf("mysql connector: bad port in dsn from %s", cfg.DSNEnv)
		}
		port = uint16(p)
	}
	sid, err := strconv.ParseUint(cfg.Options["serverId"], 10, 32)
	if err != nil || sid == 0 {
		return fmt.Errorf("mysql connector: options.serverId required (source id for binlog subscription)")
	}
	tables := map[string]bool{}
	for _, t := range strings.Split(cfg.Options["tables"], ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tables[t] = true
		}
	}
	if len(tables) == 0 {
		return errors.New("mysql connector: options.tables required (whitelist, comma-separated db.table)")
	}
	c.cfg = cfg
	c.src = NewMySQLSource(MySQLConfig{
		Host:     host,
		Port:     port,
		User:     dsnCfg.User,
		Password: dsnCfg.Passwd,
		ServerID: uint32(sid),
		Flavor:   cfg.Options["flavor"],
		Tables:   tables,
	}, c.log)
	return nil
}

// Discover information_schema 摄取列元数据与主键（ColumnsMapped 契约的
// MySQL 数据源；与能力声明同源产出）。
func (c *MySQLConnector) Discover(ctx context.Context, req connector.DiscoveryRequest) (*connector.DiscoveryResult, error) {
	if c.src == nil {
		return nil, errors.New("mysql connector: not open")
	}
	dsn := os.Getenv(c.cfg.DSNEnv)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql connector: discover open: %w", err)
	}
	defer func() { _ = db.Close() }()

	filter := map[string]bool{}
	for _, t := range req.Tables {
		filter[t] = true
	}
	const colsSQL = `SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, DATA_TYPE
FROM information_schema.COLUMNS
ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION`
	rows, err := db.QueryContext(ctx, colsSQL)
	if err != nil {
		return nil, fmt.Errorf("mysql connector: discover columns: %w", err)
	}
	type colRow struct{ schema, table, name, typ string }
	var cols []colRow
	for rows.Next() {
		var r colRow
		if err := rows.Scan(&r.schema, &r.table, &r.name, &r.typ); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("mysql connector: scan columns: %w", err)
		}
		cols = append(cols, r)
	}
	_ = rows.Close()
	const pkSQL = `SELECT tc.TABLE_SCHEMA, tc.TABLE_NAME, kcu.COLUMN_NAME
FROM information_schema.TABLE_CONSTRAINTS tc
JOIN information_schema.KEY_COLUMN_USAGE kcu
  ON tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME AND tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA
WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY'`
	pkRows, err := db.QueryContext(ctx, pkSQL)
	if err != nil {
		return nil, fmt.Errorf("mysql connector: discover primary keys: %w", err)
	}
	type pkRow struct{ schema, table, column string }
	var pks []pkRow
	for pkRows.Next() {
		var r pkRow
		if err := pkRows.Scan(&r.schema, &r.table, &r.column); err != nil {
			_ = pkRows.Close()
			return nil, fmt.Errorf("mysql connector: scan primary keys: %w", err)
		}
		pks = append(pks, r)
	}
	_ = pkRows.Close()

	pk := map[string][]string{}
	for _, r := range pks {
		pk[r.schema+"."+r.table] = append(pk[r.schema+"."+r.table], r.column)
	}
	byTable := map[string]*connector.TableMeta{}
	var order []string
	for _, r := range cols {
		full := r.schema + "." + r.table
		if len(filter) > 0 && !filter[full] {
			continue
		}
		tm, ok := byTable[full]
		if !ok {
			tm = &connector.TableMeta{Name: full}
			byTable[full] = tm
			order = append(order, full)
		}
		tm.Columns = append(tm.Columns, connector.ColumnMeta{Name: r.name, Type: r.typ})
		if keys := pk[full]; len(keys) > 0 {
			tm.PrimaryKey = keys
		}
	}
	result := &connector.DiscoveryResult{Capabilities: MySQLCapabilities()}
	for _, name := range order {
		result.Tables = append(result.Tables, *byTable[name])
	}
	return result, nil
}

// Snapshot 能力缺失显式拒绝（MySQL 快照走独立回补流程；不静默回空流）。
func (c *MySQLConnector) Snapshot(_ context.Context, _ connector.SnapshotRequest) (<-chan connector.ChangeEvent, error) {
	return nil, connector.ErrUnsupportedSnapshot
}

// Poll 从 req.From 续读（空位点探测当前起点），事件过契约桥接为统一
// ChangeEvent；req.Tables 非空时作为运行时白名单二次过滤。
func (c *MySQLConnector) Poll(ctx context.Context, req connector.PollRequest) (<-chan connector.ChangeEvent, error) {
	if c.src == nil {
		return nil, errors.New("mysql connector: not open")
	}
	events, err := c.src.Open(ctx, Bookmark{Position: req.From.Value})
	if err != nil {
		return nil, err
	}
	out := make(chan connector.ChangeEvent, 64)
	whitelist := map[string]bool{}
	for _, t := range req.Tables {
		whitelist[t] = true
	}
	go func() {
		defer close(out)
		for {
			select {
			case ev, ok := <-events:
				if !ok {
					return
				}
				if len(whitelist) > 0 && !whitelist[ev.Database+"."+ev.Table] {
					continue
				}
				ce, err := c.toContract(ev)
				if err != nil {
					c.log.Error("mysql connector: bridge event", "error", err)
					return
				}
				select {
				case out <- ce:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// toContract source.ChangeEvent → connector.ChangeEvent（无损：行像按位置
// 下标键化——MySQL binlog ROW 裸数组，契约 ColumnsMapped=false 语义）。
func (c *MySQLConnector) toContract(ev ChangeEvent) (connector.ChangeEvent, error) {
	op, err := connectorOperation(ev.Op)
	if err != nil {
		return connector.ChangeEvent{}, err
	}
	return connector.ChangeEvent{
		SourceID:      c.cfg.SourceID,
		SourceType:    "mysql",
		Database:      ev.Database,
		Table:         ev.Table,
		Operation:     op,
		Before:        positionImage(ev.BeforeJSON),
		After:         positionImage(ev.AfterJSON),
		ColumnsMapped: false, // binlog ROW 裸下标；列名映射随 Discover 摄取
		Account:       ev.Account,
		TsUnixMs:      ev.TsUnixMs,
		Position:      connector.Position{Kind: "gtid", Value: ev.GTID},
	}, nil
}

// connectorOperation source.Op → connector.Operation（闭集互转）。
func connectorOperation(op Op) (connector.Operation, error) {
	switch op {
	case OpInsert:
		return connector.OpInsert, nil
	case OpUpdate:
		return connector.OpUpdate, nil
	case OpDelete:
		return connector.OpDelete, nil
	default:
		return "", fmt.Errorf("mysql connector: unknown source op %q", op)
	}
}

// positionImage 行像 JSON 数组 → 位置下标键 map（{"0","1",…}）。空行像/
// 非法 JSON 返回 nil（与 source 层的 nil 语义对齐）。
func positionImage(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var vals []json.RawMessage
	if err := json.Unmarshal(raw, &vals); err != nil {
		return nil
	}
	m := make(map[string]json.RawMessage, len(vals))
	for i, v := range vals {
		m[strconv.Itoa(i)] = v
	}
	return m
}

// Checkpoint 当前 GTID 位点快照（runner 周期落盘数据源）。
func (c *MySQLConnector) Checkpoint(_ context.Context) (connector.Position, error) {
	if c.src == nil {
		return connector.Position{}, errors.New("mysql connector: not open")
	}
	bm := c.src.Bookmark()
	return connector.Position{Kind: "gtid", Value: bm.Position}, nil
}

// Close 断开复制流（幂等）。
func (c *MySQLConnector) Close(_ context.Context) error {
	if c.src == nil {
		return nil
	}
	return c.src.Close()
}
