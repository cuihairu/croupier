package mongo

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// 能力声明闭集（设计 §4）：afterImage/transactionMetadata/resumableCheckpoint
// 开，beforeImage/snapshot/pushdown/schemaHistory 关。
func TestMongoCapabilities(t *testing.T) {
	caps := MongoCapabilities()
	if !caps.AfterImage || caps.TransactionMetadata || !caps.ResumableCheckpoint {
		t.Fatalf("capabilities = %+v (want afterImage/resumable on, transactionMetadata off)", caps)
	}
	if caps.BeforeImage || caps.Snapshot || caps.Pushdown || caps.SchemaHistory {
		t.Fatalf("capabilities = %+v (want beforeImage/snapshot/pushdown/schemaHistory off)", caps)
	}
	if err := caps.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	// 能力感知守卫消费面：依赖前像/快照的规则必须被拒。
	if err := caps.Requires(connector.CapBeforeImage); err == nil {
		t.Fatal("requires beforeImage should fail (declared false)")
	}
	if err := caps.Requires(connector.CapAfterImage); err != nil {
		t.Fatalf("requires afterImage: %v", err)
	}
}

// 契约守卫：未 open 前 Poll/Discover 拒绝、Snapshot 显式 ErrUnsupportedSnapshot
// （不静默回空流）、Close 幂等。
func TestMongoContractGuards(t *testing.T) {
	cn := NewMongoConnector(connector.SourceConfig{SourceType: "mongodb"}, nil)
	if _, err := cn.Snapshot(t.Context(), connector.SnapshotRequest{}); !errors.Is(err, connector.ErrUnsupportedSnapshot) {
		t.Fatalf("snapshot err = %v (want ErrUnsupportedSnapshot)", err)
	}
	if _, err := cn.Poll(t.Context(), connector.PollRequest{}); err == nil {
		t.Fatal("poll before open should fail")
	}
	if _, err := cn.Discover(t.Context(), connector.DiscoveryRequest{}); err == nil {
		t.Fatal("discover before open should fail")
	}
	if err := cn.Close(t.Context()); err != nil {
		t.Fatalf("close = %v", err)
	}
	if err := cn.Close(t.Context()); err != nil {
		t.Fatalf("close twice (idempotent) = %v", err)
	}
}

// DSNEnv 未设/环境变量为空：凭据红线（错误只提变量名，不回显值）。
func TestMongoOpenMissingEnv(t *testing.T) {
	cn := NewMongoConnector(connector.SourceConfig{SourceType: "mongodb", DSNEnv: "TEST_MONGO_DSN_UNSET"}, nil)
	if err := cn.Open(t.Context(), connector.SourceConfig{SourceType: "mongodb", DSNEnv: "TEST_MONGO_DSN_UNSET"}); err == nil {
		t.Fatal("open with unset env should fail")
	}
	cn2 := NewMongoConnector(connector.SourceConfig{SourceType: "mongodb"}, nil)
	err := cn2.Open(t.Context(), connector.SourceConfig{SourceType: "mongodb"})
	if err == nil || !strings.Contains(err.Error(), "dsnEnv") {
		t.Fatalf("open without dsnEnv err = %v", err)
	}
}

func TestMongoAckCheckpoint(t *testing.T) {
	want := connector.Position{Kind: "token", Value: "ack-token"}
	cn := NewMongoConnector(connector.SourceConfig{SourceType: "mongodb"}, nil)
	if err := cn.AckCheckpoint(t.Context(), want); err != nil {
		t.Fatalf("ack checkpoint: %v", err)
	}
	got, err := cn.Checkpoint(t.Context())
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if got != want {
		t.Fatalf("checkpoint = %+v, want %+v", got, want)
	}
}

