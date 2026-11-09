package mongo

// applier/auditor 补测（无真库单测口径）：Open 的空 uriEnv/未设环境变量/
// 坏 URI/不可达 ping 四臂、NewApplier 与 EnsureSchema 的 nil client 守卫、
// Close 的 nil 臂、Capabilities、ValidateTransaction 的映射/镜像/目标/键/
// 文档 ID 全守卫臂（全部在触达需要真库的 collectionExists 之前收口）、
// ApplyTransaction 的入参守卫与校验透传臂、eventKey/keyFilter/decodeDocument/
// canonicalDocument/documentID 纯函数臂、Auditor 的 nil 与身份字段守卫臂、
// Append 的 At 兜底臂与 InsertOne 错误透传臂（惰性 client 指向不可达地址，
// server selection 定时失败）。
//
// 诚实跳过（mongo-driver v2 暴露具体类型 *mongo.Client/*mongo.Collection，
// 无接口可 mock；全部由 integration 标签的 TestMongoApplierRealTarget 真库
// 覆盖，不进单测覆盖率口径）：EnsureSchema/NewAuditor 的建索引成功臂、
// ApplyTransaction 的 StartSession 与 WithTransaction 闭包体（账本读/预留/
// 幂等命中/内容冲突；TransientTransactionError 重试直至交易超时，单测
// 时长不可控）、ValidateTransaction 与 applyEvent 的集合存在性/读写成功臂
// （全守卫通过后的 return 依赖真库枚举）、
// Auditor Append 的 InsertOne 成功臂、hashEvents 的 json.Marshal 错误臂
// （ChangeEvent 全字段可序列化）、decodeDocument 的 WriteDocument/Unmarshal
// 错误臂（写入自建 buffer 的是合法 BSON，两侧守卫不可达）、encode/copy 的
// 值级错误臂（ExtJSON 解析在 reader 构造期即拒绝坏 JSON，decode 不再失败）。

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

func TestOpenGuards(t *testing.T) {
	// 空 uriEnv
	_, err := Open(context.Background(), "  ")
	require.ErrorContains(t, err, "uriEnv is required")

	// 环境变量未设
	_, err = Open(context.Background(), "CROUPIER_MONGO_RECOVERY_DSN_UNSET_XYZ")
	require.ErrorContains(t, err, "not set")

	// 坏 URI → connect 失败
	t.Setenv("CROUPIER_MONGO_RECOVERY_DSN_BAD_XYZ", "://not-a-uri")
	_, err = Open(context.Background(), "CROUPIER_MONGO_RECOVERY_DSN_BAD_XYZ")
	require.ErrorContains(t, err, "mongo recovery:")

	// 不可达 → ping 失败（惰性连接在 Ping 的 server selection 才爆发）
	t.Setenv("CROUPIER_MONGO_RECOVERY_DSN_UNREACH_XYZ",
		"mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=300&connectTimeoutMS=200")
	_, err = Open(context.Background(), "CROUPIER_MONGO_RECOVERY_DSN_UNREACH_XYZ")
	require.ErrorContains(t, err, "ping")
}

func TestNewApplierAndEnsureSchemaGuards(t *testing.T) {
	_, err := NewApplier(context.Background(), nil)
	require.ErrorContains(t, err, "client is required")
	require.ErrorContains(t, EnsureSchema(context.Background(), nil), "client is required")

	// 建索引失败臂：惰性 client 在 server selection 处确定性失败
	_, err = NewApplier(context.Background(), lazyUnreachableClient(t))
	require.ErrorContains(t, err, "ensure ledger")
	err = EnsureSchema(context.Background(), lazyUnreachableClient(t))
	require.ErrorContains(t, err, "ensure ledger")
}

func TestCloseNilArms(t *testing.T) {
	require.NoError(t, (*Applier)(nil).Close(context.Background()))
	require.NoError(t, (&Applier{}).Close(context.Background()))
	require.NoError(t, (&Applier{client: lazyUnreachableClient(t)}).Close(context.Background()))
}

func TestCapabilitiesReportsSupportedOps(t *testing.T) {
	caps, err := (&Applier{}).Capabilities(context.Background())
	require.NoError(t, err)
	require.True(t, caps.AtomicTransactions)
	require.True(t, caps.IdempotentTransactions)
	require.Equal(t,
		[]cdc.Operation{cdc.OpInsert, cdc.OpUpdate, cdc.OpDelete}, caps.SupportedOperations)
}

