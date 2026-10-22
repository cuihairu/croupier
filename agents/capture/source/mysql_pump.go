package source

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/go-mysql-org/go-mysql/replication"
)

// pump 消费 binlog 事件流：行事件翻译成 ChangeEvent（白名单过滤后）进 out，
// GTID 集合在事务边界（XID/Query）推进。致命错误记日志并关闭 out——上层
// runner 感知流关闭后交由 supervisor 拉起恢复。
func (s *MySQLSource) pump(ctx context.Context, streamer *replication.BinlogStreamer, out chan<- ChangeEvent) {
	defer close(out)
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
		case *replication.XIDEvent:
			if e.GSet != nil {
				s.setGTID(e.GSet.String())
			}
		case *replication.QueryEvent: // DDL 等语句事务边界
			if e.GSet != nil {
				s.setGTID(e.GSet.String())
			}
		case *replication.RowsEvent:
			s.dispatchRows(ev, e, out)
		}
	}
}

// dispatchRows 按事件类型分发行事件（v1/v2 共用 RowsEvent 载荷）。
func (s *MySQLSource) dispatchRows(ev *replication.BinlogEvent, e *replication.RowsEvent, out chan<- ChangeEvent) {
	tsUnixMs := int64(ev.Header.Timestamp) * 1000
	serverID := strconv.FormatUint(uint64(ev.Header.ServerID), 10)
	gtid := s.gtidSnapshot()

	switch ev.Header.EventType {
	case replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
		s.emit(e, e.Rows, nil, OpInsert, tsUnixMs, serverID, gtid, out)
	case replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
		s.emit(e, e.Rows, nil, OpDelete, tsUnixMs, serverID, gtid, out)
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
			s.emit(e, [][]interface{}{after}, [][]interface{}{before}, OpUpdate, tsUnixMs, serverID, gtid, out)
		}
	}
}

// emit 把行像翻译成 ChangeEvent（位置数组 JSON，无列名——不引 schema 缓存，
// C1 边界；白名单过滤后进 out）。
func (s *MySQLSource) emit(tbl *replication.RowsEvent, afters, befores [][]interface{}, op Op, tsUnixMs int64, serverID, gtid string, out chan<- ChangeEvent) {
	db, table := string(tbl.Table.Schema), string(tbl.Table.Table)
	probe := ChangeEvent{Database: db, Table: table}
	if !probe.MatchesTable(s.cfg.Tables) {
		return
	}
	for i, after := range afters {
		ce := ChangeEvent{
			SourceType: "mysql",
			Database:   db,
			Table:      table,
			Op:         op,
			AfterJSON:  mustRowJSON(after),
			GTID:       gtid,
			TsUnixMs:   tsUnixMs,
			ServerID:   serverID,
			// Account：MySQL binlog ROW 事件不携带会话账号（与设计 §3.3
			// 的乐观假设不符，已补文档边界）——血缘闸在 MySQL 侧同样拿不到
			// 原生值，C1 一律空，C2 接线时按降级态处理。
		}
		if i < len(befores) {
			ce.BeforeJSON = mustRowJSON(befores[i])
		}
		out <- ce
	}
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
