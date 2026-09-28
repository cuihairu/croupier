package rbac

// 覆盖率巡检补测：导出包装 SplitLogicalPermission（个人中心权限树消费）
// 此前 0%——内部 splitLogicalPermission 已有测试覆盖，本文件锁定导出层
// 透传语义与文档声明的通配约定一致（通配语义全后端唯一实现点的对外表）。

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitLogicalPermission_ExportedPassthrough(t *testing.T) {
	cases := []struct {
		in           string
		wantResource string
		wantAction   string
	}{
		{"", "*", "*"},
		{"*", "*", "*"},
		{"admin:all", "*", "*"},
		{"players:view", "players", "view"},
		{"players", "players", "*"},
		{"players:", "players", "*"},
		{"players:all", "players", "*"},
		{"  Players:View  ", "players", "view"}, // 大小写与空白归一
	}
	for _, tc := range cases {
		resource, action := SplitLogicalPermission(tc.in)
		assert.Equal(t, tc.wantResource, resource, "resource(%q)", tc.in)
		assert.Equal(t, tc.wantAction, action, "action(%q)", tc.in)
	}
}
