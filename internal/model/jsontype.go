package model

import "github.com/cuihairu/croupier/internal/common/dbtype"

// JSON 是 dbtype.JSON 的别名（方言感知：sqlserver 落 nvarchar(max)，
// mysql JSON / postgres JSONB / sqlite JSON）。实现与迁移背景见
// dbtype 包注释；历史注记：datatypes.JSON 对 sqlserver 无类型映射，
// gorm 驱动回落字面量 json 导致建表失败，故有此类型。
type JSON = dbtype.JSON
