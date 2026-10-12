package pg

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cuihairu/croupier/agents/capture/connector"
)

// Decoder pgoutput 流解码状态机：把逐条 replication 消息翻译成
// connector.ChangeEvent。事务事件缓冲到 Commit 一次吐出（整事务原子，
// 从 commit EndLSN 恢复时重放语义完整）；Relation 消息维护表结构缓存
// （OID → 列名/身份键），并按结构变化推进 SchemaVersion 修订号（DDL
// 漂移检测消费）。
//
// 决策记录（诚实口径）：
//   - TOAST 未重写列（'u'）：行像 map 中**缺键**（不是 null）——值未携带
//     而非值为 NULL；
//   - 副本身份不足（update/delete 无旧像）：Before=nil，能力声明
//     beforeImage=false 是真相源，不伪造；
//   - TRUNCATE：connector.Operation 闭集无对应操作，跳过 + 计数
//     （TruncatedRelations），消费方按可观测性看待；
//   - 账号：pgoutput 不携带会话账号，Account 恒空（契约注释既有口径）。
type Decoder struct {
	sourceID string
	// relations 表结构缓存（wire OID → 解析结果 + 修订号）。
	relations map[uint32]*relationEntry
	revision  int64 // 全局结构修订号（每次结构变化 +1，SchemaVersion 取值）

	cur    *txnAcc
	curLSN LSN // 最近 Commit 的 EndLSN（解码器当前位点快照）
	// TruncatedRelations 已跳过的 TRUNCATE 关系数（可观测性）。
	TruncatedRelations int64
}

// relationEntry 缓存条目：结构 + 首见/变化时分配的修订号。
type relationEntry struct {
	rel      *Relation
	revision int64
}

// txnAcc 进行中事务的事件缓冲。
type txnAcc struct {
	xid    uint32
	events []connector.ChangeEvent
	seq    int64 // 事务内事件序号（Tx.Seq，1 起）
}

// NewDecoder 构造解码器。sourceID 进事件的 SourceID（策略 scope 对齐）。
func NewDecoder(sourceID string) *Decoder {
	return &Decoder{sourceID: sourceID, relations: make(map[uint32]*relationEntry)}
}

// Checkpoint 当前位点（最近提交的 EndLSN 十进制串；尚无提交时零值位点）。
// connector.SourceConnector.Checkpoint 的解码层真值源（流式接线层包装）。
func (d *Decoder) Checkpoint() connector.Position {
	if d.curLSN == 0 {
		return connector.Position{}
	}
	return connector.Position{Kind: "lsn", Value: d.curLSN.Decimal()}
}

