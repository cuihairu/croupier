package filter

// Validate 全臂（合法放行 / 多星拒收按字段名分派 / 操作闭集外）、
// componentMatch 的多星拒收、PushdownTables 的空 include 与来源不匹配
// 臂，以及 Events 的两处 ctx 取消臂（外层接收取消 / 输出阻塞取消）。
//
// 无诚实跳过：纯函数与小状态面，全臂可达。

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/stretchr/testify/require"
)

func TestValidateArms(t *testing.T) {
	// 合法选择器 → 放行
	require.NoError(t, Filter{Include: []Selector{
		{SourceID: "main", SourceType: "mysql", Database: "db", Table: "items",
			Operations: []cdc.Operation{cdc.OpInsert, cdc.OpUpdate, cdc.OpDelete}},
	}, Exclude: []Selector{{Table: "aud*"}}}.Validate())

	// 双星：按字段名分派（每选择器仅一个字段有值 → 错误信息确定）
	for _, pair := range []struct{ field, value string }{
		{"sourceId", "a**b"},
		{"sourceType", "a**b"},
		{"database", "a**b"},
		{"table", "a**b"},
	} {
		only := Selector{}
		switch pair.field {
		case "sourceId":
			only.SourceID = pair.value
		case "sourceType":
			only.SourceType = pair.value
		case "database":
			only.Database = pair.value
		case "table":
			only.Table = pair.value
		}
		err := Filter{Include: []Selector{only}}.Validate()
		require.ErrorContains(t, err, "filter: "+pair.field, "field %s", pair.field)
		require.ErrorContains(t, err, "at most one '*' wildcard")

		// Exclude 侧同样校验
		err = Filter{Exclude: []Selector{only}}.Validate()
		require.ErrorContains(t, err, "at most one '*' wildcard")
	}

	// 操作闭集外
	err := Filter{Include: []Selector{{
		Operations: []cdc.Operation{cdc.Operation("weird")},
	}}}.Validate()
	require.ErrorContains(t, err, `invalid operation "weird"`)
}

func TestComponentMatchMultiStarRejected(t *testing.T) {
	require.False(t, componentMatch("a*b*c", "abc"), "multi-star must not match")
	require.False(t, componentMatch("a*b*c", "axbyc"))
	require.True(t, componentMatch("*", "anything"))
	require.True(t, componentMatch("", "anything"))
	require.False(t, componentMatch("exact", "other"))
	// 前后缀可交接边界：值恰好等于前后缀拼接
	require.True(t, componentMatch("a*b", "ab"))
	require.True(t, componentMatch("a*b", "axxb"))
	require.False(t, componentMatch("a*b", "bxa"), "order must hold")
	require.False(t, componentMatch("a*b", "a"), "value shorter than parts")
}

func TestPushdownTablesGuards(t *testing.T) {
	// 空 include → 空 allowlist（非 nil）
	require.Equal(t, []string{}, (Filter{}).PushdownTables("main", "mysql"))

	// include 全部来源不匹配 → 空 allowlist
	f := Filter{Include: []Selector{
		{SourceID: "other", Database: "game", Table: "users"},
		{SourceType: "postgres", Database: "game", Table: "sessions"},
	}}
	require.Equal(t, []string{}, f.PushdownTables("main", "mysql"))

	// 通配来源约束放行 + 精确表 → 收敛 allowlist
	f.Include = []Selector{
		{SourceID: "*", Database: "game", Table: "users"},
		{Database: "game", Table: "users"}, // 去重
		{Database: "game", Table: "sessions"},
	}
	require.Equal(t, []string{"game.sessions", "game.users"},
		f.PushdownTables("main", "mysql"))
}

func TestEventsCancellationArms(t *testing.T) {
	f := Filter{Include: []Selector{{Database: "db"}}}

	// 外层接收取消：ctx 已取消、输入永不关闭 → 输出直接关闭
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	blocked := make(chan cdc.ChangeEvent) // 永不关闭、永不投递
	out := f.Events(ctx, blocked)
	select {
	case _, ok := <-out:
		require.False(t, ok, "output must be closed")
	case <-time.After(time.Second):
		t.Fatal("cancelled ctx must close output immediately")
	}

	// 输出阻塞取消：匹配事件已取出、无读者 → 发送阻塞 → 取消后退出
	ctx2, cancel2 := context.WithCancel(context.Background())
	in := make(chan cdc.ChangeEvent, 1)
	in <- cdc.ChangeEvent{Database: "db", Table: "users", Operation: cdc.OpInsert}
	out2 := f.Events(ctx2, in)
	time.Sleep(30 * time.Millisecond) // 等 goroutine 卡在 output 发送
	cancel2()
	select {
	case _, ok := <-out2:
		require.False(t, ok, "output must be closed after cancel")
	case <-time.After(time.Second):
		t.Fatal("cancel while blocked on send must close output")
	}
}
