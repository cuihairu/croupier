package pg

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
)

// game.items 表 fixture：id(身份键, int4) name(text) gold(numeric) blob(text)。
const itemsOID = uint32(16384)

func itemsRelation(identity byte) []byte {
	return wireMessage('R', relPayload(itemsOID, "game", "items", identity,
		relCol(FlagReplicaIdentity, "id", 23),
		relCol(0, "name", 25),
		relCol(0, "gold", 1700),
		relCol(0, "blob", 25),
	))
}

func beginMsg(xid uint32) []byte { return wireMessage('B', beginPayload(xid)) }
func commitMsg(lsn uint64) []byte {
	// 2025-06-01T00:00:00Z 对应的 PG 纪元微秒（可读性优先，公式即断言）。
	const unixMs = int64(1748736000000)
	micros := uint64((unixMs - pgEpochUnixMs) * 1000)
	return wireMessage('C', commitPayload(lsn, micros))
}

func insertMsg(tup ...[]byte) []byte {
	payload := append(u32v(itemsOID), 'N')
	return wireMessage('I', append(payload, tupleData(tup...)...))
}

func updateKMsg(old, new []byte) []byte {
	payload := append(append(u32v(itemsOID), 'K'), old...)
	payload = append(payload, append([]byte{'N'}, new...)...)
	return wireMessage('U', payload)
}

func updateNoOldMsg(new []byte) []byte {
	payload := append(append(u32v(itemsOID), 'N'), new...)
	return wireMessage('U', payload)
}

func deleteMsg(old []byte) []byte {
	payload := append(append(u32v(itemsOID), 'K'), old...)
	return wireMessage('D', payload)
}

func mustFeed(t *testing.T, d *Decoder, payload []byte) []connector.ChangeEvent {
	t.Helper()
	events, err := d.Feed(payload)
	if err != nil {
		t.Fatalf("Feed(%q): %v", string(payload[:1]), err)
	}
	return events
}

