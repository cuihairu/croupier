package pg

import "github.com/cuihairu/croupier/agents/capture/connector"

// PostgresCapabilities 本源的能力自报（connector 简档 §3.1 的 PG logical
// replication 实现面；注册 labels captureConnectorCapabilities 的数据源，
// server 发布校验能力感知 lint 消费）。诚实口径（与 MySQL 侧同规矩）：
//   - afterImage：变更后像恒可得（pgoutput 新元组必发）；
//   - transactionMetadata：xid + 事务边界 + LSN 位点（Begin/Commit 消息）；
//   - resumableCheckpoint：replication slot confirmed_flush_lsn 断点续传；
//   - beforeImage：依赖源端 REPLICA IDENTITY（'d'/'i' 只给身份键列、'f'
//     才给全行、'n' 无旧像）——agent 不可靠感知也不可强制源端配置，从严
//     声明 false（与 MySQL binlog_row_image 同口径）；运行时事件里旧像
//     可得性如实表达（'K' 元组=仅身份键、缺失=nil）；
//   - snapshot：logical replication 无初始快照（历史回补走独立流程，
//     过滤策略简档 §4；Snapshot 实现返回 ErrUnsupportedSnapshot）；
//   - pushdown：无（表过滤是 agent 侧代码过滤，非源端下推）。
func PostgresCapabilities() connector.Capabilities {
	return connector.Capabilities{
		AfterImage:          true,
		TransactionMetadata: true,
		ResumableCheckpoint: true,
	}
}
