package pg

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// ---- 二进制 fixture 工具（大端，与 pgoutput wire 布局一致）----

func u16v(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }
func u32v(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }
func u64v(v uint64) []byte { return binary.BigEndian.AppendUint64(nil, v) }

func cstr(s string) []byte { return append([]byte(s), 0) }

// textDatum 't' 列值：kind + int32 长度 + 字节。
func textDatum(s string) []byte {
	return append(append([]byte{'t'}, u32v(uint32(len(s)))...), s...)
}

func nullDatum() []byte      { return []byte{'n'} }
func unchangedDatum() []byte { return []byte{'u'} }
func binaryDatum(s string) []byte {
	return append(append([]byte{'b'}, u32v(uint32(len(s)))...), s...)
}

// tupleData uint16 列数 + 列值序列。
func tupleData(cols ...[]byte) []byte {
	out := u16v(uint16(len(cols)))
	for _, c := range cols {
		out = append(out, c...)
	}
	return out
}

// relCol Relation 列：flags + name cstring + typeOID + typeMod。
func relCol(flags byte, name string, typeOID uint32) []byte {
	out := append([]byte{flags}, cstr(name)...)
	out = append(out, u32v(typeOID)...)
	return append(out, u32v(0)...)
}

// relPayload Relation 负载：oid + ns + name + identity + 列数 + 列元数据。
func relPayload(oid uint32, schema, name string, identity byte, cols ...[]byte) []byte {
	out := u32v(oid)
	out = append(out, cstr(schema)...)
	out = append(out, cstr(name)...)
	out = append(out, identity)
	out = append(out, u16v(uint16(len(cols)))...)
	for _, c := range cols {
		out = append(out, c...)
	}
	return out
}

// wireMessage 完整消息（含首字节类型）。
func wireMessage(t byte, payload []byte) []byte {
	return append([]byte{t}, payload...)
}

func beginPayload(xid uint32) []byte {
	return append(append(u64v(0), u64v(0)...), u32v(xid)...)
}

func commitPayload(endLSN uint64, micros uint64) []byte {
	out := []byte{0} // flags
	out = append(out, u64v(endLSN-1)...)
	out = append(out, u64v(endLSN)...)
	return append(out, u64v(micros)...)
}

// ---- Relation / Tuple / 基础消息解析 ----

func TestParseRelation(t *testing.T) {
	payload := relPayload(16384, "game", "items", 'd',
		relCol(FlagReplicaIdentity, "id", 23),
		relCol(0, "name", 25),
	)
	rel, err := ParseRelation(payload)
	if err != nil {
		t.Fatalf("ParseRelation: %v", err)
	}
	if rel.OID != 16384 || rel.Schema != "game" || rel.Name != "items" || rel.ReplicaIdentity != 'd' {
		t.Fatalf("relation header mismatch: %+v", rel)
	}
	if len(rel.Columns) != 2 ||
		rel.Columns[0].Name != "id" || rel.Columns[0].TypeOID != 23 ||
		rel.Columns[0].Flags&FlagReplicaIdentity == 0 ||
		rel.Columns[1].Name != "name" {
		t.Fatalf("relation columns mismatch: %+v", rel.Columns)
	}
}

func TestParseRelationTruncated(t *testing.T) {
	payload := relPayload(1, "game", "items", 'd', relCol(0, "id", 23))
	for _, n := range []int{0, 3, len(payload) - 2} {
		if _, err := ParseRelation(payload[:n]); err == nil {
			t.Fatalf("ParseRelation(payload[:%d]) succeeded, want error", n)
		}
	}
}

func TestRelationKeyColumns(t *testing.T) {
	rel := &Relation{
		ReplicaIdentity: 'd',
		Columns: []RelationColumn{
			{Flags: FlagReplicaIdentity, Name: "id"},
			{Flags: 0, Name: "name"},
		},
	}
	if got := rel.KeyColumns(); len(got) != 1 || got[0] != "id" {
		t.Fatalf("KeyColumns(d) = %v, want [id]", got)
	}
	// 'f'（全行）/ 'n'（无）无键信息，不猜。
	for _, identity := range []byte{'f', 'n'} {
		rel.ReplicaIdentity = identity
		if got := rel.KeyColumns(); got != nil {
			t.Fatalf("KeyColumns(%c) = %v, want nil", identity, got)
		}
	}
}

func TestParseTupleKinds(t *testing.T) {
	tup, err := ParseTuple(tupleData(nullDatum(), unchangedDatum(), textDatum("hi"), binaryDatum("x")))
	if err != nil {
		t.Fatalf("ParseTuple: %v", err)
	}
	if len(tup.Columns) != 4 {
		t.Fatalf("column count = %d, want 4", len(tup.Columns))
	}
	if tup.Columns[0].Kind != tupleNull || tup.Columns[1].Kind != tupleUnchanged {
		t.Fatalf("kind mismatch: %+v", tup.Columns[:2])
	}
	if string(tup.Columns[2].Text) != "hi" || string(tup.Columns[3].Text) != "x" {
		t.Fatalf("text mismatch: %+v", tup.Columns[2:])
	}
	// 未知 kind 报错。
	if _, err := ParseTuple([]byte{1, 'z'}); err == nil {
		t.Fatalf("unknown kind accepted")
	}
	// 截断：长度声明大于实际字节。
	if _, err := ParseTuple([]byte{'t', 0, 0, 0, 9, 'a'}); err == nil {
		t.Fatalf("truncated text accepted")
	}
}

