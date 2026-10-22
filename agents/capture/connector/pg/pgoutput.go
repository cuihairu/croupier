package pg

import (
	"encoding/binary"
	"fmt"
)

// pgoutput 协议消息解析（二进制负载，不含首字节消息类型——调用方按类型分发
// 后把余下负载传入；本文件只做无损解码，事件组装在 decoder.go）。
//
// 布局依据 PG 官方文档 pgoutput v1；多字节序为大端。cstring 以 NUL 结尾。

// TupleColumnKind 元组列值形态（pgoutput 'n'/'u'/'t'/'b' 四态）。
const (
	tupleNull      byte = 'n' // 值为 SQL NULL
	tupleUnchanged byte = 'u' // TOAST 未重写（本消息不携带该列值）
	tupleText      byte = 't' // 文本格式值
	tupleBinary    byte = 'b' // 二进制格式值（本解码层不申请 binary 模式，防御性报错）
)

// FlagReplicaIdentity Relation 列标志：该列属于副本身份键（主键/身份索引）。
const FlagReplicaIdentity byte = 1

// Relation 表结构（pgoutput Relation 消息）。列名可用 ⇒ 解出的事件
// ColumnsMapped=true（与 MySQL binlog 裸位置下标的关键差异）。
type Relation struct {
	OID             uint32
	Schema          string
	Name            string
	ReplicaIdentity byte // 'd' 默认(身份索引) / 'n' 无 / 'f' 全行 / 'i' 索引 / ' ' 未知
	Columns         []RelationColumn
}

// RelationColumn 列元数据。
type RelationColumn struct {
	Flags   byte
	Name    string
	TypeOID uint32
	TypeMod int32
}

// KeyColumns 副本身份列名集（对账关联键保护消费，过滤策略简档 §6）。
// 身份为 'f'（全行）或 'n'（无）时无键信息，返回 nil——不猜。
func (r *Relation) KeyColumns() []string {
	if r.ReplicaIdentity != 'd' && r.ReplicaIdentity != 'i' {
		return nil
	}
	var keys []string
	for _, c := range r.Columns {
		if c.Flags&FlagReplicaIdentity != 0 {
			keys = append(keys, c.Name)
		}
	}
	return keys
}

// parseCString 读 NUL 结尾串并推进游标。
func parseCString(b []byte, off int) (string, int, error) {
	end := off
	for end < len(b) && b[end] != 0 {
		end++
	}
	if end >= len(b) {
		return "", off, fmt.Errorf("pg: unterminated cstring at offset %d", off)
	}
	return string(b[off:end]), end + 1, nil
}

// ParseRelation 解析 Relation 消息负载。
func ParseRelation(payload []byte) (*Relation, error) {
	if len(payload) < 10 {
		return nil, fmt.Errorf("pg: relation payload too short: %d", len(payload))
	}
	off := 0
	r := &Relation{OID: binary.BigEndian.Uint32(payload)}
	off += 4
	var err error
	if r.Schema, off, err = parseCString(payload, off); err != nil {
		return nil, fmt.Errorf("pg: relation schema: %w", err)
	}
	if r.Name, off, err = parseCString(payload, off); err != nil {
		return nil, fmt.Errorf("pg: relation name: %w", err)
	}
	r.ReplicaIdentity = payload[off]
	off++
	ncols := int(binary.BigEndian.Uint16(payload[off:]))
	off += 2
	if ncols > len(payload) { // 防御：列数上限不会超过负载字节数
		return nil, fmt.Errorf("pg: relation %s.%s column count %d exceeds payload", r.Schema, r.Name, ncols)
	}
	r.Columns = make([]RelationColumn, 0, ncols)
	for i := 0; i < ncols; i++ {
		c := RelationColumn{}
		if off+6 > len(payload) {
			return nil, fmt.Errorf("pg: relation %s.%s column %d truncated", r.Schema, r.Name, i)
		}
		c.Flags = payload[off]
		off++
		if c.Name, off, err = parseCString(payload, off); err != nil {
			return nil, fmt.Errorf("pg: relation %s.%s column %d name: %w", r.Schema, r.Name, i, err)
		}
		if off+8 > len(payload) {
			return nil, fmt.Errorf("pg: relation %s.%s column %d meta truncated", r.Schema, r.Name, i)
		}
		c.TypeOID = binary.BigEndian.Uint32(payload[off:])
		off += 4
		c.TypeMod = int32(binary.BigEndian.Uint32(payload[off:]))
		off += 4
		r.Columns = append(r.Columns, c)
	}
	return r, nil
}

