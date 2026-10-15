//go:build integration

package mongo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"

	"go.mongodb.org/mongo-driver/v2/bson"
	mdriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// 活栈冒烟（4310 M3，CI capture-matrix 与本地 docker 同跑法）：单节点
// replica set（--replSet rs0 + rs.initiate()）的 mongo:7 走完整链——
// 建集合 → 空位点 Poll（从当前时间起）→ INSERT/UPDATE/DELETE 事件全收
// （ext-JSON 后像/主键 _id/无前像口径）→ Checkpoint=token 可续读 →
// 断流期间的写不丢（resumeAfter 从 token 续传）→ 白名单外集合不进流、
// 非行级事件（drop）不进流。
//
// 环境变量：TEST_MONGO_CAPTURE_DSN（缺席跳过）。要求 replica set 或
// mongos 拓扑（standalone 会被 Open 拒绝——这本身就是断言面）。

func waitMongoEvent(t *testing.T, events <-chan connector.ChangeEvent, match func(connector.ChangeEvent) bool) connector.ChangeEvent {
	t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatalf("event stream closed before match")
			}
			if match(ev) {
				return ev
			}
			t.Logf("non-matching event: %+v", ev)
		case <-deadline:
			t.Fatalf("timed out waiting for matching event")
		}
	}
}