// Feed 消费一条 pgoutput 消息负载（含首字节消息类型；对齐 pglogrepl
// XLogData.WALData 载荷形态）。返回提交完成时整段事务的事件（其余消息
// 返回 nil）。解码错误返回 error（调用方按协议致命处理，重连换槽续读）。
func (d *Decoder) Feed(payload []byte) ([]connector.ChangeEvent, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("pg: empty payload")
	}
	typ, body := payload[0], payload[1:]
	switch typ {
	case 'B':
		begin, err := ParseBegin(body)
		if err != nil {
			return nil, fmt.Errorf("pg: begin: %w", err)
		}
		if d.cur != nil {
			return nil, fmt.Errorf("pg: begin inside transaction %d", d.cur.xid)
		}
		d.cur = &txnAcc{xid: begin.XID}
	case 'C':
		commit, err := ParseCommit(body)
		if err != nil {
			return nil, fmt.Errorf("pg: commit: %w", err)
		}
		if d.cur == nil {
			return nil, fmt.Errorf("pg: commit without begin")
		}
		events := d.cur.events
		d.cur = nil
		d.curLSN = commit.EndLSN
		for i := range events {
			events[i].TsUnixMs = commit.TsUnixMs
			events[i].Position = connector.Position{Kind: "lsn", Value: commit.EndLSN.Decimal()}
			if err := events[i].Validate(); err != nil {
				// 解码器产出自检不过 = 本包缺陷（非数据问题），按错误暴露。
				return nil, fmt.Errorf("pg: decoded event %d: %w", i, err)
			}
		}
		return events, nil
	case 'R':
		rel, err := ParseRelation(body)
		if err != nil {
			return nil, fmt.Errorf("pg: relation: %w", err)
		}
		d.cacheRelation(rel)
	case 'I':
		msg, err := ParseInsert(body)
		if err != nil {
			return nil, fmt.Errorf("pg: insert: %w", err)
		}
		if err := d.appendRow(connector.OpInsert, msg.RelationOID, nil, msg.New); err != nil {
			return nil, fmt.Errorf("pg: insert: %w", err)
		}
	case 'U':
		msg, err := ParseUpdate(body)
		if err != nil {
			return nil, fmt.Errorf("pg: update: %w", err)
		}
		if err := d.appendRow(connector.OpUpdate, msg.RelationOID, msg.Old, msg.New); err != nil {
			return nil, fmt.Errorf("pg: update: %w", err)
		}
	case 'D':
		msg, err := ParseDelete(body)
		if err != nil {
			return nil, fmt.Errorf("pg: delete: %w", err)
		}
		if err := d.appendRow(connector.OpDelete, msg.RelationOID, msg.Old, nil); err != nil {
			return nil, fmt.Errorf("pg: delete: %w", err)
		}
	case 'T':
		msg, err := ParseTruncate(body)
		if err != nil {
			return nil, fmt.Errorf("pg: truncate: %w", err)
		}
		// 闭集无 truncate 操作：跳过 + 计数（不伪造 delete，不静默）。
		d.TruncatedRelations += int64(len(msg.RelationOIDs))
	case 'Y', 'O', 'M':
		// Type / Origin / 逻辑消息：与行变更无关，跳过。
	default:
		return nil, fmt.Errorf("pg: unknown pgoutput message type %q", typ)
	}
	return nil, nil
}

// appendRow 行事件入事务缓冲：结构缓存查列名 → 元组译行像 → 契约自检 →
// 入队（位点/时间在 Commit 点统一回填）。
func (d *Decoder) appendRow(op connector.Operation, relOID uint32, old, new *Tuple) error {
	if d.cur == nil {
		return fmt.Errorf("row message outside transaction")
	}
	entry, ok := d.relations[relOID]
	if !ok {
		return fmt.Errorf("relation %d referenced before its relation message", relOID)
	}
	rel := entry.rel

	ce := connector.ChangeEvent{
		SourceID:      d.sourceID,
		SourceType:    "postgres",
		Database:      rel.Schema,
		Table:         rel.Name,
		Operation:     op,
		ColumnsMapped: true, // pgoutput Relation 携带列名（MySQL binlog 裸下标的关键差异）
		Key:           rel.KeyColumns(),
		Tx:            connector.TxInfo{ID: strconv.FormatUint(uint64(d.cur.xid), 10)},
		SchemaVersion: entry.revision,
	}
	var err error
	if op != connector.OpDelete {
		if ce.After, err = d.tupleImage(rel, new); err != nil {
			return err
		}
	}
	if op != connector.OpInsert {
		// 旧像可得性随源端 REPLICA IDENTITY：'K' 身份键元组非键列为 'u'
		// （仅键入像）；缺失（身份 'd' 无索引更新？PG 保证 d/i 发 K、f 发 O、
		// n 不发旧元组）= Before 不可得，nil 不伪造。
		if ce.Before, err = d.tupleImage(rel, old); err != nil {
			return err
		}
	}
	if op == connector.OpUpdate && len(ce.Before) == 0 && len(ce.After) == 0 {
		// 契约自检的行像部分（位点/时间在 Commit 点回填后才跑全量 Validate）：
		// 协议保证 update 至少携带新像，双像全空 = 本包缺陷，按错误暴露。
		return fmt.Errorf("pg: update event has neither before nor after image")
	}
	d.cur.seq++
	ce.Tx.Seq = d.cur.seq
	d.cur.events = append(d.cur.events, ce)
	return nil
}

