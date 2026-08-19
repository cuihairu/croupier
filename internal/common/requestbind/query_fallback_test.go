package requestbind

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBindContext(t *testing.T, query string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/x?"+query, nil)
	ctx.Request = req
	return ctx
}

type fullFallbackDTO struct {
	Trigger int      `form:"trigger"`
	Name    string   `json:"name"`
	Count   int64    `json:"count"`
	BadInt  int      `json:"badInt"`
	Flag    bool     `json:"flag"`
	BadFlag bool     `json:"badFlag"`
	Tags    []string `json:"tags"`
	Nums    []int    `json:"nums"`
	Missing string   `json:"absent"`
	Ignored string   `json:"-"`
	hidden  string
}

func TestBindQueryCompat_FallbackCoversAllFieldKinds(t *testing.T) {
	// trigger=NaN makes gin's form binding fail, forcing the JSON-tag fallback.
	ctx := newBindContext(t, "trigger=notanum&name=alice&count=42&badInt=zzz&flag=1&badFlag=maybe&tags=a&tags=b&nums=1&nums=2")

	var req fullFallbackDTO
	err := BindQueryCompat(ctx, &req)
	require.NoError(t, err)

	assert.Equal(t, "alice", req.Name)
	assert.Equal(t, int64(42), req.Count)
	assert.Equal(t, 0, req.BadInt, "unparseable int must stay zero")
	assert.True(t, req.Flag)
	assert.False(t, req.BadFlag, "unparseable bool must stay false")
	assert.Equal(t, []string{"a", "b"}, req.Tags)
	assert.Empty(t, req.Nums, "non-string slices must not be bound")
	assert.Empty(t, req.Missing)
	assert.Empty(t, req.Ignored)
}

func TestBindQueryCompat_FallbackPointerToNonStruct(t *testing.T) {
	ctx := newBindContext(t, "")

	n := 5
	err := BindQueryCompat(ctx, &n)
	// Either gin's form binding rejects the non-struct and the fallback
	// validates it, or the initial bind succeeds: no panic either way.
	_ = err
	assert.Equal(t, 5, n)
}

func TestBindQueryCompat_NilValidator(t *testing.T) {
	ctx := newBindContext(t, "trigger=bad&name=bob")

	orig := binding.Validator
	binding.Validator = nil
	defer func() { binding.Validator = orig }()

	var req fullFallbackDTO
	require.NoError(t, BindQueryCompat(ctx, &req))
	assert.Equal(t, "bob", req.Name)
}

func TestBindQueryCompat_QueryValuePresentButEmpty(t *testing.T) {
	type emptyValue struct {
		Boom  int    `form:"boom"`
		Title string `json:"title"`
	}
	// boom= (empty string) fails int conversion in gin, empty title value is
	// present but blank.
	ctx := newBindContext(t, "boom=&title=")

	var req emptyValue
	require.NoError(t, BindQueryCompat(ctx, &req))
	assert.Empty(t, req.Title)
}

func TestBindQueryCompat_TrailingTagOptions(t *testing.T) {
	type opts struct {
		Boom int    `form:"boom"`
		Name string `json:"name,omitempty"`
	}
	ctx := newBindContext(t, "boom=oops&name=plain")

	var req opts
	require.NoError(t, BindQueryCompat(ctx, &req))
	assert.Equal(t, "plain", req.Name)
}

func TestTagKey_Whitespace(t *testing.T) {
	assert.Equal(t, "name", tagKey("  name , omitempty "))
	assert.Equal(t, "", tagKey(""))
}

func TestBindQueryCompat_StringSliceSingleValue(t *testing.T) {
	type single struct {
		Boom int      `form:"boom"`
		IDs  []string `json:"ids"`
	}
	ctx := newBindContext(t, "boom=xx&ids=only")

	var req single
	require.NoError(t, BindQueryCompat(ctx, &req))
	assert.Equal(t, []string{"only"}, req.IDs)
}
