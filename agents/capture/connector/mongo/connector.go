// Package mongo 实现 MongoDB CDC 源连接器（设计：docs/design/cdc-mongodb-connector.md）。
//
// 数据面：change streams（MongoDB 6.0+，go driver v2 线）。行级事件
// insert/update/replace/delete 进契约 ChangeEvent；drop/rename/invalidate
// 非行级，在流级 $match 排除（不进客户端，也不进 runner 计数——简档原文
// 「runner 计数 Skipped」修正为流级排除，见设计文档决策记录）。
package mongo

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoConnector change streams 源连接器（契约六方法）。
// 结构沿 pg 先例：pollMu 串行化生命周期方法；pump goroutine 持流，
// close-join 模式收尾（置位 closed → cancel pump ctx → 等 pump 退出 → 断开）。
type MongoConnector struct {
	cfg  connector.SourceConfig
	log  *slog.Logger
	poll sync.Mutex

	client   *mongo.Client
	cancel   context.CancelFunc
	pumpDone chan struct{}

	cpMu sync.Mutex
	cp   connector.Position

	closed atomic.Bool
}

// NewMongoConnector 构造连接器（不立即连接，Open 时建立）。
func NewMongoConnector(cfg connector.SourceConfig, log *slog.Logger) *MongoConnector {
	if log == nil {
		log = slog.Default()
	}
	return &MongoConnector{cfg: cfg, log: log}
}

// errNoImage update 的 fullDocument 查不回（行已被删/权限不可见）：双像
// 全空的 update 会被契约 Validate 拒绝，跳过不伪造。
var errNoImage = errors.New("mongo: update event has no retrievable full document")

// changeDoc change stream 事件的解码头（只取契约映射所需字段）。
type changeDoc struct {
	OperationType string `bson:"operationType"`
	Namespace     struct {
		Db   string `bson:"db"`
		Coll string `bson:"coll"`
	} `bson:"ns"`
	FullDocument bson.Raw `bson:"fullDocument,omitempty"`
	TxnNumber    *int64   `bson:"txnNumber,omitempty"`
	Lsid         *struct {
		UUID bson.Binary `bson:"uuid"`
	} `bson:"lsid,omitempty"`
	ClusterTime bson.Timestamp `bson:"clusterTime"`
	WallTime    bson.DateTime  `bson:"wallTime,omitempty"`
}

