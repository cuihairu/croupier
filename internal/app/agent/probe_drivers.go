package agent

// DB 探针目标的 driver 由 sql.Open 按名解析，驱动注册必须在 agent 二进制里
// 显式声明（core/healthprobe 保持零外部依赖，见 core/healthprobe/db.go）。
// server 派生的 db 目标把逻辑 driver 映射为 sql driver 名（postgres → pgx）；
// 本地 probe.targets 声明的 db 目标同样走这条路径。
//
// 历史坑：agent 依赖图此前只经 gorm mysql driver 传递注册了 mysql，postgres
// 目标在 sql.Open("pgx", ...) 处直接失败——派生通道上线前必须补齐，否则
// 「归属 postgres 源」的周期探测在 agent 侧永远报不可用。
import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)