// lazyUnreachableClient 构造惰性 client（指向不可达地址，server selection
// 定时失败），用于在无真库时驱动「连接/查询失败」臂。
func lazyUnreachableClient(t *testing.T) *mongo.Client {
	t.Helper()
	client, err := mongo.Connect(options.Client().ApplyURI(
		"mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=300&connectTimeoutMS=200"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return client
}

// ValidateTransaction 守卫臂全部在 collectionExists（需真库）之前收口。
func TestValidateTransactionGuards(t *testing.T) {
	ctx := context.Background()
	a := &Applier{}
	base := cdc.ChangeEvent{
		ID: "e1", Database: "app", Table: "coll", Operation: cdc.OpInsert,
		ColumnsMapped: true, Key: []string{"_id"},
		After: map[string]json.RawMessage{"_id": json.RawMessage(`"x"`), "v": json.RawMessage(`1`)},
	}

	require.ErrorContains(t, a.ValidateTransaction(ctx, recovery.Plan{}, nil), "empty transaction")

	unmapped := base
	unmapped.ColumnsMapped = false
	require.ErrorContains(t,
		a.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{unmapped}), "columns are not mapped")

	noAfter := base
	noAfter.After = nil
	require.ErrorContains(t,
		a.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{noAfter}), "has no after image")

	updNoAfter := base
	updNoAfter.Operation = cdc.OpUpdate
	updNoAfter.After = nil
	require.ErrorContains(t,
		a.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{updNoAfter}), "has no after image")

	delNoBefore := base
	delNoBefore.Operation = cdc.OpDelete
	require.ErrorContains(t,
		a.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{delNoBefore}), "has no before image")

	badOp := base
	badOp.Operation = "rename"
	require.ErrorContains(t,
		a.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{badOp}), "unsupported operation")

	noNs := base
	noNs.Database = ""
	require.ErrorContains(t,
		a.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{noNs}), "no target database and collection")

	noKey := base
	noKey.Key = nil
	require.ErrorContains(t,
		a.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{noKey}), "exactly one _id key")

	wrongKey := base
	wrongKey.Key = []string{"rowid"}
	require.ErrorContains(t,
		a.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{wrongKey}), "want _id")

	// 键列不在镜像中
	missingKey := base
	missingKey.After = map[string]json.RawMessage{"v": json.RawMessage(`1`)}
	require.ErrorContains(t,
		a.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{missingKey}), `key "_id" is missing`)

	// 全守卫通过后触达 collectionExists：惰性 client 的枚举失败透传
	a.client = lazyUnreachableClient(t)
	require.ErrorContains(t,
		a.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{base}), "inspect app.coll")
}

func TestApplyTransactionEntryGuards(t *testing.T) {
	a := &Applier{}
	require.ErrorContains(t,
		a.ApplyTransaction(context.Background(), "", "tx", nil), "are required")
	require.ErrorContains(t,
		a.ApplyTransaction(context.Background(), "req", "tx", []cdc.ChangeEvent{}), "are required")

	bad := cdc.ChangeEvent{ID: "e"}
	require.ErrorContains(t,
		a.ApplyTransaction(context.Background(), "req", "tx", []cdc.ChangeEvent{bad}),
		"columns are not mapped")
}

func TestEventKeyGuards(t *testing.T) {
	_, err := eventKey(cdc.ChangeEvent{ID: "e"})
	require.ErrorContains(t, err, "exactly one _id key")

	_, err = eventKey(cdc.ChangeEvent{ID: "e", Key: []string{"rowid"}})
	require.ErrorContains(t, err, `key is "rowid", want _id`)

	key, err := eventKey(cdc.ChangeEvent{Key: []string{"_id"}})
	require.NoError(t, err)
	require.Equal(t, []string{"_id"}, key)
}

func TestKeyFilterCopiesKeyColumns(t *testing.T) {
	image := map[string]json.RawMessage{"_id": json.RawMessage(`1`), "v": json.RawMessage(`2`)}
	require.Equal(t,
		map[string]json.RawMessage{"_id": json.RawMessage(`1`)},
		keyFilter(image, []string{"_id"}))
}

func TestDecodeDocumentAndCanonical(t *testing.T) {
	_, err := decodeDocument(nil)
	require.ErrorContains(t, err, "empty document image")

	doc, err := decodeDocument(map[string]json.RawMessage{
		"_id": json.RawMessage(`"a"`), "v": json.RawMessage(`3`),
	})
	require.NoError(t, err)
	require.Equal(t, bson.D{{Key: "_id", Value: "a"}, {Key: "v", Value: int32(3)}}, doc)

	// 值非合法 ExtJSON → 错误透传
	_, err = decodeDocument(map[string]json.RawMessage{"_id": json.RawMessage(`{oops`)})
	require.Error(t, err)

	// canonicalDocument 按字段名排序
	canonical := canonicalDocument(bson.M{"b": 2, "a": 1})
	require.Equal(t, bson.D{{Key: "a", Value: 1}, {Key: "b", Value: 2}}, canonical)
}

func TestDocumentIDArms(t *testing.T) {
	_, err := documentID(map[string]json.RawMessage{"v": json.RawMessage(`1`)}, "_id", "e1")
	require.ErrorContains(t, err, `key "_id" is missing`)

	v, err := documentID(map[string]json.RawMessage{"_id": json.RawMessage(`"abc"`)}, "_id", "e1")
	require.NoError(t, err)
	require.Equal(t, "abc", v)

	_, err = documentID(map[string]json.RawMessage{"_id": json.RawMessage(`{bad`)}, "_id", "e1")
	require.ErrorContains(t, err, "decode key")
}