// Open 建立 client 连接并校验拓扑。DSN 只从 DSNEnv 指向的环境变量解析
// （凭据红线：不进配置文件、不落日志）；standalone mongod 无 change
// streams，fail-fast 给可执行提示（不静默降级）。
func (c *MongoConnector) Open(ctx context.Context, cfg connector.SourceConfig) error {
	c.poll.Lock()
	defer c.poll.Unlock()
	if c.closed.Load() {
		return errors.New("mongo: connector closed")
	}
	if c.client != nil {
		return errors.New("mongo: connector already open")
	}
	if cfg.DSNEnv == "" {
		return errors.New("mongo: dsnEnv missing (source config must reference a DSN environment variable)")
	}
	dsn := os.Getenv(cfg.DSNEnv)
	if dsn == "" {
		return fmt.Errorf("mongo: environment variable %s not set (mongodb DSN)", cfg.DSNEnv)
	}
	cl, err := mongo.Connect(options.Client().ApplyURI(dsn))
	if err != nil {
		return fmt.Errorf("mongo: connect via %s: %w", cfg.DSNEnv, err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := cl.Ping(pingCtx, nil); err != nil {
		_ = cl.Disconnect(context.Background())
		return fmt.Errorf("mongo: ping via %s: %w", cfg.DSNEnv, err)
	}
	// 拓扑硬前置：setName 非 null = rs 成员；msg=isdbgrid = mongos；
	// loadBalanced=true URI 是 LB 面（hello 无 setName）——三者之外视为
	// standalone，change streams 不可用。
	if !strings.Contains(dsn, "loadBalanced=true") {
		topoCtx, tcancel := context.WithTimeout(ctx, 5*time.Second)
		defer tcancel()
		var hello struct {
			SetName *string `bson:"setName"`
			Msg     string  `bson:"msg"`
		}
		if err := cl.Database("admin").RunCommand(topoCtx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello); err != nil {
			_ = cl.Disconnect(context.Background())
			return fmt.Errorf("mongo: hello via %s: %w", cfg.DSNEnv, err)
		}
		if (hello.SetName == nil || *hello.SetName == "") && hello.Msg != "isdbgrid" {
			_ = cl.Disconnect(context.Background())
			return errors.New("mongo: change streams require a replica set (or mongos): start mongod with --replSet rs0 and run rs.initiate()")
		}
	}
	c.cfg = cfg
	c.client = cl
	return nil
}

// Discover listCollections 摄取 db.collection 清单（schema-less 无列元数据，
// PrimaryKey 恒 ["_id"]）。Options["database"] 限定单库扫描，缺省扫全部
// 非系统库（admin/local/config 跳过）。
func (c *MongoConnector) Discover(ctx context.Context, req connector.DiscoveryRequest) (*connector.DiscoveryResult, error) {
	c.poll.Lock()
	defer c.poll.Unlock()
	if c.closed.Load() || c.client == nil {
		return nil, errors.New("mongo: connector not open")
	}
	var dbs []string
	if db := strings.TrimSpace(c.cfg.Options["database"]); db != "" {
		dbs = []string{db}
	} else {
		names, err := c.client.ListDatabaseNames(ctx, bson.D{})
		if err != nil {
			return nil, fmt.Errorf("mongo: list databases: %w", err)
		}
		for _, n := range names {
			if n != "admin" && n != "local" && n != "config" {
				dbs = append(dbs, n)
			}
		}
	}
	want := make(map[string]bool, len(req.Tables))
	for _, t := range req.Tables {
		want[t] = true
	}
	var tables []connector.TableMeta
	for _, db := range dbs {
		colls, err := c.client.Database(db).ListCollectionNames(ctx, bson.D{})
		if err != nil {
			return nil, fmt.Errorf("mongo: list collections %s: %w", db, err)
		}
		for _, coll := range colls {
			name := db + "." + coll
			if len(want) > 0 && !want[name] {
				continue
			}
			tables = append(tables, connector.TableMeta{Name: name, PrimaryKey: []string{"_id"}})
		}
	}
	return &connector.DiscoveryResult{Tables: tables, Capabilities: MongoCapabilities()}, nil
}

// Snapshot v1 不做（能力声明 snapshot=false，绝不静默回空流）。
func (c *MongoConnector) Snapshot(_ context.Context, _ connector.SnapshotRequest) (<-chan connector.ChangeEvent, error) {
	return nil, connector.ErrUnsupportedSnapshot
}

// Poll 打开 change stream 永久消费（From 空=从当前时间点起，起点显式记日志；
// From=token 按位续传）。行级 $in 过滤 + 白名单 ns $match 都在流级下推；
// 非行级事件（drop/rename/invalidate）不进客户端。
func (c *MongoConnector) Poll(ctx context.Context, req connector.PollRequest) (<-chan connector.ChangeEvent, error) {
	c.poll.Lock()
	defer c.poll.Unlock()
	if c.closed.Load() || c.client == nil {
		return nil, errors.New("mongo: connector not open")
	}
	cso := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	switch req.From.Kind {
	case "":
		c.log.Info("mongo poll: starting change stream at current time",
			"sourceId", c.cfg.SourceID, "tables", strings.Join(req.Tables, ","))
	case "token":
		tok, err := decodeToken(req.From.Value)
		if err != nil {
			return nil, fmt.Errorf("mongo: decode checkpoint token: %w", err)
		}
		cso = cso.SetResumeAfter(tok)
		c.log.Info("mongo poll: resuming change stream from token",
			"sourceId", c.cfg.SourceID, "token", req.From.Value)
	default:
		return nil, fmt.Errorf("mongo: position kind %q not applicable to mongodb source", req.From.Kind)
	}
	streamCtx, cancel := context.WithCancel(ctx)
	cs, err := c.client.Watch(streamCtx, buildPipeline(req.Tables), cso)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("mongo: open change stream: %w", err)
	}
	events := make(chan connector.ChangeEvent)
	done := make(chan struct{})
	c.cancel = cancel
	c.pumpDone = done
	go func() {
		defer close(done)
		defer close(events)
		defer func() { _ = cs.Close(context.Background()) }()
		for cs.Next(streamCtx) {
			ev, err := mapChangeEvent(c.cfg.SourceID, cs.Current, encodeToken(cs.ResumeToken()))
			if err != nil {
				if errors.Is(err, errNoImage) {
					c.log.Warn("mongo: skip update without retrievable full document",
						"sourceId", c.cfg.SourceID)
					continue
				}
				c.log.Error("mongo: map change event failed", "error", err)
				continue
			}
			select {
			case events <- ev:
			case <-streamCtx.Done():
				return
			}
		}
		if err := cs.Err(); err != nil && !errors.Is(err, context.Canceled) {
			c.log.Error("mongo: change stream ended", "sourceId", c.cfg.SourceID, "error", err)
		}
	}()
	return events, nil
}

// Checkpoint 最近消费的 resume token（kind=token，值=token 序列化 hex）。
// pump 并发写，锁内取；无位点时返回零 Position（消费方按无效跳过落盘）。
func (c *MongoConnector) Checkpoint(_ context.Context) (connector.Position, error) {
	c.cpMu.Lock()
	defer c.cpMu.Unlock()
	return c.cp, nil
}

// AckCheckpoint marks a consumer-delivered resume token as resumable.
func (c *MongoConnector) AckCheckpoint(_ context.Context, position connector.Position) error {
	if c.closed.Load() {
		return errors.New("mongo: connector closed")
	}
	if position.Kind != "token" || !position.Valid() {
		return errors.New("mongo: invalid token checkpoint")
	}
	c.setCheckpoint(position)
	return nil
}

// Close 断开连接（幂等）。close-join：置位 closed → cancel 流 ctx（解除
// Next 阻塞，流由 pump 退出时关闭）→ 等 pump 退出 → Disconnect。
func (c *MongoConnector) Close(_ context.Context) error {
	c.poll.Lock()
	defer c.poll.Unlock()
	if c.closed.Load() {
		return nil
	}
	c.closed.Store(true)
	if c.cancel != nil {
		c.cancel()
		select {
		case <-c.pumpDone:
		case <-time.After(5 * time.Second):
			c.log.Error("mongo: close timed out waiting for pump exit")
		}
		c.cancel = nil
		c.pumpDone = nil
	}
	if c.client != nil {
		dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dcancel()
		if err := c.client.Disconnect(dctx); err != nil {
			c.log.Warn("mongo: disconnect", "error", err)
		}
		c.client = nil
	}
	return nil
}

func (c *MongoConnector) setCheckpoint(p connector.Position) {
	c.cpMu.Lock()
	c.cp = p
	c.cpMu.Unlock()
}

// buildPipeline 流级过滤：行级操作闭集 $in + 白名单 ns $or（空白名单=全库，
// 契约 PollRequest.Tables 语义）。表名两段 "db.collection"，首段为库——
// 点号集合名只以整段 coll 匹配（含点集合名与两段白名单形态不兼容，属
// 已知边界）。
func buildPipeline(tables []string) []bson.D {
	pipeline := []bson.D{{
		{Key: "$match", Value: bson.D{{Key: "operationType",
			Value: bson.D{{Key: "$in", Value: bson.A{"insert", "update", "replace", "delete"}}}}}},
	}}
	var or bson.A
	for _, t := range tables {
		db, coll, ok := strings.Cut(t, ".")
		if !ok || db == "" || coll == "" || strings.Contains(coll, ".") {
			continue
		}
		or = append(or, bson.D{
			{Key: "ns.db", Value: db},
			{Key: "ns.coll", Value: coll},
		})
	}
	if len(or) > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.D{{Key: "$or", Value: or}}}})
	}
	return pipeline
}