// 非行级操作（drop/rename/invalidate/未知）映射拒绝——流级 $in 已排除，
// 这是第二道防线。
func TestMapChangeEventNonRowRejected(t *testing.T) {
	for _, op := range []string{"drop", "rename", "invalidate", "dropDatabase", "other"} {
		raw, err := bson.Marshal(bson.D{
			{Key: "operationType", Value: op},
			{Key: "ns", Value: bson.D{{Key: "db", Value: "g"}, {Key: "coll", Value: "c"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := mapChangeEvent("s", raw, "ab"); err == nil {
			t.Fatalf("operationType %q should be rejected", op)
		}
	}
}

// 事件映射四操作面：insert 全量后像 / update updateLookup 后像+无像跳过 /
// delete 无像合法 / replace 并入 update。
func TestMapChangeEventOperations(t *testing.T) {
	doc := bson.D{
		{Key: "_id", Value: bson.Binary{Subtype: 4, Data: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}}},
		{Key: "name", Value: "alice"},
		{Key: "level", Value: int32(3)},
	}
	mk := func(op string, full any, txn *int64) bson.Raw {
		d := bson.D{
			{Key: "operationType", Value: op},
			{Key: "ns", Value: bson.D{{Key: "db", Value: "game"}, {Key: "coll", Value: "players"}}},
			{Key: "clusterTime", Value: bson.Timestamp{T: 1760000000}},
			{Key: "wallTime", Value: bson.NewDateTimeFromTime(time.UnixMilli(1760000000123))},
		}
		if full != nil {
			d = append(d, bson.E{Key: "fullDocument", Value: full})
		}
		if txn != nil {
			d = append(d, bson.E{Key: "txnNumber", Value: *txn})
			d = append(d, bson.E{Key: "lsid", Value: bson.D{{Key: "uuid",
				Value: bson.Binary{Subtype: 4, Data: []byte{9, 9, 9}}}}})
		}
		raw, err := bson.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	t.Run("insert", func(t *testing.T) {
		ev, err := mapChangeEvent("src1", mk("insert", doc, nil), "deadbeef")
		if err != nil {
			t.Fatal(err)
		}
		if ev.SourceType != "mongodb" || ev.Database != "game" || ev.Table != "players" {
			t.Fatalf("ev = %+v", ev)
		}
		if ev.Operation != connector.OpInsert || ev.Before != nil {
			t.Fatalf("op/before = %v/%v", ev.Operation, ev.Before)
		}
		if !ev.ColumnsMapped {
			t.Fatal("columnsMapped should be true (bson field-name image keys)")
		}
		if len(ev.Key) != 1 || ev.Key[0] != "_id" {
			t.Fatalf("key = %v", ev.Key)
		}
		if ev.Position.Kind != "token" || ev.Position.Value != "deadbeef" {
			t.Fatalf("position = %+v", ev.Position)
		}
		if ev.TsUnixMs != 1760000000123 {
			t.Fatalf("ts = %d (wallTime preferred)", ev.TsUnixMs)
		}
		if string(ev.After["name"]) != `"alice"` {
			t.Fatalf("after.name = %s", ev.After["name"])
		}
		if string(ev.After["level"]) != "3" {
			t.Fatalf("after.level = %s (number stays number)", ev.After["level"])
		}
	})
	t.Run("update with lookup", func(t *testing.T) {
		txn := int64(7)
		ev, err := mapChangeEvent("src1", mk("update", doc, &txn), "aa")
		if err != nil {
			t.Fatal(err)
		}
		if ev.Operation != connector.OpUpdate || ev.Before != nil {
			t.Fatalf("op/before = %v/%v", ev.Operation, ev.Before)
		}
		if ev.Tx.Seq != 7 || ev.Tx.ID != hex.EncodeToString([]byte{9, 9, 9}) {
			t.Fatalf("tx = %+v", ev.Tx)
		}
	})
	t.Run("update without retrievable doc", func(t *testing.T) {
		_, err := mapChangeEvent("src1", mk("update", nil, nil), "aa")
		if !errors.Is(err, errNoImage) {
			t.Fatalf("err = %v (want errNoImage)", err)
		}
	})
	t.Run("replace maps to update", func(t *testing.T) {
		ev, err := mapChangeEvent("src1", mk("replace", doc, nil), "aa")
		if err != nil {
			t.Fatal(err)
		}
		if ev.Operation != connector.OpUpdate {
			t.Fatalf("op = %v", ev.Operation)
		}
	})
	t.Run("delete no image", func(t *testing.T) {
		ev, err := mapChangeEvent("src1", mk("delete", nil, nil), "aa")
		if err != nil {
			t.Fatal(err)
		}
		if ev.Operation != connector.OpDelete || ev.After != nil || ev.Before != nil {
			t.Fatalf("delete ev = %+v", ev)
		}
	})
	t.Run("clusterTime fallback ts", func(t *testing.T) {
		d := bson.D{
			{Key: "operationType", Value: "insert"},
			{Key: "ns", Value: bson.D{{Key: "db", Value: "g"}, {Key: "coll", Value: "c"}}},
			{Key: "clusterTime", Value: bson.Timestamp{T: 1760000000}},
			{Key: "fullDocument", Value: bson.D{{Key: "x", Value: int32(1)}}},
		}
		raw, err := bson.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		ev, err := mapChangeEvent("s", raw, "aa")
		if err != nil {
			t.Fatal(err)
		}
		if ev.TsUnixMs != 1760000000*1000 {
			t.Fatalf("ts = %d (clusterTime seconds fallback)", ev.TsUnixMs)
		}
	})
}

// 行像 ext-JSON relaxed 形态：ObjectId→{"$oid"}（schema-less 边界的可观测
// 面向消费方的固定形态）。
func TestBSONToJSONMapExtJSONShape(t *testing.T) {
	oid := bson.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c}
	raw, err := bson.Marshal(bson.D{
		{Key: "_id", Value: oid},
		{Key: "n", Value: int64(9007199254740993)},
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := bsonToJSONMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	var idv map[string]json.RawMessage
	if err := json.Unmarshal(m["_id"], &idv); err != nil || len(idv) == 0 {
		t.Fatalf("_id = %s (want ext-json object)", m["_id"])
	}
	if string(m["n"]) != "9007199254740993" {
		t.Fatalf("n = %s", m["n"])
	}
	if got, err := bsonToJSONMap(nil); err != nil || got != nil {
		t.Fatalf("empty doc: %v %v", got, err)
	}
}

// token 位点序列化往返（hex；与契约 ParsePosition/String 闭环）。
func TestTokenRoundTrip(t *testing.T) {
	tok := bson.Raw([]byte{0x05, 0x00, 0x00, 0x00, 0x00})
	s := encodeToken(tok)
	back, err := decodeToken(s)
	if err != nil || string(back) != string(tok) {
		t.Fatalf("round trip: %q %v", s, err)
	}
	pos := connector.Position{Kind: "token", Value: s}
	parsed, err := connector.ParsePosition(pos.String())
	if err != nil || parsed.Kind != "token" || parsed.Value != s {
		t.Fatalf("parse position: %+v %v", parsed, err)
	}
	if _, err := decodeToken("zz"); err == nil {
		t.Fatal("non-hex token should fail")
	}
	if _, err := decodeToken(""); err == nil {
		t.Fatal("empty token should fail")
	}
}

// 流级 pipeline：行级 $in 恒在；白名单 ns $or 按两段形态，空白名单=全库。
func TestBuildPipeline(t *testing.T) {
	p := buildPipeline(nil)
	if len(p) != 1 {
		t.Fatalf("no whitelist pipeline = %d stages (want 1)", len(p))
	}
	p = buildPipeline([]string{"game.players", "log.events", "bad", "a.b.c"})
	if len(p) != 2 {
		t.Fatalf("pipeline = %d stages (want 2)", len(p))
	}
	j, err := bson.MarshalExtJSON(p[1], false, false)
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Match struct {
			Or []map[string]string `json:"$or"`
		} `json:"$match"`
	}
	if err := json.Unmarshal(j, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Match.Or) != 2 {
		t.Fatalf("$or = %v (malformed entries dropped)", v.Match.Or)
	}
}
