package source

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-mysql-org/go-mysql/replication"
)

// binlogEventStream 是 pump 需要的最小事件流面；*replication.BinlogStreamer
// 在生产路径实现，测试可用内存事件序列复现提交边界。
type binlogEventStream interface {
	GetEvent(context.Context) (*replication.BinlogEvent, error)
}

// pump 消费 binlog 事件流：行事件先按源事务 GTID 暂存，并在提交边界
// （XID/Query）统一补事务 ID、连续序号与末事件标记后，过白名单再进 out。
// GTID 集合只在边界推进，未提交行事件不会提前暴露。致命错误记日志并关闭
// out——上层 runner 感知流关闭后交由 supervisor 拉起恢复。
func (s *MySQLSource) pump(ctx context.Context, streamer binlogEventStream, out chan<- ChangeEvent) {
	defer close(out)
	pending := make([]ChangeEvent, 0)
	currentTxID := ""
	// 放行前先把 GTID 集合推进到提交点（XID/Query 携带的 GSet 已含本事务），
	// 再作为所有已提交事件的位点——emitRows 不再逐事件填 GTID，桥接面
	// （bridgeEvent → Position.Value）与 IdempotencyKey 依赖它。
	flush := func() {
		if len(pending) > 0 {
			gtid := s.gtidSnapshot()
			for index := range pending {
				pending[index].GTID = gtid
			}
			for _, event := range annotateTx(pending, currentTxID) {
				out <- event
			}
		}
		pending = pending[:0]
		currentTxID = ""
	}
	for {
		ev, err := streamer.GetEvent(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return // 主动停机，静默
			default:
			}
			s.log.Error("mysql binlog stream ended", "err", err)
			return
		}

		switch e := ev.Event.(type) {
		case *replication.GTIDEvent:
			if gtid, err := e.GTIDNext(); err == nil && gtid != nil {
				currentTxID = gtid.String()
			}
		case *replication.MariadbGTIDEvent:
			if gtid, err := e.GTIDNext(); err == nil && gtid != nil {
				currentTxID = gtid.String()
			}
		case *replication.QueryEvent: // BEGIN 或 DDL 事务边界
			// BEGIN 只开启事务，位点必须等 XID 提交后再推进；DDL/显式提交
			// 仍按 Query 边界处理。
			if isTransactionBegin(e.Query) {
				continue
			}
			if e.GSet != nil {
				s.setGTID(e.GSet.String())
			}
			flush()
		case *replication.XIDEvent:
			if e.GSet != nil {
				s.setGTID(e.GSet.String())
			}
			flush()
		case *replication.RowsEvent:
			pending = append(pending, s.rowEvents(ev, e)...)
		}
	}
}

// isTransactionBegin 判断 QueryEvent 是否只是事务开启语句。BEGIN 时源已把
// 当前 GTID 放进 GSet，但事务尚未提交，不能据此推进恢复位点。
func isTransactionBegin(query []byte) bool {
	q := strings.ToUpper(strings.TrimSpace(string(query)))
	return q == "BEGIN" || strings.HasPrefix(q, "BEGIN ") ||
		q == "START TRANSACTION" || strings.HasPrefix(q, "START TRANSACTION ")
}

// annotateTx 给事务内待放行的行事件统一补事务 ID、连续序号与末事件标记。
func annotateTx(pending []ChangeEvent, txID string) []ChangeEvent {
	for index := range pending {
		pending[index].TxID = txID
		pending[index].TxSeq = int64(index + 1)
		pending[index].TxEnd = index+1 == len(pending)
	}
	return pending
}

// rowEvents 按事件类型翻译行事件（v1/v2 共用 RowsEvent 载荷），白名单
// 在此提前过滤；事务元数据由泵循环在提交边界统一补齐。
func (s *MySQLSource) rowEvents(ev *replication.BinlogEvent, e *replication.RowsEvent) []ChangeEvent {
	tsUnixMs := int64(ev.Header.Timestamp) * 1000
	serverID := strconv.FormatUint(uint64(ev.Header.ServerID), 10)
	events := make([]ChangeEvent, 0)

	switch ev.Header.EventType {
	case replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
		events = append(events, s.emitRows(e, e.Rows, nil, OpInsert, tsUnixMs, serverID)...)
	case replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
		events = append(events, s.emitRows(e, e.Rows, nil, OpDelete, tsUnixMs, serverID)...)
	case replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:
		// UPDATE 行像成对：Rows[i] = [before, after]。
		for _, pair := range e.Rows {
			if len(pair) != 2 {
				continue
			}
			before, okB := pair[0].([]interface{})
			after, okA := pair[1].([]interface{})
			if !okB || !okA {
				continue
			}
			events = append(events, s.emitRows(e, [][]interface{}{after}, [][]interface{}{before}, OpUpdate, tsUnixMs, serverID)...)
		}
	}
	return events
}

// emitRows 把行像翻译成未提交的 ChangeEvent（位置数组 JSON，无列名——
// 不引 schema 缓存，C1 边界；白名单过滤后返回）。
func (s *MySQLSource) emitRows(tbl *replication.RowsEvent, afters, befores [][]interface{}, op Op, tsUnixMs int64, serverID string) []ChangeEvent {
	db, table := string(tbl.Table.Schema), string(tbl.Table.Table)
	probe := ChangeEvent{Database: db, Table: table}
	if !probe.MatchesTable(s.cfg.Tables) {
		return nil
	}
	events := make([]ChangeEvent, 0, len(afters))
	for i, after := range afters {
		ce := ChangeEvent{
			SourceType: "mysql",
			Database:   db,
			Table:      table,
			Op:         op,
			AfterJSON:  mustRowJSON(after),
			TsUnixMs:   tsUnixMs,
			ServerID:   serverID,
			// Account：MySQL binlog ROW 事件不携带会话账号（与设计 §3.3
			// 的乐观假设不符，已补文档边界）——血缘闸在 MySQL 侧同样拿不到
			// 原生值，C1 一律空，C2 接线时按降级态处理。
		}
		if i < len(befores) {
			ce.BeforeJSON = mustRowJSON(befores[i])
		}
		events = append(events, ce)
	}
	return events
}

func mustRowJSON(row []interface{}) json.RawMessage {
	data, err := json.Marshal(row)
	if err != nil {
		// 行像值全部来自 go-mysql 解码（int/string/[]byte/time/decimal），不可
		// 能到这；万一异常序列化成错误占位，不让单行毒性阻塞整条流。
		return json.RawMessage(fmt.Sprintf(`{"rowJsonError":%q}`, err.Error()))
	}
	return data
}
