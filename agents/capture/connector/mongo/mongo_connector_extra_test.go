package mongo

// connector.go / address.go 补测：Open 的关闭态/已打开/坏 URI/不可达
// ping 收口、Discover 的列表失败两臂（全库扫描与限定库）、Poll 的坏
// token 与未知位点 kind 拒绝、AckCheckpoint 守卫、Close 的幂等/join/
// 滞留超时、mapChangeEvent 解码与校验失败臂、bsonToJSONMap 空与坏文档、
// encodeToken 空值、AddressFromDSN 的补端口与 SRV 解析失败。
//
// 诚实跳过（需真实 mongod/副本集，由集成测试覆盖）：Open 的拓扑校验臂
// （hello/replica set 判定）、Discover 成功路径、Poll 的空起点/续传
// token 日志臂（Watch 同一活流路径）、Watch 流与 pump 循环、SRV 解析
// 成功臂、Close 的 Disconnect 分支（零值 Client 句柄触碰未初始化驱动
// 内部）。诚实跳过（结构性不可达）：bsonToJSONMap 的 extJSON 编码/解析
// 失败分支（合法 BSON 的 extJSON 往返恒成功）；fullDocument 行像坏值
// 经手工拼字节覆盖——bson.Marshal 会深度校验嵌套 Raw，正常构造不出来。

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/connector"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// refusedMongoURI 指向必拒端口的短超时 DSN（server selection 快速失败，
// 不依赖外部服务）。
func refusedMongoURI() string {
	return "mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=300&connectTimeoutMS=300"
}

// refusedClient 懒连接客户端（不拨号即可作非 nil 句柄；首次操作快速失败）。
func refusedClient(t *testing.T) *mongo.Client {
	t.Helper()
	cl, err := mongo.Connect(options.Client().ApplyURI(refusedMongoURI()))
	if err != nil {
		t.Fatalf("lazy connect: %v", err)
	}
	return cl
}

func TestOpenClosedAlreadyOpenAndConnectArms(t *testing.T) {
	const env = "CROUPIER_CAPTURE_MONGO_DSN_EXTRA_TEST"

	// 关闭态拒绝
	c := NewMongoConnector(connector.SourceConfig{DSNEnv: env}, nil)
	c.closed.Store(true)
	if err := c.Open(context.Background(), connector.SourceConfig{DSNEnv: env}); err == nil ||
		!strings.Contains(err.Error(), "closed") {
		t.Fatalf("open after close = %v", err)
	}

	// 已打开拒绝（客户端句柄占位即可，守卫不触碰连接方法）
	c2 := NewMongoConnector(connector.SourceConfig{}, nil)
	c2.client = &mongo.Client{}
	if err := c2.Open(context.Background(), connector.SourceConfig{DSNEnv: env}); err == nil ||
		!strings.Contains(err.Error(), "already open") {
		t.Fatalf("double open = %v", err)
	}

	// 坏 URI → 连接构造失败（错误只提环境变量名）
	t.Setenv(env, "this-is-not-a-uri")
	c3 := NewMongoConnector(connector.SourceConfig{}, nil)
	if err := c3.Open(context.Background(), connector.SourceConfig{DSNEnv: env}); err == nil ||
		!strings.Contains(err.Error(), "connect via") {
		t.Fatalf("bad uri = %v", err)
	}

	// 不可达 → ping 失败收口（短超时快速失败）
	t.Setenv(env, refusedMongoURI())
	c4 := NewMongoConnector(connector.SourceConfig{}, nil)
	if err := c4.Open(context.Background(), connector.SourceConfig{DSNEnv: env}); err == nil ||
		!strings.Contains(err.Error(), "ping via") {
		t.Fatalf("unreachable ping = %v", err)
	}
}