// mapChangeEvent 事件映射（纯函数，单测覆盖）：insert→insert、update/replace
// →update（After=fullDocument，updateLookup 后像；Before 恒 nil——能力声明
// beforeImage=false，不伪造）、delete→delete（无像，单像缺失合法）。
func mapChangeEvent(sourceID string, raw bson.Raw, tokenHex string) (connector.ChangeEvent, error) {
	var cd changeDoc
	if err := bson.Unmarshal(raw, &cd); err != nil {
		return connector.ChangeEvent{}, fmt.Errorf("mongo: decode change event: %w", err)
	}
	var op connector.Operation
	switch cd.OperationType {
	case "insert":
		op = connector.OpInsert
	case "update", "replace":
		op = connector.OpUpdate
	case "delete":
		op = connector.OpDelete
	default:
		return connector.ChangeEvent{}, fmt.Errorf("mongo: non-row operation %q", cd.OperationType)
	}
	ts := int64(cd.WallTime)
	if ts == 0 {
		ts = int64(cd.ClusterTime.T) * 1000
	}
	ev := connector.ChangeEvent{
		SourceID:      sourceID,
		SourceType:    "mongodb",
		Database:      cd.Namespace.Db,
		Table:         cd.Namespace.Coll,
		Operation:     op,
		ColumnsMapped: true, // 像键=bson 字段名（非位置下标）
		Key:           []string{"_id"},
		Position:      connector.Position{Kind: "token", Value: tokenHex},
		TsUnixMs:      ts,
	}
	after, err := bsonToJSONMap(cd.FullDocument)
	if err != nil {
		return connector.ChangeEvent{}, err
	}
	ev.After = after
	if op == connector.OpUpdate && len(after) == 0 {
		return connector.ChangeEvent{}, errNoImage
	}
	if cd.TxnNumber != nil {
		ev.Tx = connector.TxInfo{Seq: *cd.TxnNumber}
		if cd.Lsid != nil {
			ev.Tx.ID = hex.EncodeToString(cd.Lsid.UUID.Data)
		}
	}
	if err := ev.Validate(); err != nil {
		return connector.ChangeEvent{}, err
	}
	return ev, nil
}