// 完整事务：B → I → U(K) → U(无旧像) → D → T → C，四行事件在提交点一次
// 吐出，元数据（列名映射/身份键/事务序号/位点/时间/结构修订）齐备。
func TestDecoderTransactionAssembly(t *testing.T) {
	d := NewDecoder("cap-pg-1")
	mustFeed(t, d, itemsRelation('d'))

	mustFeed(t, d, beginMsg(77))
	mustFeed(t, d, insertMsg(textDatum("1"), textDatum("sword"), textDatum("100"), nullDatum()))
	mustFeed(t, d, updateKMsg(
		tupleData(textDatum("1"), unchangedDatum(), unchangedDatum(), unchangedDatum()),
		tupleData(textDatum("1"), textDatum("sword2"), textDatum("150"), nullDatum()),
	))
	mustFeed(t, d, updateNoOldMsg(tupleData(textDatum("2"), textDatum("shield"), textDatum("30"), nullDatum())))
	mustFeed(t, d, deleteMsg(tupleData(textDatum("1"), unchangedDatum(), unchangedDatum(), unchangedDatum())))
	mustFeed(t, d, wireMessage('T', append(append(u32v(1), 0), u32v(itemsOID)...)))

	if d.TruncatedRelations != 1 {
		t.Fatalf("TruncatedRelations = %d, want 1", d.TruncatedRelations)
	}
	events := mustFeed(t, d, commitMsg(0x10<<32|0xABC))
	if len(events) != 4 {
		t.Fatalf("event count = %d, want 4 (truncate skipped)", len(events))
	}

	wantOps := []connector.Operation{connector.OpInsert, connector.OpUpdate, connector.OpUpdate, connector.OpDelete}
	for i, ev := range events {
		if ev.Operation != wantOps[i] {
			t.Fatalf("event %d op = %s, want %s", i, ev.Operation, wantOps[i])
		}
		if err := (&events[i]).Validate(); err != nil {
			t.Fatalf("event %d invalid: %v", i, err)
		}
		if ev.SourceID != "cap-pg-1" || ev.SourceType != "postgres" ||
			ev.Database != "game" || ev.Table != "items" {
			t.Fatalf("event %d identity mismatch: %+v", i, ev)
		}
		// 列名映射（PG 关键差异）+ 身份键。
		if !ev.ColumnsMapped {
			t.Fatalf("event %d ColumnsMapped = false, want true", i)
		}
		if len(ev.Key) != 1 || ev.Key[0] != "id" {
			t.Fatalf("event %d key = %v, want [id]", i, ev.Key)
		}
		// 事务元数据 + 结构修订 + 位点 + 提交时间。
		if ev.Tx.ID != "77" || ev.Tx.Seq != int64(i+1) {
			t.Fatalf("event %d tx = %+v", i, ev.Tx)
		}
		if ev.Tx.End != (i == len(events)-1) {
			t.Fatalf("event %d transaction end = %v", i, ev.Tx.End)
		}
		if ev.SchemaVersion != 1 {
			t.Fatalf("event %d schemaVersion = %d, want 1", i, ev.SchemaVersion)
		}
		if ev.Position.Kind != "lsn" || ev.Position.Value != (LSN(0x10)<<32|0xABC).Decimal() {
			t.Fatalf("event %d position = %+v", i, ev.Position)
		}
		if want := int64(1748736000000); ev.TsUnixMs != want {
			t.Fatalf("event %d ts = %d, want %d", i, ev.TsUnixMs, want)
		}
	}

	// 行像抽查：insert 全后像；update(K) 前像仅身份键、后像全行；delete 前像。
	ins := events[0]
	if v, _ := ins.After["name"], ins.After["gold"]; string(v) != `"sword"` {
		t.Fatalf("insert after.name = %s", v)
	}
	if got := string(ins.After["gold"]); got != "100" {
		t.Fatalf("insert after.gold = %s, want numeric passthrough 100", got)
	}
	if got, ok := ins.After["blob"]; !ok || string(got) != "null" {
		t.Fatalf("insert after.blob = %v,%v want explicit null", got, ok)
	}
	if ins.Before != nil {
		t.Fatalf("insert before = %v, want nil", ins.Before)
	}

	upd := events[1]
	if len(upd.Before) != 1 || string(upd.Before["id"]) != "1" {
		t.Fatalf("update(K) before = %v, want key-only {id:1}", upd.Before)
	}
	if string(upd.After["name"]) != `"sword2"` || string(upd.After["gold"]) != "150" {
		t.Fatalf("update after mismatch: %v", upd.After)
	}

	updNoOld := events[2]
	if updNoOld.Before != nil {
		t.Fatalf("update no-old before = %v, want nil", updNoOld.Before)
	}

	del := events[3]
	if len(del.Before) != 1 || del.After != nil {
		t.Fatalf("delete images mismatch: before=%v after=%v", del.Before, del.After)
	}

	if got := d.Checkpoint(); got.Kind != "lsn" || got.Value != (LSN(0x10)<<32|0xABC).Decimal() {
		t.Fatalf("checkpoint = %+v", got)
	}
}

// 值类型化：数值直通为 JSON 数字、bool 译真假、NaN 等非 JSON 字面量回退
// 字符串（不伪造数字）、文本一律 JSON 字符串。
func TestDecoderValueTyping(t *testing.T) {
	d := NewDecoder("s")
	mustFeed(t, d, wireMessage('R', relPayload(1, "g", "t", 'd',
		relCol(FlagReplicaIdentity, "id", 23),
		relCol(0, "f8", 701),
		relCol(0, "flag", 16),
		relCol(0, "note", 25),
	)))
	mustFeed(t, d, beginMsg(1))
	mustFeed(t, d, wireMessage('I', append(append(u32v(1), 'N'),
		tupleData(textDatum("1"), textDatum("NaN"), textDatum("t"), textDatum("hi 你好"))...)))
	events := mustFeed(t, d, commitMsg(100))
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	img := events[0].After
	if got := string(img["id"]); got != "1" {
		t.Fatalf("id = %s, want raw number", got)
	}
	if got := string(img["f8"]); got != `"NaN"` {
		t.Fatalf("f8 = %s, want quoted string fallback", got)
	}
	if got := string(img["flag"]); got != "true" {
		t.Fatalf("flag = %s, want true", got)
	}
	if got := string(img["note"]); got != `"hi 你好"` {
		t.Fatalf("note = %s, want quoted string", got)
	}
}

