package schema

// schema.go 补测：ColumnNames 全函数（非空与空列集）、Record 的无比较器臂
// 与缺表身份臂、Validate 的乱序错误臂（Record 拒收乱序，乱序链只能同包直
// 构 tables 注入——防御性审计面）、tableSchemaEqual 的身份不匹配 / 列数
// 不匹配与列内容不匹配三臂。
//
// 无诚实跳过：全臂可达（乱序臂经同包直构注入）。

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestColumnNames(t *testing.T) {
	ts := testSchema("db", "items", "id", "name")
	require.Equal(t, []string{"id", "name"}, ts.ColumnNames())
	require.Empty(t, TableSchema{}.ColumnNames())
}

func TestRecordGuardsNoComparatorAndNoIdentity(t *testing.T) {
	// 无比较器 → 拒绝
	h := NewHistory(nil)
	require.ErrorContains(t, h.Record("db.items", 1, "10", testSchema("db", "items", "id")),
		"history has no position comparator")

	// 缺表身份 → 拒绝
	h2 := NewHistory(lsnLessForTest)
	require.ErrorContains(t, h2.Record("db.items", 1, "10",
		TableSchema{Columns: []Column{{Name: "id"}}}),
		"has no table identity")
}

func TestValidateDetectsOutOfOrderChain(t *testing.T) {
	// 乱序链：Record 入口先拒，Validate 作为防御性审计面仅直构可达
	h := &History{
		less: lsnLessForTest,
		tables: map[string][]Version{
			"db.items": {
				{Revision: 2, From: "20"},
				{Revision: 1, From: "10"},
			},
		},
	}
	require.ErrorContains(t, h.Validate(), "positions out of order")

	// 对照：升序链放行
	ok := &History{
		less: lsnLessForTest,
		tables: map[string][]Version{
			"db.items": {
				{Revision: 1, From: "10"},
				{Revision: 2, From: "20"},
			},
		},
	}
	require.NoError(t, ok.Validate())
}

func TestTableSchemaEqualFalseArms(t *testing.T) {
	base := testSchema("db", "items", "id", "name")

	// 身份不匹配（表名不同）
	require.False(t, tableSchemaEqual(base, testSchema("db", "other", "id", "name")))
	// 列数不匹配
	require.False(t, tableSchemaEqual(base, testSchema("db", "items", "id")))
	// 列内容不匹配（同数不同名）
	require.False(t, tableSchemaEqual(base, testSchema("db", "items", "id", "other")))
	// 对照：相等放行
	require.True(t, tableSchemaEqual(base, testSchema("db", "items", "id", "name")))
}