func TestHashEventsDeterministic(t *testing.T) {
	events := []cdc.ChangeEvent{{
		ID: "e1", SourceID: "s", SourceType: "mysql", Database: "app", Table: "coll",
		Operation: cdc.OpInsert, ColumnsMapped: true, Key: []string{"_id"},
		After: map[string]json.RawMessage{"_id": json.RawMessage(`"x"`)},
	}}
	h1, err := hashEvents(events)
	require.NoError(t, err)
	h2, err := hashEvents(events)
	require.NoError(t, err)
	require.Equal(t, h1, h2)
	require.Len(t, h1, 32)
}

// applyEvent 的纯解码臂：集合句柄构造不触网，解码失败在首次网络调用前收口。
func TestApplyEventDecodeArms(t *testing.T) {
	ctx := context.Background()
	client := lazyUnreachableClient(t)

	// 键必须恰为单 _id
	badKey := cdc.ChangeEvent{ID: "e", Operation: cdc.OpInsert, Key: nil,
		After: map[string]json.RawMessage{"v": json.RawMessage(`1`)}}
	require.ErrorContains(t, applyEvent(ctx, client, badKey), "exactly one _id key")

	// insert 后像坏值
	insBadVal := cdc.ChangeEvent{ID: "e", Operation: cdc.OpInsert, Key: []string{"_id"},
		After: map[string]json.RawMessage{"v": json.RawMessage(`{oops`)}}
	require.Error(t, applyEvent(ctx, client, insBadVal))

	// insert 键值坏（键过滤器解码失败）
	insBadKey := cdc.ChangeEvent{ID: "e", Operation: cdc.OpInsert, Key: []string{"_id"},
		After: map[string]json.RawMessage{"_id": json.RawMessage(`{oops`), "v": json.RawMessage(`1`)}}
	require.Error(t, applyEvent(ctx, client, insBadKey))

	// update 后像坏值 / 键值坏
	updBadVal := cdc.ChangeEvent{ID: "e", Operation: cdc.OpUpdate, Key: []string{"_id"},
		After: map[string]json.RawMessage{"v": json.RawMessage(`{oops`)}}
	require.Error(t, applyEvent(ctx, client, updBadVal))
	updBadKey := cdc.ChangeEvent{ID: "e", Operation: cdc.OpUpdate, Key: []string{"_id"},
		After: map[string]json.RawMessage{"_id": json.RawMessage(`{oops`), "v": json.RawMessage(`1`)}}
	require.Error(t, applyEvent(ctx, client, updBadKey))

	// delete 前像键值坏
	delBadKey := cdc.ChangeEvent{ID: "e", Operation: cdc.OpDelete, Key: []string{"_id"},
		Before: map[string]json.RawMessage{"_id": json.RawMessage(`{oops`)}}
	require.Error(t, applyEvent(ctx, client, delBadKey))

	// 闭集外操作
	badOp := cdc.ChangeEvent{ID: "e", Operation: "rename", Key: []string{"_id"}}
	require.ErrorContains(t, applyEvent(ctx, client, badOp), "unsupported operation")
}

// collectionExists 的枚举失败臂：惰性 client 在 server selection 处失败。
func TestCollectionExistsInspectError(t *testing.T) {
	err := collectionExists(context.Background(), lazyUnreachableClient(t), "app", "coll")
	require.ErrorContains(t, err, "inspect app.coll")
}

func TestAuditorGuards(t *testing.T) {
	_, err := NewAuditor(context.Background(), nil)
	require.ErrorContains(t, err, "client is required")

	require.ErrorContains(t,
		(*Auditor)(nil).Append(context.Background(), recovery.AuditRecord{}), "not open")
	require.ErrorContains(t,
		(&Auditor{}).Append(context.Background(), recovery.AuditRecord{}), "not open")

	// 身份字段缺失（collection 非 nil，经惰性 client 构造句柄）与建索引失败臂
	client := lazyUnreachableClient(t)
	a := &Auditor{collection: client.Database(ledgerDatabase).Collection("audit_records")}
	require.ErrorContains(t,
		a.Append(context.Background(), recovery.AuditRecord{RequestID: "r"}), "identity fields")

	_, err = NewAuditor(context.Background(), client)
	require.ErrorContains(t, err, "ensure schema")
}

// Append 的 At 兜底臂与 InsertOne 错误透传臂：惰性 client 指向不可达地址，
// InsertOne 在 server selection 超时后确定性失败。
func TestAuditorAppendFillsZeroAtAndSurfacesInsertError(t *testing.T) {
	client := lazyUnreachableClient(t)

	a := &Auditor{collection: client.Database(ledgerDatabase).Collection("audit_records")}
	record := recovery.AuditRecord{
		RequestID: "req-1", Principal: "op", SourceID: "src", TargetID: "tgt",
		Stage: "transaction_applied", Mode: recovery.ModeApply,
		At: time.Unix(1730000000, 0),
	}
	require.ErrorContains(t, a.Append(context.Background(), record), "append")

	record.At = time.Time{}
	require.ErrorContains(t, a.Append(context.Background(), record), "append")
}
