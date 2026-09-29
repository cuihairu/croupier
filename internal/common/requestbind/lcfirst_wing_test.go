// 覆盖率巡检第十八轮（wt-api）：query.go lcFirst 空串早退翼
// （107-108）收口——空 tag 名不进 rune 切片、原样返回。
package requestbind

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLcFirst_EmptyStringEarlyReturn(t *testing.T) {
	assert.Empty(t, lcFirst(""), "空串早退，不构造 rune 切片")
	assert.Equal(t, "gameId", lcFirst("GameId"))
	assert.Equal(t, "aB", lcFirst("AB"), "仅首字符折叠，其余保留")
}