// Tuple 行像元组。
type Tuple struct {
	Columns []TupleColumn
}

// TupleColumn 单列值（Kind ∈ n/u/t/b；Text 仅 t 态有值）。
type TupleColumn struct {
	Kind byte
	Text []byte
}

// ParseTuple 解析 TupleData：uint16 列数 + 每列 [kind, t/b 另带 int32 长度+字节]。
func ParseTuple(payload []byte) (*Tuple, error) {
	if len(payload) < 2 {
		return nil, fmt.Errorf("pg: tuple payload too short: %d", len(payload))
	}
	n := int(binary.BigEndian.Uint16(payload))
	t := &Tuple{Columns: make([]TupleColumn, 0, n)}
	off := 2
	for i := 0; i < n; i++ {
		if off >= len(payload) {
			return nil, fmt.Errorf("pg: tuple column %d truncated", i)
		}
		c := TupleColumn{Kind: payload[off]}
		off++
		switch c.Kind {
		case tupleNull, tupleUnchanged:
			// 无随附值
		case tupleText, tupleBinary:
			if off+4 > len(payload) {
				return nil, fmt.Errorf("pg: tuple column %d length truncated", i)
			}
			sz := int(binary.BigEndian.Uint32(payload[off:]))
			off += 4
			if off+sz > len(payload) {
				return nil, fmt.Errorf("pg: tuple column %d data truncated", i)
			}
			c.Text = payload[off : off+sz]
			off += sz
		default:
			return nil, fmt.Errorf("pg: tuple column %d unknown kind %q", i, c.Kind)
		}
		t.Columns = append(t.Columns, c)
	}
	return t, nil
}

// BeginMsg 事务开始（XID 是 PG 事务标识）。
type BeginMsg struct {
	XID uint32
}

// ParseBegin 解析 Begin：int64 finalLSN + int64 commitTS + int32 xid（前两者
// 由 decoder 的 Commit 侧取位点/时间，这里只取 xid）。
func ParseBegin(payload []byte) (BeginMsg, error) {
	if len(payload) < 20 {
		return BeginMsg{}, fmt.Errorf("pg: begin payload too short: %d", len(payload))
	}
	return BeginMsg{XID: binary.BigEndian.Uint32(payload[16:])}, nil
}

// CommitMsg 事务提交。EndLSN 是提交完成后的下一位点（resume 起点：从该位点
// 重启整事务原子重放）。
type CommitMsg struct {
	EndLSN LSN
	// TsUnixMs 提交时间（PG epoch 微秒 → Unix 毫秒）。
	TsUnixMs int64
}

// pgEpochUnixMs PG 内部时间纪元（2000-01-01 UTC）的 Unix 毫秒。
const pgEpochUnixMs = int64(946684800) * 1000

// ParseCommit 解析 Commit：uint8 flags + int64 commitLSN + int64 endLSN +
// int64 commitTS（微秒，PG 纪元）。
func ParseCommit(payload []byte) (CommitMsg, error) {
	if len(payload) < 25 {
		return CommitMsg{}, fmt.Errorf("pg: commit payload too short: %d", len(payload))
	}
	return CommitMsg{
		EndLSN:   LSN(binary.BigEndian.Uint64(payload[9:17])),
		TsUnixMs: pgEpochUnixMs + int64(binary.BigEndian.Uint64(payload[17:25]))/1000,
	}, nil
}

// InsertMsg 插入：int32 relationOID + 'N' + tuple。
type InsertMsg struct {
	RelationOID uint32
	New         *Tuple
}

// ParseInsert 解析 Insert 负载。
func ParseInsert(payload []byte) (*InsertMsg, error) {
	if len(payload) < 5 {
		return nil, fmt.Errorf("pg: insert payload too short: %d", len(payload))
	}
	m := &InsertMsg{RelationOID: binary.BigEndian.Uint32(payload)}
	if payload[4] != 'N' {
		return nil, fmt.Errorf("pg: insert missing 'N' marker, got %q", payload[4])
	}
	t, err := ParseTuple(payload[5:])
	if err != nil {
		return nil, fmt.Errorf("pg: insert tuple: %w", err)
	}
	m.New = t
	return m, nil
}