func TestParseBeginCommit(t *testing.T) {
	begin, err := ParseBegin(beginPayload(77))
	if err != nil || begin.XID != 77 {
		t.Fatalf("ParseBegin = %+v, %v", begin, err)
	}
	// ts 微秒 → Unix 毫秒：1234567 微秒 → +1234ms。
	commit, err := ParseCommit(commitPayload(0x2000, 1234567))
	if err != nil {
		t.Fatalf("ParseCommit: %v", err)
	}
	if commit.EndLSN != 0x2000 {
		t.Fatalf("EndLSN = %d, want 8192", commit.EndLSN)
	}
	if want := pgEpochUnixMs + 1234; commit.TsUnixMs != want {
		t.Fatalf("TsUnixMs = %d, want %d", commit.TsUnixMs, want)
	}
	if _, err := ParseBegin([]byte{1, 2, 3}); err == nil {
		t.Fatalf("short begin accepted")
	}
	if _, err := ParseCommit([]byte{1, 2, 3}); err == nil {
		t.Fatalf("short commit accepted")
	}
}

// ---- 行消息解析 ----

func TestParseInsert(t *testing.T) {
	payload := append(u32v(16384), 'N')
	payload = append(payload, tupleData(textDatum("1"), textDatum("sword"))...)
	msg, err := ParseInsert(payload)
	if err != nil {
		t.Fatalf("ParseInsert: %v", err)
	}
	if msg.RelationOID != 16384 || len(msg.New.Columns) != 2 || string(msg.New.Columns[1].Text) != "sword" {
		t.Fatalf("insert mismatch: %+v", msg)
	}
	// 缺 'N' 标记报错。
	if _, err := ParseInsert(append(u32v(1), tupleData(textDatum("x"))...)); err == nil {
		t.Fatalf("insert without 'N' accepted")
	}
}

func TestParseUpdateVariants(t *testing.T) {
	old := tupleData(textDatum("1"), unchangedDatum())
	new := tupleData(textDatum("1"), textDatum("v2"))

	// 'K'（身份键旧元组）。
	msg, err := ParseUpdate(append(append(append(u32v(7), 'K'), old...), append([]byte{'N'}, new...)...))
	if err != nil {
		t.Fatalf("ParseUpdate(K): %v", err)
	}
	if !msg.OldIsKey || msg.Old == nil || string(msg.New.Columns[1].Text) != "v2" {
		t.Fatalf("update K mismatch: %+v", msg)
	}

	// 'O'（全行旧元组）。
	msg, err = ParseUpdate(append(append(append(u32v(7), 'O'), old...), append([]byte{'N'}, new...)...))
	if err != nil {
		t.Fatalf("ParseUpdate(O): %v", err)
	}
	if msg.OldIsKey || msg.Old == nil {
		t.Fatalf("update O mismatch: %+v", msg)
	}

	// 无旧元组（副本身份不提供）。
	msg, err = ParseUpdate(append(u32v(7), append([]byte{'N'}, new...)...))
	if err != nil {
		t.Fatalf("ParseUpdate(no old): %v", err)
	}
	if msg.Old != nil {
		t.Fatalf("update no-old mismatch: %+v", msg)
	}

	// 非法标记。
	if _, err := ParseUpdate(append(append(u32v(7), 'X'), new...)); err == nil {
		t.Fatalf("update bad marker accepted")
	}
	// 'K' 后缺新元组 'N'。
	if _, err := ParseUpdate(append(append(u32v(7), 'K'), old...)); err == nil {
		t.Fatalf("update missing new tuple accepted")
	}
}

func TestParseDelete(t *testing.T) {
	old := tupleData(textDatum("9"))
	msg, err := ParseDelete(append(append(u32v(7), 'O'), old...))
	if err != nil {
		t.Fatalf("ParseDelete: %v", err)
	}
	if msg.OldIsKey || msg.RelationOID != 7 || string(msg.Old.Columns[0].Text) != "9" {
		t.Fatalf("delete mismatch: %+v", msg)
	}
	if _, err := ParseDelete(append(u32v(7), 'X', 't', 0, 0, 0, 1, 'x')); err == nil {
		t.Fatalf("delete bad marker accepted")
	}
}

func TestParseTruncate(t *testing.T) {
	payload := append(append(u32v(2), 0), append(u32v(11), u32v(12)...)...)
	msg, err := ParseTruncate(payload)
	if err != nil {
		t.Fatalf("ParseTruncate: %v", err)
	}
	if len(msg.RelationOIDs) != 2 || msg.RelationOIDs[0] != 11 || msg.RelationOIDs[1] != 12 {
		t.Fatalf("truncate mismatch: %+v", msg)
	}
	// 关系数声明超出负载。
	if _, err := ParseTruncate(append(u32v(5), 0)); err == nil {
		t.Fatalf("truncate over-count accepted")
	}
}

// wire 一致性：fixture 拼装的 Relation 消息经过 bytes 缓冲无别名问题（守卫
// tupleSize 游标推进在 'K'/'O' 双元组场景的正确性）。
func TestTupleSizeAdvancesBothTuples(t *testing.T) {
	old := tupleData(textDatum("1"), unchangedDatum())
	new := tupleData(textDatum("1"), textDatum("v2"))
	payload := append(append(append(u32v(7), 'K'), old...), append([]byte{'N'}, new...)...)
	msg, err := ParseUpdate(payload)
	if err != nil {
		t.Fatalf("ParseUpdate: %v", err)
	}
	// 双元组都要解出来（tupleSize 推进错了会在这里炸）。
	if msg.Old == nil || msg.New == nil || len(msg.Old.Columns) != 2 || len(msg.New.Columns) != 2 {
		t.Fatalf("both tuples expected: %+v", msg)
	}
	if !bytes.HasPrefix(payload, u32v(7)) {
		t.Fatalf("payload prefix mutated")
	}
}
