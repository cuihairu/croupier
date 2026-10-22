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
	"time"

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
	cfg     connector.SourceConfig
	log     *slog.Logger
	bridge  *ProviderConnector // Open 后组装的 Provider→契约桥接（流/位点复用同一份映射）
	execSQL func(ctx context.Context, query string) (sql.Result, error)
}

// NewMySQLConnector 构造 MySQL 契约连接器（不立即连接，Open 时解析配置）。
func NewMySQLConnector(cfg connector.SourceConfig, log *slog.Logger) *MySQLConnector {
	if log == nil {
		log = slog.Default()
	}
	return &MySQLConnector{cfg: cfg, log: log, execSQL: defaultConnectorSQLExecutor(cfg.DSNEnv)}
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
	mysqlConfig := MySQLConfig{
		Host:     host,
		Port:     port,
		User:     dsnCfg.User,
		Password: dsnCfg.Passwd,
		ServerID: uint32(sid),
		Flavor:   cfg.Options["flavor"],
		Tables:   tables,
	}
	if c.execSQL != nil {
		mysqlConfig.ExecSQL = c.execSQL
	}
	c.bridge = NewProviderConnector(NewMySQLSource(mysqlConfig, c.log), "mysql", c.log)
	return nil
}

// Discover information_schema 摄取列元数据与主键（ColumnsMapped 契约的
// MySQL 数据源；与能力声明同源产出）。
func (c *MySQLConnector) Discover(ctx context.Context, req connector.DiscoveryRequest) (*connector.DiscoveryResult, error) {
	if c.bridge == nil {
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

// Snapshot remains explicitly unsupported because the public Connector
// contract has no consistency probe. SnapshotWithProbe is the guarded source
// recovery extension and must not be exposed as a generic snapshot source.
func (c *MySQLConnector) Snapshot(_ context.Context, _ connector.SnapshotRequest) (<-chan connector.ChangeEvent, error) {
	return nil, connector.ErrUnsupportedSnapshot
}

// SnapshotWithProbe captures a consistent row snapshot. It requires MySQL 8
// backup-lock support. GTID is read on the same locked transaction so the
// returned position cannot advance past rows observed by the snapshot; the
// probe callback stays available for a caller that needs its own same-session
// position source.
func (c *MySQLConnector) SnapshotWithProbe(ctx context.Context, req connector.SnapshotRequest, probe GTIDProbe) (<-chan connector.ChangeEvent, error) {
	if c.bridge == nil {
		return nil, errors.New("mysql connector: not open")
	}
	if probe == nil {
		return nil, errors.New("mysql connector: snapshot consistency probe is required")
	}
	for _, table := range req.Tables {
		db, name, ok := strings.Cut(table, ".")
		if !ok || db == "" || name == "" {
			return nil, fmt.Errorf("mysql connector: snapshot table %q must be database.table", table)
		}
	}
	dsn := os.Getenv(c.cfg.DSNEnv)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql connector: snapshot open: %w", err)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql connector: snapshot begin: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "LOCK INSTANCE FOR BACKUP"); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return nil, fmt.Errorf("mysql connector: snapshot requires backup lock: %w", err)
	}
	position, err := probe(ctx)
	if err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return nil, fmt.Errorf("mysql connector: snapshot probe: %w", err)
	}
	// Unlock only after the GTID set is captured. A callback that needs a
	// separate writer for deterministic fault injection runs before this
	// same-transaction probe.
	if c.execSQL != nil {
		if _, err := c.execSQL(ctx, "UNLOCK INSTANCE"); err != nil {
			_ = tx.Rollback()
			_ = db.Close()
			return nil, fmt.Errorf("mysql connector: snapshot consistency injection: %w", err)
		}
	}
	discovery, err := c.Discover(ctx, connector.DiscoveryRequest{Tables: req.Tables})
	if err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return nil, err
	}
	out := make(chan connector.ChangeEvent)
	go func() {
		defer close(out)
		defer func() { _ = tx.Rollback() }()
		defer func() { _ = db.Close() }()
		for _, table := range discovery.Tables {
			if ctx.Err() != nil {
				return
			}
			qualifiedName, err := quoteSnapshotTable(table.Name)
			if err != nil {
				c.log.Error("mysql snapshot table invalid", "table", table.Name, "error", err)
				return
			}
			rows, err := tx.QueryContext(ctx, "SELECT * FROM "+qualifiedName)
			if err != nil {
				c.log.Error("mysql snapshot scan failed", "table", table.Name, "error", err)
				return
			}
			columns, err := rows.Columns()
			if err != nil {
				_ = rows.Close()
				c.log.Error("mysql snapshot columns failed", "table", table.Name, "error", err)
				return
			}
			for rows.Next() {
				values := make([]any, len(columns))
				scans := make([]any, len(columns))
				for i := range values {
					scans[i] = &values[i]
				}
				if err := rows.Scan(scans...); err != nil {
					_ = rows.Close()
					c.log.Error("mysql snapshot row failed", "table", table.Name, "error", err)
					return
				}
				image, err := snapshotImage(values)
				if err != nil {
					_ = rows.Close()
					c.log.Error("mysql snapshot encode failed", "table", table.Name, "error", err)
					return
				}
				event := connector.ChangeEvent{
					SourceID: c.cfg.SourceID, SourceType: "mysql", Database: strings.SplitN(table.Name, ".", 2)[0],
					Table: strings.SplitN(table.Name, ".", 2)[1], Operation: connector.OpInsert,
					After: image, ColumnsMapped: true, Key: table.PrimaryKey,
					Position: connector.Position{Kind: "gtid", Value: position},
					Tx:       connector.TxInfo{ID: "snapshot:" + position, Seq: 1, End: true}, TsUnixMs: time.Now().UnixMilli(),
				}
				select {
				case out <- event:
				case <-ctx.Done():
					return
				}
			}
			if err := rows.Err(); err != nil {
				c.log.Error("mysql snapshot rows failed", "table", table.Name, "error", err)
				return
			}
			_ = rows.Close()
		}
	}()
	return out, nil
}

func quoteSnapshotIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func quoteSnapshotTable(value string) (string, error) {
	schema, table, ok := strings.Cut(value, ".")
	if !ok || schema == "" || table == "" || strings.Contains(table, ".") {
		return "", fmt.Errorf("mysql connector: invalid snapshot table %q", value)
	}
	return quoteSnapshotIdentifier(schema) + "." + quoteSnapshotIdentifier(table), nil
}

func snapshotImage(values []any) (map[string]json.RawMessage, error) {
	image := make(map[string]json.RawMessage, len(values))
	for i, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("column %d: %w", i, err)
		}
		image[strconv.Itoa(i)] = encoded
	}
	return image, nil
}

// Poll 从 req.From 续读（空位点探测当前起点），事件过契约桥接为统一
// ChangeEvent；req.Tables 非空时作为运行时白名单二次过滤。流/桥接复用
// ProviderConnector（Provider 形态在此边界收口，映射口径单一实现）。
func (c *MySQLConnector) Poll(ctx context.Context, req connector.PollRequest) (<-chan connector.ChangeEvent, error) {
	if c.bridge == nil {
		return nil, errors.New("mysql connector: not open")
	}
	return c.bridge.Poll(ctx, req)
}

// Checkpoint 当前 GTID 位点快照（runner 周期落盘数据源）。
func (c *MySQLConnector) Checkpoint(ctx context.Context) (connector.Position, error) {
	if c.bridge == nil {
		return connector.Position{}, errors.New("mysql connector: not open")
	}
	return c.bridge.Checkpoint(ctx)
}

// AckCheckpoint runner 消费完成后确认 GTID 位点。
func (c *MySQLConnector) AckCheckpoint(ctx context.Context, position connector.Position) error {
	if c.bridge == nil {
		return errors.New("mysql connector: not open")
	}
	return c.bridge.AckCheckpoint(ctx, position)
}

// Close 断开复制流（幂等）。
func (c *MySQLConnector) Close(ctx context.Context) error {
	if c.bridge == nil {
		return nil
	}
	return c.bridge.Close(ctx)
}