func TestDiscoverListFailures(t *testing.T) {
	ctx := context.Background()

	// 未限定库：ListDatabaseNames 失败收口
	c := NewMongoConnector(connector.SourceConfig{}, nil)
	c.client = refusedClient(t)
	if _, err := c.Discover(ctx, connector.DiscoveryRequest{}); err == nil ||
		!strings.Contains(err.Error(), "list databases") {
		t.Fatalf("list databases = %v", err)
	}

	// 限定库：ListCollectionNames 失败收口
	c2 := NewMongoConnector(connector.SourceConfig{Options: map[string]string{"database": "game"}}, nil)
	c2.client = refusedClient(t)
	if _, err := c2.Discover(ctx, connector.DiscoveryRequest{}); err == nil ||
		!strings.Contains(err.Error(), "list collections") {
		t.Fatalf("list collections = %v", err)
	}
}

func TestPollRejectsBadTokenAndUnknownKind(t *testing.T) {
	c := NewMongoConnector(connector.SourceConfig{}, nil)
	c.client = &mongo.Client{}

	// 非法 hex token
	_, err := c.Poll(context.Background(),
		connector.PollRequest{From: connector.Position{Kind: "token", Value: "zz-not-hex"}})
	if err == nil || !strings.Contains(err.Error(), "decode checkpoint token") {
		t.Fatalf("bad token = %v", err)
	}

	// 位点 kind 闭集外
	_, err = c.Poll(context.Background(),
		connector.PollRequest{From: connector.Position{Kind: "gtid", Value: "1"}})
	if err == nil || !strings.Contains(err.Error(), "gtid") || !strings.Contains(err.Error(), "not applicable") {
		t.Fatalf("gtid kind = %v", err)
	}
}

func TestAckCheckpointClosedAndInvalid(t *testing.T) {
	c := NewMongoConnector(connector.SourceConfig{}, nil)

	c.closed.Store(true)
	if err := c.AckCheckpoint(context.Background(),
		connector.Position{Kind: "token", Value: "aa"}); err == nil ||
		!strings.Contains(err.Error(), "closed") {
		t.Fatalf("ack after close = %v", err)
	}
	c.closed.Store(false)

	if err := c.AckCheckpoint(context.Background(),
		connector.Position{Kind: "lsn", Value: "1"}); err == nil ||
		!strings.Contains(err.Error(), "invalid token checkpoint") {
		t.Fatalf("lsn ack = %v", err)
	}

	if err := c.AckCheckpoint(context.Background(),
		connector.Position{Kind: "token", Value: "aabb"}); err != nil {
		t.Fatalf("valid ack = %v", err)
	}
	pos, err := c.Checkpoint(context.Background())
	if err != nil || pos.Value != "aabb" {
		t.Fatalf("checkpoint = %+v, %v", pos, err)
	}
}

func TestCloseGuardsJoinAndTimeout(t *testing.T) {
	// 未开连接的 Close 是 no-op 且幂等
	c := NewMongoConnector(connector.SourceConfig{}, nil)
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("close fresh = %v", err)
	}
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("close idempotent = %v", err)
	}

	// pump join：取消被调用、句柄复位
	cancelled := false
	done := make(chan struct{})
	c2 := NewMongoConnector(connector.SourceConfig{}, nil)
	c2.cancel = func() { cancelled = true }
	c2.pumpDone = done
	close(done)
	if err := c2.Close(context.Background()); err != nil {
		t.Fatalf("close with pump = %v", err)
	}
	if !cancelled || c2.cancel != nil || c2.pumpDone != nil {
		t.Fatalf("close join = cancelled %v cancel %v done %v", cancelled, c2.cancel, c2.pumpDone)
	}

	// pump 滞留：Close 防御超时（5s）后收尾，不无限挂死
	c3 := NewMongoConnector(connector.SourceConfig{}, nil)
	c3.cancel = func() {} // 永不结束的 pump
	c3.pumpDone = make(chan struct{})
	if err := c3.Close(context.Background()); err != nil {
		t.Fatalf("close stuck pump = %v", err)
	}
	if c3.cancel != nil || c3.pumpDone != nil {
		t.Fatalf("stuck pump close handles = %v %v", c3.cancel, c3.pumpDone)
	}
}