// bool datum 非法值报错（防御性：PG 只发 t/f）。
func TestDecoderInvalidBoolDatum(t *testing.T) {
	d := NewDecoder("s")
	mustFeed(t, d, wireMessage('R', relPayload(1, "g", "t", 'd',
		relCol(FlagReplicaIdentity, "id", 23), relCol(0, "flag", 16))))
	mustFeed(t, d, beginMsg(1))
	_, err := d.Feed(wireMessage('I', append(append(u32v(1), 'N'),
		tupleData(textDatum("1"), textDatum("yes"))...)))
	if err == nil || !strings.Contains(err.Error(), "bool") {
		t.Fatalf("invalid bool datum err = %v", err)
	}
}

// 二进制 datum：本解码层只申请 text 模式，遇到 'b' 报错而非瞎猜。
func TestDecoderBinaryDatumRejected(t *testing.T) {
	d := NewDecoder("s")
	mustFeed(t, d, wireMessage('R', relPayload(1, "g", "t", 'd',
		relCol(FlagReplicaIdentity, "id", 23))))
	mustFeed(t, d, beginMsg(1))
	_, err := d.Feed(wireMessage('I', append(append(u32v(1), 'N'),
		tupleData(binaryDatum("\x01\x02"))...)))
	if err == nil || !strings.Contains(err.Error(), "binary") {
		t.Fatalf("binary datum err = %v", err)
	}
}

// 副本身份 'n'：delete 旧元组全 'u' → Before 不可得置 nil（事件仍合法，
// 契约允许 delete 单像缺失）。
func TestDecoderDeleteWithoutIdentity(t *testing.T) {
	d := NewDecoder("s")
	mustFeed(t, d, itemsRelation('n'))
	mustFeed(t, d, beginMsg(5))
	mustFeed(t, d, deleteMsg(tupleData(unchangedDatum(), unchangedDatum(), unchangedDatum(), unchangedDatum())))
	events := mustFeed(t, d, commitMsg(50))
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].Operation != connector.OpDelete || events[0].Before != nil {
		t.Fatalf("delete without identity mismatch: %+v", events[0])
	}
	if err := (&events[0]).Validate(); err != nil {
		t.Fatalf("delete event invalid: %v", err)
	}
}

// update 双像全空（新元组全 'u' 且无旧像）= 契约违例：解码器产出自检
// 拦截，不下发坏事件。
func TestDecoderUpdateWithoutImagesRejected(t *testing.T) {
	d := NewDecoder("s")
	mustFeed(t, d, itemsRelation('n'))
	mustFeed(t, d, beginMsg(5))
	_, err := d.Feed(updateNoOldMsg(tupleData(unchangedDatum(), unchangedDatum(), unchangedDatum(), unchangedDatum())))
	if err == nil || !strings.Contains(err.Error(), "neither before nor after") {
		t.Fatalf("update without images err = %v", err)
	}
}

// TOAST 未重写列：UPDATE 新元组里未重写列（'u'）行像缺键（非 null），
// 其余列不受影响——真实场景：UPDATE 只改 gold，name/blob 未动。
func TestDecoderToastUnchangedOmitted(t *testing.T) {
	d := NewDecoder("s")
	mustFeed(t, d, itemsRelation('d'))
	mustFeed(t, d, beginMsg(9))
	mustFeed(t, d, updateNoOldMsg(tupleData(
		textDatum("1"), unchangedDatum(), textDatum("150"), unchangedDatum(),
	)))
	events := mustFeed(t, d, commitMsg(90))
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	img := events[0].After
	if _, ok := img["name"]; ok {
		t.Fatalf("after.name present, want omitted (toast unchanged)")
	}
	if _, ok := img["blob"]; ok {
		t.Fatalf("after.blob present, want omitted (toast unchanged)")
	}
	if got := string(img["gold"]); got != "150" {
		t.Fatalf("after.gold = %s, want 150", got)
	}
	if got := string(img["id"]); got != "1" {
		t.Fatalf("after.id = %s, want 1", got)
	}
}

