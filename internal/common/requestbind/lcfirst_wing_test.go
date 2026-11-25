// Coverage sweep round 18 (wt-api): closing out the lcFirst empty-string early
// return wing in query.go (107-108) — an empty tag name does not enter the
// rune slice and is returned as-is.
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