// assertNoMongoEvent 负向窗口：白名单外/非行级事件 3s 内不得到达。
func assertNoMongoEvent(t *testing.T, events <-chan connector.ChangeEvent) {
	t.Helper()
	select {
	case ev, ok := <-events:
		if ok {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-time.After(3 * time.Second):
	}
}

func TestMongoConnectorChangeStreams(t *testing.T) {
	dsn := os.Getenv("TEST_MONGO_CAPTURE_DSN")
	if dsn == "" {
		t.Skip("TEST_MONGO_CAPTURE_DSN not configured")
	}
	const dbName = "capture_itest"

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 管理面 client（建/清集合与 DML，不经连接器）。
	admin, err := mdriver.Connect(options.Client().ApplyURI(dsn))
	if err != nil {
		t.Fatalf("admin connect: %v", err)
	}
	defer func() { _ = admin.Disconnect(context.Background()) }()
	if err := admin.Ping(ctx, nil); err != nil {
		t.Fatalf("admin ping: %v", err)
	}
	players := admin.Database(dbName).Collection("players")
	audit := admin.Database(dbName).Collection("audit")
	_ = players.Drop(ctx)
	_ = audit.Drop(ctx)
	// 集合非惰性建（Discover 扫描前须存在；audit 由首个 insert 惰性建即可）。
	if err := admin.Database(dbName).CreateCollection(ctx, "players"); err != nil {
		t.Fatalf("create players: %v", err)
	}
	t.Cleanup(func() { _ = admin.Database(dbName).Drop(context.Background()) })

	cfg := connector.SourceConfig{
		SourceID:   "mongo-itest",
		SourceType: "mongodb",
		DSNEnv:     "TEST_MONGO_CAPTURE_DSN",
		Options:    map[string]string{"database": dbName},
	}
	c := NewMongoConnector(cfg, nil)
	if err := c.Open(ctx, cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = c.Close(ctx) }()

	// Discover：database 限定扫描，_id 主键口径，能力同源。
	disc, err := c.Discover(ctx, connector.DiscoveryRequest{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	found := false
	for _, tm := range disc.Tables {
		if tm.Name == dbName+".players" {
			found = true
			if len(tm.PrimaryKey) != 1 || tm.PrimaryKey[0] != "_id" {
				t.Fatalf("players pk = %v, want [_id]", tm.PrimaryKey)
			}
		}
	}
	if !found {
		t.Fatalf("discover tables = %+v, want %s.players", disc.Tables, dbName)
	}
	if !disc.Capabilities.AfterImage || disc.Capabilities.BeforeImage || !disc.Capabilities.ResumableCheckpoint {
		t.Fatalf("capabilities = %+v", disc.Capabilities)
	}

	// 空位点 Poll：从当前时间起（collection 创建即触发不了事件——历史不回放）。
	events, err := c.Poll(ctx, connector.PollRequest{Tables: []string{dbName + ".players"}})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}

	// INSERT → 事件（ext-JSON 后像 / _id 主键 / token 位点）。
	if _, err := players.InsertOne(ctx, bson.D{{Key: "_id", Value: 1}, {Key: "name", Value: "marker-ins"}}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	ev := waitMongoEvent(t, events, func(e connector.ChangeEvent) bool {
		return e.Operation == connector.OpInsert && string(e.After["name"]) == `"marker-ins"`
	})
	if ev.Database != dbName || ev.Table != "players" {
		t.Fatalf("insert identity: %+v", ev)
	}
	if !ev.ColumnsMapped || string(ev.After["_id"]) != "1" {
		t.Fatalf("insert image: columnsMapped=%v after=%v", ev.ColumnsMapped, ev.After)
	}
	if len(ev.Key) != 1 || ev.Key[0] != "_id" {
		t.Fatalf("insert key = %v, want [_id]", ev.Key)
	}
	if ev.Position.Kind != "token" || !ev.Position.Valid() || ev.TsUnixMs <= 0 {
		t.Fatalf("insert position/ts: %+v", ev)
	}

	// UPDATE → 后像 updateLookup（Before 恒 nil，与 beforeImage=false 口径一致）。
	if _, err := players.UpdateOne(ctx, bson.D{{Key: "_id", Value: 1}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "name", Value: "marker-upd"}}}}); err != nil {
		t.Fatalf("update: %v", err)
	}
	ev = waitMongoEvent(t, events, func(e connector.ChangeEvent) bool {
		return e.Operation == connector.OpUpdate && string(e.After["name"]) == `"marker-upd"`
	})
	if ev.Before != nil {
		t.Fatalf("update before image should be nil, got %v", ev.Before)
	}

	// Checkpoint：token 位点 hex 可解码。
	cp, err := c.Checkpoint(ctx)
	if err != nil || cp.Kind != "token" || cp.Value == "" {
		t.Fatalf("checkpoint = %+v err=%v", cp, err)
	}
	if _, err := decodeToken(cp.Value); err != nil {
		t.Fatalf("checkpoint token %q undecodable: %v", cp.Value, err)
	}

	// 断流期间白名单外写入（audit）与非行级事件（drop audit）：都不进流。
	if _, err := audit.InsertOne(ctx, bson.D{{Key: "_id", Value: 1}, {Key: "e", Value: "noise"}}); err != nil {
		t.Fatalf("audit insert: %v", err)
	}
	if err := audit.Drop(ctx); err != nil {
		t.Fatalf("audit drop: %v", err)
	}
	assertNoMongoEvent(t, events)

	// 断开 → 断流期写入 players → 从 checkpoint 重开 → 断流期事件不丢。
	if err := c.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := c.Poll(ctx, connector.PollRequest{}); err == nil {
		t.Fatal("poll after close accepted")
	}
	if _, err := players.InsertOne(ctx, bson.D{{Key: "_id", Value: 2}, {Key: "name", Value: "marker-gap"}}); err != nil {
		t.Fatalf("gap insert: %v", err)
	}

	c2 := NewMongoConnector(cfg, nil)
	if err := c2.Open(ctx, cfg); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = c2.Close(ctx) }()
	events2, err := c2.Poll(ctx, connector.PollRequest{From: cp, Tables: []string{dbName + ".players"}})
	if err != nil {
		t.Fatalf("repoll: %v", err)
	}
	// token 位点严格续传（resumeAfter 不回放已交付事件），断流期写入必达。
	ev = waitMongoEvent(t, events2, func(e connector.ChangeEvent) bool {
		return e.Operation == connector.OpInsert && string(e.After["name"]) == `"marker-gap"`
	})
	if string(ev.After["_id"]) != "2" {
		t.Fatalf("gap insert _id = %v", ev.After["_id"])
	}

	// DELETE → 无像事件（单像缺失合法）。
	if _, err := players.DeleteOne(ctx, bson.D{{Key: "_id", Value: 1}}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	ev = waitMongoEvent(t, events2, func(e connector.ChangeEvent) bool {
		return e.Operation == connector.OpDelete
	})
	if ev.Before != nil || len(ev.After) != 0 || ev.Table != "players" {
		t.Fatalf("delete event: %+v", ev)
	}
	if err := c2.Close(ctx); err != nil {
		t.Fatalf("close2: %v", err)
	}
}
