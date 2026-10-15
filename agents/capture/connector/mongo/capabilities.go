package mongo

import "github.com/cuihairu/croupier/agents/capture/connector"

// MongoCapabilities 能力声明（诚实边界，server 发布校验消费）：
//   - afterImage：insert/replace 必带全量后像；update 走 updateLookup
//     （行已被删则无可发像 → 连接器跳过，不伪造）。
//   - beforeImage：v1 保守 false——updateLookup 给的是后像不是前像；
//     前像要 changeStreamPreAndPostImages（另开集合，v1 不要求）。声明
//     false 后 gate lint 的能力感知守卫会拒绝依赖前像的规则，如实生效。
//   - transactionMetadata：txnNumber/lsid/clusterTime 原生字段。
//   - snapshot：不做（Snapshot 显式返回 ErrUnsupportedSnapshot）。
//   - resumableCheckpoint：resume token 稳定可持久化（等价 LSN）。
//   - pushdown：pipeline $match 是流级白名单变换，不是订阅级过滤。
func MongoCapabilities() connector.Capabilities {
	return connector.Capabilities{
		AfterImage:          true,
		TransactionMetadata: true,
		ResumableCheckpoint: true,
	}
}
