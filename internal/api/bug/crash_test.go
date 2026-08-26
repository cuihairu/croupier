package bug

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newCrashHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	db := newBugTestDB(t)
	return NewHandler(NewService(&svc.ServiceContext{BugModel: model.NewBugModel(db)})), db
}

func crashReq(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestFingerprintStack_NormalizesNoise(t *testing.T) {
	a := fingerprintStack("lua: /game/bag.lua:123: in function 'open'\n0x7ffd1234 bag.lua:45")
	b := fingerprintStack("lua: /game/bag.lua:999: in function 'open'\n0x7ffdabcd bag.lua:77")
	assert.Equal(t, a, b, "line numbers and addresses must not affect the fingerprint")

	c := fingerprintStack("lua: /game/bag.lua:123: in function 'close'\n0x7ffd1234 bag.lua:45")
	assert.NotEqual(t, a, c, "different function must produce a different fingerprint")
}

func TestReportCrash_Aggregation(t *testing.T) {
	h, db := newCrashHandler(t)
	report := func(stack, player string) *ReportCrashResponse {
		c, w := crashReq(http.MethodPost, "/bugs/crash", fmt.Sprintf(
			`{"gameId":"demo","env":"prod","platform":"ios","playerId":%q,"appVersion":"1.5.0","stack":%q,"message":"bag crash"}`,
			player, stack))
		h.ReportCrash(c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp ReportCrashResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return &resp
	}

	// First report creates the bug.
	resp := report("lua: /game/bag.lua:123: in function 'open'\n0x7ffd1 bag.lua:45", "p-1")
	assert.True(t, resp.Created)
	assert.Equal(t, int64(1), resp.Count)

	// Same crash site (different line/address noise, same player) → counter bump.
	resp = report("lua: /game/bag.lua:321: in function 'open'\n0x7ffd9 bag.lua:88", "p-1")
	assert.False(t, resp.Created)
	assert.Equal(t, int64(2), resp.Count)

	// Different player, same site → bump again.
	resp = report("lua: /game/bag.lua:55: in function 'open'\n0x7ffd2 bag.lua:9", "p-2")
	assert.Equal(t, int64(3), resp.Count)

	// Exactly one bug row exists.
	var count int64
	db.Model(&model.Bug{}).Count(&count)
	assert.Equal(t, int64(1), count)

	// Extra carries aggregation metadata.
	var stored model.Bug
	require.NoError(t, db.First(&stored).Error)
	assert.Equal(t, float64(3), stored.Extra[crashCountKey])
	assert.Equal(t, "p-2", stored.Extra[crashLastPlayerKey])
	assert.Equal(t, model.BugStatusTriage, stored.Status)
	assert.Equal(t, model.BugSeverityCritical, stored.Severity)

	// A different crash site opens a second bug.
	resp = report("lua: /game/shop.lua:7: attempt to index nil\n0x1 shop.lua:7", "p-3")
	assert.True(t, resp.Created)
	db.Model(&model.Bug{}).Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestReportCrash_Validation(t *testing.T) {
	h, _ := newCrashHandler(t)
	c, w := crashReq(http.MethodPost, "/bugs/crash", `{"gameId":"demo","stack":""}`)
	h.ReportCrash(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = crashReq(http.MethodPost, "/bugs/crash", `{"gameId":"","stack":"x"}`)
	h.ReportCrash(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
