package pg

import (
	"strings"
	"testing"
)

// decoder.go 防御臂收口：各消息类型的解析错误包装、Type/Origin 消息跳过、
// commit 点事件自检失败、schema 历史乱序位点报错、relationEqual 的身份/
// 列差异两分支、tupleImage 的 null 首列与未知列形态、datumJSON 的 bool
// 'f' 分支。
//
// 诚实跳过：datumJSON 的 json.Marshal(string) 失败臂（317-318）——string
// 序列化恒成功，结构性不可达。

func TestDecoderFeedParseErrorArms(t *testing.T) {
	cases := []struct {
		name, want string
		payload    []byte
	}{
		{"begin 截断", "pg: begin:", wireMessage('B', []byte("short"))},
		{"commit 截断", "pg: commit:", wireMessage('C', []byte("short"))},
		{"relation 截断", "pg: relation:", wireMessage('R', []byte("bad"))},
		{"insert 截断", "pg: insert:", wireMessage('I', []byte("abc"))},
		{"update 截断", "pg: update:", wireMessage('U', []byte("abc"))},
		{"delete 截断", "pg: delete:", wireMessage('D', []byte("abcde"))},
		{"truncate 截断", "pg: truncate:", wireMessage('T', []byte("abcd"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDecoder("s")
			if _, err := d.Feed(tc.payload, "100"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want prefix %q", err, tc.want)
			}
		})
	}
}

// Type/Origin/逻辑消息与行变更无关，静默跳过（181）。
func TestDecoderSkipsTypeOriginMessages(t *testing.T) {
	d := NewDecoder("s")
	for _, typ := range []byte{'Y', 'O', 'M'} {
		events, err := d.Feed([]byte{typ}, "100")
		if err != nil || events != nil {
			t.Fatalf("Feed(%q) = %v, %v (want nil, nil)", typ, events, err)
		}
	}
}

// commit 点事件自检失败：空 SourceID 的解码器产出的首事件过不了契约
// 校验，按本包缺陷暴露（138-139）。
func TestDecoderCommitValidateRejectsMissingSourceID(t *testing.T) {
	d := NewDecoder("")
	mustFeed(t, d, wireMessage('R', relPayload(itemsOID, "game", "items", 'd',
		relCol(FlagReplicaIdentity, "id", 23))))
	mustFeed(t, d, beginMsg(1))
	mustFeed(t, d, insertMsg(textDatum("1")))
	_, err := d.Feed(commitMsg(10), "100")
	if err == nil || !strings.Contains(err.Error(), "decoded event 0") {
		t.Fatalf("commit validate err = %v", err)
	}
}

// schema 历史乱序位点：同表在更早位点记新版本 = 协议致命错误（345-346
// → Feed 包装 148-149）。
func TestDecoderHistoryRecordRejectsOutOfOrderLSN(t *testing.T) {
	d := NewDecoder("s")
	mustFeedAt(t, d, "200", itemsRelation('d'))
	_, err := d.Feed(itemsRelationChanged('d'), "100")
	if err == nil || !strings.Contains(err.Error(), "pg: relation game.items") {
		t.Fatalf("out-of-order record err = %v", err)
	}
}

// relationEqual 身份差异（366-367）：同 OID 同列集但 ReplicaIdentity 变化
// 视为结构变化，修订号 bump。
func TestDecoderRelationIdentityChangeBumpsRevision(t *testing.T) {
	d := NewDecoder("s")
	mustFeedAt(t, d, "100", itemsRelation('d'))
	mustFeedAt(t, d, "200", itemsRelation('f'))
	if d.revision != 2 {
		t.Fatalf("revision = %d, want 2 (identity change)", d.revision)
	}
	if vers := d.SchemaHistory().Versions("game.items"); len(vers) != 2 {
		t.Fatalf("versions = %d, want 2", len(vers))
	}
}

// relationEqual 列差异（374-375）：同列数但列类型 OID 变化视为结构变化。
func TestDecoderColumnTypeChangeBumpsRevision(t *testing.T) {
	d := NewDecoder("s")
	mustFeedAt(t, d, "100", itemsRelation('d'))
	changed := wireMessage('R', relPayload(itemsOID, "game", "items", 'd',
		relCol(FlagReplicaIdentity, "id", 23),
		relCol(0, "name", 25),
		relCol(0, "gold", 23), // numeric → integer
		relCol(0, "blob", 25),
	))
	mustFeedAt(t, d, "200", changed)
	if d.revision != 2 {
		t.Fatalf("revision = %d, want 2 (column type change)", d.revision)
	}
}

// tupleImage 防御臂：null 首列初始化行像（254-255）、未知列形态拒绝
// （272-273）。
func TestDecoderTupleImageDefensiveArms(t *testing.T) {
	d := NewDecoder("s")
	rel := &Relation{OID: 1, Schema: "g", Name: "t",
		Columns: []RelationColumn{{Name: "id", TypeOID: 23}}}

	img, err := d.tupleImage(rel, &Tuple{Columns: []TupleColumn{{Kind: tupleNull}}})
	if err != nil || string(img["id"]) != "null" {
		t.Fatalf("null-first image = %v, %v", img, err)
	}

	if _, err = d.tupleImage(rel, &Tuple{Columns: []TupleColumn{{Kind: 'x'}}}); err == nil ||
		!strings.Contains(err.Error(), "unknown tuple kind") {
		t.Fatalf("unknown kind err = %v", err)
	}
}

// datumJSON bool 'f' 分支（307）。
func TestDecoderDatumJSONFalseBool(t *testing.T) {
	v, err := datumJSON(boolTypeOID, []byte("f"))
	if err != nil || string(v) != "false" {
		t.Fatalf("bool f = %s, %v", v, err)
	}
}

// appendRow 的旧像译码失败包装（223-224 经 Feed 的 update 包装）：update
// 键元组列数与 Relation 不符。
func TestDecoderUpdateOldTupleShapeMismatch(t *testing.T) {
	d := NewDecoder("s")
	mustFeed(t, d, itemsRelation('d'))
	mustFeed(t, d, beginMsg(1))
	_, err := d.Feed(updateKMsg(tupleData(textDatum("1")),
		tupleData(textDatum("1"), textDatum("n"), textDatum("0"), nullDatum())), "100")
	if err == nil || !strings.Contains(err.Error(), "pg: update") ||
		!strings.Contains(err.Error(), "tuple has 1 columns") {
		t.Fatalf("old tuple shape err = %v", err)
	}
}

// truncate 消息带多个 OID 时按关系数计数（TruncatedRelations 面补齐）。
func TestDecoderTruncateCountsAllRelations(t *testing.T) {
	d := NewDecoder("s")
	payload := append(u32v(2), 0)
	payload = append(payload, u32v(itemsOID)...)
	payload = append(payload, u32v(2)...)
	mustFeed(t, d, wireMessage('T', payload))
	if d.TruncatedRelations != 2 {
		t.Fatalf("TruncatedRelations = %d, want 2", d.TruncatedRelations)
	}
}
