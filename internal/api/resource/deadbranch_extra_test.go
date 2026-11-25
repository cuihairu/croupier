package resource

import (
	"net/http"
	"net/http/httptest"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below document unreachable defensive branches in this package
// (matching the convention of internal/analytics/mq/deadbranch_doc_test.go):
//
//  1. handler.go List (:23) error branch: ResourceListRequest holds only the
//     Category/Query string form fields, and ShouldBindQuery always succeeds
//     for pure strings, so the error branch cannot be triggered.
//  2. service.go loadPersistentResources (:96): ListByScope uses GORM
//     Find(&[]*ResourceCapability), whose result elements are never nil, so
//     the cap==nil branch cannot be triggered.
//  3. service.go humanizeKey (:289): strings.FieldsFunc never produces empty
//     string segments, so the parts[i]=="" branch cannot be triggered.

func TestHandlerListQueryBindNeverFailsV11(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")

	// Pure string form fields never fail to bind for any query string (odd
	// encodings / duplicate params included).
	for _, target := range []string{
		"/api/v1/resources?category=player&q=guild",
		"/api/v1/resources?category=%E4%B8&category=x&q[]=",
		"/api/v1/resources?=v&category",
	} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest(http.MethodGet, target, nil)
		c.Request = req.WithContext(ctx)

		var reqDTO ResourceListRequest
		require.NoError(t, c.ShouldBindQuery(&reqDTO), "string-only form binding must never fail: %s", target)
	}
}

func TestHumanizeKeyNeverSeesEmptyPartsV11(t *testing.T) {
	assert.Equal(t, "", humanizeKey("...___---"))
	assert.Equal(t, "Player Guild", humanizeKey("player.guild"))
	assert.Equal(t, "A B", humanizeKey("a--b"))
}