func TestMapChangeEventErrorArms(t *testing.T) {
	// 事件头解码失败（坏 BSON）
	if _, err := mapChangeEvent("s", bson.Raw("\x05\x00\x00"), ""); err == nil ||
		!strings.Contains(err.Error(), "decode change event") {
		t.Fatalf("bad event doc = %v", err)
	}

	// fullDocument 行像解码失败：bson.Marshal 会深度校验嵌套 Raw，构造
	// 不出「外层自洽、内层越界」形态，只能手工拼字节——fullDocument 的
	// string 元素长度声明越界 → bsonToJSONMap 解码收口
	nested := []byte{0x0C, 0x00, 0x00, 0x00, 0x02, 'a', 0x00, 0x7F, 0xFF, 0xFF, 0xFF, 0x00}
	var junkDoc []byte
	junkDoc = append(junkDoc, 0x00, 0x00, 0x00, 0x00) // 长度占位
	junkDoc = append(junkDoc, 0x02)                   // operationType: "insert"
	junkDoc = append(junkDoc, "operationType"...)
	junkDoc = append(junkDoc, 0x00, 0x07, 0x00, 0x00, 0x00)
	junkDoc = append(junkDoc, "insert"...)
	junkDoc = append(junkDoc, 0x00)
	junkDoc = append(junkDoc, 0x03) // fullDocument: 嵌套文档
	junkDoc = append(junkDoc, "fullDocument"...)
	junkDoc = append(junkDoc, 0x00)
	junkDoc = append(junkDoc, nested...)
	junkDoc = append(junkDoc, 0x00)
	binary.LittleEndian.PutUint32(junkDoc[0:4], uint32(len(junkDoc)))
	if _, err := mapChangeEvent("s", bson.Raw(junkDoc), ""); err == nil {
		t.Fatal("bad full document accepted")
	}

	// 契约校验失败（sourceID 为空 → 契约自检拒绝）
	doc, err := bson.Marshal(bson.D{
		{Key: "operationType", Value: "insert"},
		{Key: "ns", Value: bson.D{{Key: "db", Value: "game"}, {Key: "coll", Value: "players"}}},
		{Key: "fullDocument", Value: bson.D{{Key: "_id", Value: "1"}}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err = mapChangeEvent("", doc, "aa"); err == nil {
		t.Fatal("empty source id must fail contract validation")
	}
}

func TestBSONToJSONMapEmptyAndJunk(t *testing.T) {
	m, err := bsonToJSONMap(nil)
	if err != nil || m != nil {
		t.Fatalf("empty raw = %v, %v (want nil, nil)", m, err)
	}
	if _, err = bsonToJSONMap(bson.Raw("\x05\x00\x00")); err == nil ||
		!strings.Contains(err.Error(), "decode document") {
		t.Fatalf("junk raw = %v", err)
	}
}

func TestEncodeTokenEmpty(t *testing.T) {
	if got := encodeToken(nil); got != "" {
		t.Fatalf("empty token = %q", got)
	}
}

func TestAddressFromDSNPortAndSRVFailure(t *testing.T) {
	const env = "CROUPIER_CAPTURE_MONGO_ADDR_EXTRA_TEST"

	// 无端口地址补默认端口
	t.Setenv(env, "mongodb://10.0.0.8")
	addr, err := AddressFromDSN(env)
	if err != nil || addr != "10.0.0.8:27017" {
		t.Fatalf("port append = %q, %v", addr, err)
	}

	// 多 host 取首个（url.Parse 不接受带端口逗号串，裸 host 串才可解析）
	t.Setenv(env, "mongodb://10.0.0.8,10.0.0.9/")
	addr, err = AddressFromDSN(env)
	if err != nil || addr != "10.0.0.8:27017" {
		t.Fatalf("multi host = %q, %v", addr, err)
	}

	// mongodb+srv：SRV 记录解析失败收口（.invalid 域恒不解析）
	t.Setenv(env, "mongodb+srv://no-such-host.invalid")
	_, err = AddressFromDSN(env)
	if err == nil || !strings.Contains(err.Error(), "srv lookup") {
		t.Fatalf("srv failure = %v", err)
	}
}