// 结构修订：首见 revision=1；相同结构重发不 bump；列集变化 bump →
// SchemaVersion 漂移信号。
func TestDecoderRelationRevisionBump(t *testing.T) {
	d := NewDecoder("s")

	// n=列数（DDL 加列后 insert 元组同步加宽）。
	feedInsert := func(n int) int64 {
		tup := [][]byte{textDatum("1"), textDatum("n"), textDatum("0"), nullDatum()}
		for i := 4; i < n; i++ {
			tup = append(tup, textDatum("x"))
		}
		mustFeed(t, d, beginMsg(1))
		mustFeed(t, d, insertMsg(tup...))
		events := mustFeed(t, d, commitMsg(10))
		if len(events) != 1 {
			t.Fatalf("events = %d", len(events))
		}
		return events[0].SchemaVersion
	}

	mustFeed(t, d, itemsRelation('d'))
	if v := feedInsert(4); v != 1 {
		t.Fatalf("first revision = %d, want 1", v)
	}
	mustFeed(t, d, itemsRelation('d')) // 相同结构重发（PG 会周期性重发）
	if v := feedInsert(4); v != 1 {
		t.Fatalf("stable revision = %d, want 1", v)
	}
	changed := wireMessage('R', relPayload(itemsOID, "game", "items", 'd',
		relCol(FlagReplicaIdentity, "id", 23),
		relCol(0, "name", 25),
		relCol(0, "gold", 1700),
		relCol(0, "blob", 25),
		relCol(0, "extra", 25), // DDL：新列
	))
	mustFeed(t, d, changed)
	if v := feedInsert(5); v != 2 {
		t.Fatalf("bumped revision = %d, want 2", v)
	}
}

// 协议违例与漂移逐项报错。
func TestDecoderProtocolErrors(t *testing.T) {
	t.Run("行消息在事务外", func(t *testing.T) {
		d := NewDecoder("s")
		if _, err := d.Feed(insertMsg(textDatum("1"))); err == nil || !strings.Contains(err.Error(), "outside transaction") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("行消息先于 Relation", func(t *testing.T) {
		d := NewDecoder("s")
		mustFeed(t, d, beginMsg(1))
		_, err := d.Feed(insertMsg(textDatum("1")))
		if err == nil || !strings.Contains(err.Error(), "before its relation message") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("事务内二次 Begin", func(t *testing.T) {
		d := NewDecoder("s")
		mustFeed(t, d, beginMsg(1))
		_, err := d.Feed(beginMsg(2))
		if err == nil || !strings.Contains(err.Error(), "inside transaction") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("无 Begin 的 Commit", func(t *testing.T) {
		d := NewDecoder("s")
		_, err := d.Feed(commitMsg(1))
		if err == nil || !strings.Contains(err.Error(), "without begin") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("未知消息类型", func(t *testing.T) {
		d := NewDecoder("s")
		_, err := d.Feed([]byte{'Z', 0, 1})
		if err == nil || !strings.Contains(err.Error(), "unknown pgoutput message type") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("空负载", func(t *testing.T) {
		d := NewDecoder("s")
		if _, err := d.Feed(nil); err == nil {
			t.Fatal("empty payload accepted")
		}
	})
	t.Run("元组列数与 Relation 不符", func(t *testing.T) {
		d := NewDecoder("s")
		mustFeed(t, d, itemsRelation('d'))
		mustFeed(t, d, beginMsg(1))
		_, err := d.Feed(insertMsg(textDatum("1"))) // 少 3 列
		if err == nil || !strings.Contains(err.Error(), "columns") {
			t.Fatalf("err = %v", err)
		}
	})
}

// 提交时间换算：PG 纪元微秒 → Unix 毫秒（与 PG 服务器时间对齐）。
func TestDecoderCommitTimestamp(t *testing.T) {
	d := NewDecoder("s")
	mustFeed(t, d, itemsRelation('d'))
	mustFeed(t, d, beginMsg(1))
	mustFeed(t, d, insertMsg(textDatum("1"), textDatum("n"), textDatum("0"), nullDatum()))
	events := mustFeed(t, d, commitMsg(42))
	want := time.UnixMilli(1748736000000).UTC()
	got := time.UnixMilli(events[0].TsUnixMs).UTC()
	if !got.Equal(want) {
		t.Fatalf("commit ts = %v, want %v", got, want)
	}
	// 行像值 JSON 可反序列化（契约面 RawMessage 完整性）。
	var row map[string]json.RawMessage
	if err := json.Unmarshal(mustJSON(t, events[0].After), &row); err != nil {
		t.Fatalf("after image not valid JSON object: %v", err)
	}
}

func mustJSON(t *testing.T, m map[string]json.RawMessage) []byte {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal image: %v", err)
	}
	return data
}
