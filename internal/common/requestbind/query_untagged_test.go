package requestbind

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 回归（OPEN-ISSUES #5）：无 form/json tag 的结构体上，gin ShouldBindQuery
// 按字段名精确匹配（大小写敏感），前端 lowerCamelCase query key 从未绑定
// 成功；且 string-only 结构 ShouldBindQuery 恒成功，旧代码的反射 fallback
// 永不执行——resource-catalog 的 category/query 过滤因此静默失效。

type untaggedDTO struct {
	GameID   string
	Env      string
	Category string
	Query    string
	ID       string
	Ignored  string `form:"-"`
}

func TestBindQueryCompat_UntaggedFieldsBindLowercaseNames(t *testing.T) {
	ctx := newBindContext(t, "gameId=demo&env=prod&category=inventory&query=sword&id=abc&ignored=nope")

	var req untaggedDTO
	require.NoError(t, BindQueryCompat(ctx, &req))

	assert.Equal(t, "demo", req.GameID, "lcFirst fallback: GameID→gameId")
	assert.Equal(t, "prod", req.Env)
	assert.Equal(t, "inventory", req.Category)
	assert.Equal(t, "sword", req.Query)
	assert.Equal(t, "abc", req.ID, "all-lower fallback: ID→id")
	assert.Empty(t, req.Ignored, `form:"-" must stay unbound`)
}

func TestBindQueryCompat_TaggedFieldsStillWinOverLowercaseFallback(t *testing.T) {
	type aliased struct {
		Category string `form:"cat"`
	}
	ctx := newBindContext(t, "cat=mail&category=player")

	var req aliased
	require.NoError(t, BindQueryCompat(ctx, &req))

	assert.Equal(t, "mail", req.Category, "form tag key takes precedence over field-name fallback")
}
