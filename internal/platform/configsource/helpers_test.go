package configsource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 覆盖可测辅助函数（构造器校验/tableAllowed）。

func TestNewDBSourceValidation(t *testing.T) {
	// 缺 driver → 报错
	_, err := newDBSource(map[string]interface{}{})
	assert.Error(t, err)

	// 不支持的 driver → 报错
	_, err = newDBSource(map[string]interface{}{"driver": "oracle"})
	assert.Error(t, err)

	// 缺 dsn → 报错
	_, err = newDBSource(map[string]interface{}{"driver": "mysql"})
	assert.Error(t, err)
}

func TestTableAllowed(t *testing.T) {
	s := &dbSource{}
	// 空 → false
	assert.False(t, s.tableAllowed(""))
	// tables nil → 全部允许
	assert.True(t, s.tableAllowed("any_table"))

	// tables 有值 → 白名单
	s2 := &dbSource{tables: map[string]struct{}{"config": {}, "settings": {}}}
	assert.True(t, s2.tableAllowed("config"))
	assert.False(t, s2.tableAllowed("other"))
}