// bsonToJSONMap 行像转 JSON 键值。ext-JSON relaxed 形态：数值保数值、
// ObjectId→{"$oid"}、日期→{"$date"}（schema-less 源无标量保证，消费方按
// ext-JSON 形态理解——已知边界，见设计文档）。
func bsonToJSONMap(raw bson.Raw) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var d bson.D
	if err := bson.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("mongo: decode document: %w", err)
	}
	jb, err := bson.MarshalExtJSON(d, false, false)
	if err != nil {
		return nil, fmt.Errorf("mongo: encode document to ext json: %w", err)
	}
	m := make(map[string]json.RawMessage, len(d))
	if err := json.Unmarshal(jb, &m); err != nil {
		return nil, fmt.Errorf("mongo: parse ext json: %w", err)
	}
	return m, nil
}

// 位点 token 序列化：resume token 是 BSON 文档，hex 串即稳定可持久化形态
// （Position.Kind="token" 契约闭集既有值，零枚举扩展）。
func encodeToken(raw bson.Raw) string {
	if len(raw) == 0 {
		return ""
	}
	return hex.EncodeToString(raw)
}

func decodeToken(s string) (bson.Raw, error) {
	if s == "" {
		return nil, errors.New("empty resume token")
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("resume token not hex: %w", err)
	}
	return bson.Raw(b), nil
}