// tupleImage 元组 → 行像 map（列名 → JSON 值）。'n'=JSON null（值真为
// NULL）；'u'=缺键（TOAST 未重写，值未携带）；空像返回 nil（契约 Before/
// After「拿不到=nil」口径）。元组列数与 Relation 列数不符 = 协议/缓存漂移，
// 报错拒绝。
func (d *Decoder) tupleImage(rel *Relation, t *Tuple) (map[string]json.RawMessage, error) {
	if t == nil {
		return nil, nil
	}
	if len(t.Columns) != len(rel.Columns) {
		return nil, fmt.Errorf("relation %s.%s: tuple has %d columns, relation says %d",
			rel.Schema, rel.Name, len(t.Columns), len(rel.Columns))
	}
	var image map[string]json.RawMessage
	for i, c := range t.Columns {
		switch c.Kind {
		case tupleNull:
			if image == nil {
				image = make(map[string]json.RawMessage, len(rel.Columns))
			}
			image[rel.Columns[i].Name] = json.RawMessage("null")
		case tupleUnchanged:
			// 缺键（非 null）——TOAST 未重写，本消息不携带该列值。
		case tupleText:
			v, err := datumJSON(rel.Columns[i].TypeOID, c.Text)
			if err != nil {
				return nil, fmt.Errorf("relation %s.%s column %s: %w", rel.Schema, rel.Name, rel.Columns[i].Name, err)
			}
			if image == nil {
				image = make(map[string]json.RawMessage, len(rel.Columns))
			}
			image[rel.Columns[i].Name] = v
		case tupleBinary:
			return nil, fmt.Errorf("relation %s.%s column %s: binary datum not supported (text mode only)",
				rel.Schema, rel.Name, rel.Columns[i].Name)
		default:
			return nil, fmt.Errorf("relation %s.%s column %s: unknown tuple kind %q",
				rel.Schema, rel.Name, rel.Columns[i].Name, c.Kind)
		}
	}
	if len(image) == 0 {
		return nil, nil
	}
	return image, nil
}

// numericTypeOIDs 文本值可直通为 JSON 数字的类型（int8/int2/int4/float4/
// float8/numeric）——PG 文本格式数值即其规范十进制串。NaN/Infinity 等非
// JSON 合法字面量回退为字符串（不伪造数字）。
var numericTypeOIDs = map[uint32]bool{
	20:   true, // int8
	21:   true, // int2
	23:   true, // int4
	700:  true, // float4
	701:  true, // float8
	1700: true, // numeric
}

// boolTypeOID bool 类型 OID（文本值 "t"/"f"）。
const boolTypeOID = 16

// datumJSON 文本 datum → JSON 值（按类型 OID）：数值直通、bool 译真假、
// 其余一律 JSON 字符串——无类型推断的形态不猜（能力诚实口径在数据面同
// 样适用）。
func datumJSON(typeOID uint32, raw []byte) (json.RawMessage, error) {
	switch {
	case typeOID == boolTypeOID:
		switch string(raw) {
		case "t":
			return json.RawMessage("true"), nil
		case "f":
			return json.RawMessage("false"), nil
		default:
			return nil, fmt.Errorf("invalid bool datum %q", raw)
		}
	case numericTypeOIDs[typeOID] && json.Valid(raw):
		// 数值文本即 JSON 数字字面量；拷贝脱离底层负载（上游可能复用缓冲）。
		return json.RawMessage(append([]byte(nil), raw...)), nil
	default:
		v, err := json.Marshal(string(raw))
		if err != nil {
			return nil, err // string marshaling 不可能失败；防御性返回
		}
		return v, nil
	}
}

// cacheRelation 结构缓存更新：形状未变复用（修订号稳定）；首见或变化时
// 全局修订号 +1（SchemaVersion 漂移信号）。
func (d *Decoder) cacheRelation(rel *Relation) {
	if cur, ok := d.relations[rel.OID]; ok && relationEqual(cur.rel, rel) {
		return
	}
	d.revision++
	d.relations[rel.OID] = &relationEntry{rel: rel, revision: d.revision}
}

func relationEqual(a, b *Relation) bool {
	if a.OID != b.OID || a.Schema != b.Schema || a.Name != b.Name || a.ReplicaIdentity != b.ReplicaIdentity {
		return false
	}
	if len(a.Columns) != len(b.Columns) {
		return false
	}
	for i := range a.Columns {
		ac, bc := a.Columns[i], b.Columns[i]
		if ac.Flags != bc.Flags || ac.Name != bc.Name || ac.TypeOID != bc.TypeOID || ac.TypeMod != bc.TypeMod {
			return false
		}
	}
	return true
}
