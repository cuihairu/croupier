package pg

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/pkg/cdc/schema"
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
	// schemaHist 源侧 schema 演进历史：每次结构变化按消息位点记录一个
	// 列型映射版本（DDL→版本化；恢复时按事务位点查询校验）。见 SchemaHistory。
	schemaHist *schema.History

	cur    *txnAcc
	curLSN LSN // 最近 Commit 的 EndLSN（解码器当前位点快照）
	// curBeginLSN 当前事务 Begin 消息位点：pgoutput 的 Relation 消息是
	// catalog 消息，不携带自身 WAL 位点（WALStart=0）——schema 版本的生效
	// 点用所在事务的 Begin 位点兜底定位（见 cacheRelation）。
	curBeginLSN string
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
// schema 历史以 LSN 位点排序（十进制串比较）。
func NewDecoder(sourceID string) *Decoder {
	return &Decoder{
		sourceID:   sourceID,
		relations:  make(map[uint32]*relationEntry),
		schemaHist: schema.NewHistory(lsnLess),
	}
}

// lsnLess 十进制 LSN 串的严格弱序（LSN.Decimal() 无前导零，长度即量级）。
func lsnLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// SchemaHistory 返回本解码器记录的 schema 演进历史（结构变化→列型映射
// 版本，按消息位点排序）。连接器把它接到恢复校验；消费方应使用 Clone()
// 或只读访问，避免与并发 Feed 竞态。
func (d *Decoder) SchemaHistory() *schema.History {
	return d.schemaHist
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
// XLogData.WALData 载荷形态）及其消息位点（XLogData.WALStart 十进制串，
// Relation 消息用它定位 schema 版本生效点；空串=位点未知，schema 历史
// 跳过该版本——不猜）。返回提交完成时整段事务的事件（其余消息返回 nil）。
// 解码错误返回 error（调用方按协议致命处理，重连换槽续读）。
func (d *Decoder) Feed(payload []byte, msgLSN string) ([]connector.ChangeEvent, error) {
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
		d.curBeginLSN = msgLSN // 事务位点（Relation 兜底定位用）
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
		// 事务结束：清空事务 Begin 位点。Relation 是 catalog 消息，不带自身
		// WAL 位点，cacheRelation 用所在事务 Begin 位点兜底；若不清空，事务外
		// 且位点未知的 Relation 会误回退到「上一个事务」的陈旧位点，把新
		// schema 版本记到早于实际变更的位置（Record not-after 断流或恢复时点
		// 误判列集）。事务外且无位点应走 cacheRelation 的「跳过（不猜）」。
		d.curBeginLSN = ""
		if len(events) > 0 {
			events[len(events)-1].Tx.End = true
		}
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
		if err := d.cacheRelation(rel, msgLSN); err != nil {
			return nil, fmt.Errorf("pg: relation %s.%s: %w", rel.Schema, rel.Name, err)
		}
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
// 全局修订号 +1（SchemaVersion 漂移信号），并按消息位点把该版本记入
// schema 历史。pgoutput 的 Relation 是 catalog 消息，自身不带 WAL 位点
// （WALStart=0）——用所在事务的 Begin 位点兜底定位（版本自该事务起生效，
// 语义正确）；事务外且位点仍未知时整体跳过：不记历史、不推进修订号、
// 不更新关系缓存（修订号与历史必须同进同退，否则后续 DML 产出历史解析
// 不到的 SchemaVersion，恢复校验对该表该时点按「无版本」拒绝
// （fail-closed，不猜））。历史链不连续（乱序位点）按协议致命错误暴露，
// 不静默丢版本。
func (d *Decoder) cacheRelation(rel *Relation, lsn string) error {
	if cur, ok := d.relations[rel.OID]; ok && relationEqual(cur.rel, rel) {
		return nil
	}
	if lsn == "" || lsn == "0" {
		lsn = d.curBeginLSN // catalog 消息位点缺失 → 事务位点兜底
	}
	if lsn == "" || lsn == "0" {
		return nil // 事务外且无位点：跳过（不猜）——不推进修订号、不更新关系缓存
	}
	d.revision++
	d.relations[rel.OID] = &relationEntry{rel: rel, revision: d.revision}
	if err := d.schemaHist.Record(rel.Schema+"."+rel.Name, d.revision, lsn, tableSchemaOf(rel)); err != nil {
		return err
	}
	return nil
}

// tableSchemaOf Relation → schema.TableSchema（列型映射；Type 取 wire OID
// 十进制串，TypeMod 透传——恢复侧按同源语义解释，不跨方言猜测）。
func tableSchemaOf(rel *Relation) schema.TableSchema {
	cols := make([]schema.Column, len(rel.Columns))
	for i, c := range rel.Columns {
		cols[i] = schema.Column{
			Name:    c.Name,
			Type:    strconv.FormatUint(uint64(c.TypeOID), 10),
			TypeMod: int64(c.TypeMod),
		}
	}
	return schema.TableSchema{Database: rel.Schema, Table: rel.Name, Columns: cols}
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