// UpdateMsg 更新：int32 relationOID + [ 'K'|'O' + 旧元组 ] + 'N' + 新元组。
// 旧元组仅在副本身份提供时出现（'K'=身份键列，'O'=全行）；缺失 = Before 不
// 可得（能力诚实口径，decoder 置 nil 不伪造）。
type UpdateMsg struct {
	RelationOID uint32
	Old         *Tuple // nil=源端未提供
	OldIsKey    bool   // true=Old 来自 'K'（仅身份键列，其余列标 'u'）
	New         *Tuple
}

// ParseUpdate 解析 Update 负载。
func ParseUpdate(payload []byte) (*UpdateMsg, error) {
	if len(payload) < 5 {
		return nil, fmt.Errorf("pg: update payload too short: %d", len(payload))
	}
	off := 4
	m := &UpdateMsg{RelationOID: binary.BigEndian.Uint32(payload)}
	switch payload[off] {
	case 'K':
		m.OldIsKey = true
		off++
		t, err := ParseTuple(payload[off:])
		if err != nil {
			return nil, fmt.Errorf("pg: update key tuple: %w", err)
		}
		m.Old = t
		off += tupleSize(payload[off:])
		if off >= len(payload) || payload[off] != 'N' {
			return nil, fmt.Errorf("pg: update missing new-tuple 'N' marker")
		}
		off++
	case 'O':
		off++
		t, err := ParseTuple(payload[off:])
		if err != nil {
			return nil, fmt.Errorf("pg: update old tuple: %w", err)
		}
		m.Old = t
		off += tupleSize(payload[off:])
		if off >= len(payload) || payload[off] != 'N' {
			return nil, fmt.Errorf("pg: update missing new-tuple 'N' marker")
		}
		off++
	case 'N':
		off++
	default:
		return nil, fmt.Errorf("pg: update unexpected marker %q", payload[off])
	}
	t, err := ParseTuple(payload[off:])
	if err != nil {
		return nil, fmt.Errorf("pg: update new tuple: %w", err)
	}
	m.New = t
	return m, nil
}

// tupleSize 计算从 b 起始的 TupleData 总字节数（解析一次数长度，供
// 「同负载双元组」场景推进游标）。
func tupleSize(b []byte) int {
	t, err := ParseTuple(b)
	if err != nil {
		return 0
	}
	off := 2
	for _, c := range t.Columns {
		off++ // kind
		if c.Kind == tupleText || c.Kind == tupleBinary {
			off += 4 + len(c.Text)
		}
	}
	return off
}

// DeleteMsg 删除：int32 relationOID + 'K'|'O' + 旧元组（副本身份 'n' 时源端
// 仍会发旧元组但全列为 'u'/'n'——Before 不可得，decoder 置 nil）。
type DeleteMsg struct {
	RelationOID uint32
	OldIsKey    bool
	Old         *Tuple
}

// ParseDelete 解析 Delete 负载。
func ParseDelete(payload []byte) (*DeleteMsg, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("pg: delete payload too short: %d", len(payload))
	}
	m := &DeleteMsg{RelationOID: binary.BigEndian.Uint32(payload)}
	switch payload[4] {
	case 'K':
		m.OldIsKey = true
	case 'O':
	default:
		return nil, fmt.Errorf("pg: delete unexpected marker %q", payload[4])
	}
	t, err := ParseTuple(payload[5:])
	if err != nil {
		return nil, fmt.Errorf("pg: delete old tuple: %w", err)
	}
	m.Old = t
	return m, nil
}

// TruncateMsg 截断（connector.Operation 闭集无 truncate——decoder 跳过并
// 计数，见 decoder.go；此处保留解析供测试锁定布局）。
type TruncateMsg struct {
	RelationOIDs []uint32
}

// ParseTruncate 解析 Truncate：int32 关系数 + uint8 flags + int32 oid 列表。
func ParseTruncate(payload []byte) (*TruncateMsg, error) {
	if len(payload) < 5 {
		return nil, fmt.Errorf("pg: truncate payload too short: %d", len(payload))
	}
	n := int(binary.BigEndian.Uint32(payload))
	if len(payload) < 5+n*4 {
		return nil, fmt.Errorf("pg: truncate relation list truncated: want %d oids, payload %d bytes", n, len(payload))
	}
	m := &TruncateMsg{RelationOIDs: make([]uint32, 0, n)}
	off := 5 // 跳过 flags 字节
	for i := 0; i < n; i++ {
		m.RelationOIDs = append(m.RelationOIDs, binary.BigEndian.Uint32(payload[off:]))
		off += 4
	}
	return m, nil
}
