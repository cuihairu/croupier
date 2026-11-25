package incident

import (
	"bytes"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
)

func setupRouter(t *testing.T) (*gin.Engine, *Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := setupService(t)
	router := gin.New()
	// Minimal middleware that injects a JWT username (CreateIncident reads c.GetString("username")).
	router.Use(func(c *gin.Context) {
		c.Set("username", "tester")
		c.Next()
	})
	g := router.Group("/incident-categories")
	g.GET("", func(c *gin.Context) { NewHandler(s).ListCategories(c) })
	g.POST("", func(c *gin.Context) { NewHandler(s).CreateCategory(c) })
	g.DELETE("/:id", func(c *gin.Context) { NewHandler(s).DeleteCategory(c) })
	ig := router.Group("/incidents")
	ig.GET("", func(c *gin.Context) { NewHandler(s).ListIncidents(c) })
	ig.POST("", func(c *gin.Context) { NewHandler(s).CreateIncident(c) })
	ig.POST("/:id/status", func(c *gin.Context) { NewHandler(s).TransitionIncident(c) })
	return router, s
}

func doJSON(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestHandler_CreateIncident_CreatedVsReplay(t *testing.T) {
	router, s := setupRouter(t)
	qa := mustCategory(t, s, model.IncidentCatQA)
	body := `{"title":"漏测","categoryId":` + strconv.FormatUint(uint64(qa.ID), 10) + `,"incidentKey":"qa:1"}`
	w := doJSON(t, router, http.MethodPost, "/incidents", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create = %d, want 201: %s", w.Code, w.Body.String())
	}
	w = doJSON(t, router, http.MethodPost, "/incidents", body)
	if w.Code != http.StatusOK {
		t.Fatalf("replay = %d, want 200: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ListCategories(t *testing.T) {
	router, _ := setupRouter(t)
	w := doJSON(t, router, http.MethodGet, "/incident-categories", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !strings.Contains(got, `"slug":"uncategorized"`) {
		t.Fatalf("missing uncategorized seed in %s", got)
	}
}

func TestHandler_BadRequestPaths(t *testing.T) {
	router, _ := setupRouter(t)
	// Non-numeric id
	if w := doJSON(t, router, http.MethodDelete, "/incident-categories/abc", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id = %d, want 400", w.Code)
	}
	// Malformed JSON body
	if w := doJSON(t, router, http.MethodPost, "/incidents", "{not json"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad body = %d, want 400", w.Code)
	}
	// Invalid status
	if w := doJSON(t, router, http.MethodPost, "/incidents/1/status", `{"status":"bogus"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad status = %d, want 400: %s", w.Code, w.Body.String())
	}
}

// TestHandler_ListIncidents_UnixSecondsFrom: the query from also accepts the unix
// seconds form (RangePicker uses RFC3339; script calls often pass seconds).
func TestHandler_ListIncidents_UnixSecondsFrom(t *testing.T) {
	router, _ := setupRouter(t)
	w := doJSON(t, router, http.MethodGet, "/incidents?from=1700000000", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", w.Code, w.Body.String())
	}
	// Unparseable time values are ignored (no error); the list is returned as usual
	w = doJSON(t, router, http.MethodGet, "/incidents?from=not-a-time", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list with bad from = %d: %s", w.Code, w.Body.String())
	}
}

// ---- parseUintParam/parseUintQuery platform width wraparound (code scanning #226/#227) ----
//
// bitSize=strconv.IntSize: a 64-bit input overflows on a 32-bit platform and
// errors/falls back right away, while uint(v) is never truncated - proving
// "the bound is enforced during parsing" rather than "a check afterwards".

func TestParseUintParam_RejectsZero(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Params = gin.Params{{Key: "id", Value: "0"}}
	if _, err := parseUintParam(c, "id"); err == nil {
		t.Fatal("zero id must be rejected")
	}
}

func TestParseUintParam_NonNumeric(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	if _, err := parseUintParam(c, "id"); err == nil {
		t.Fatal("non-numeric id must be rejected")
	}
}

// Build an input beyond IntSize: 10×(2^64-1) exceeds any platform width bound
// (always over-bound on both 32/64-bit - bitSize itself is the target type's
// width, and exceeding it makes ParseUint report a range error).
func overflowInput() string {
	return strconv.FormatUint(math.MaxUint64, 10) + "0"
}

func TestParseUintParam_OverflowRejected(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Params = gin.Params{{Key: "id", Value: overflowInput()}}
	if _, err := parseUintParam(c, "id"); err == nil {
		t.Fatalf("overflow input %s must be rejected", overflowInput())
	}
}

func TestParseUintQuery_OverflowFallsBackToZero(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/incidents?page="+overflowInput(), nil)
	if got := parseUintQuery(c, "page"); got != 0 {
		t.Fatalf("overflow query = %d, want 0 (no truncation)", got)
	}
}

func TestParseUintQuery_ValidPasses(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/incidents?page=3&pageSize=50", nil)
	if got := parseUintQuery(c, "page"); got != 3 {
		t.Fatalf("page = %d, want 3", got)
	}
	if got := parseUintQuery(c, "pageSize"); got != 50 {
		t.Fatalf("pageSize = %d, want 50", got)
	}
}

// ---- parseIntQuery pagination clamping (code scanning #228/#229) ----
//
// A direct uint->int conversion wraps negative on 64-bit platforms for uint
// inputs >MaxInt64. After parsing, page values are clamped to MaxInt32 - values
// never approach that bound, and anything above it falls back to 0 (via the
// Normalize defaults).

// 2^63: the exact form where ParseUint (64-bit) succeeds but the direct
// uint->int conversion wraps negative.
func wrapInput() string {
	return strconv.FormatUint(uint64(math.MaxInt64)+1, 10)
}

func TestParseIntQuery_UintWrapClampsToZero(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/incident-reports/stored?page="+wrapInput(), nil)
	if got := parseIntQuery(c, "page"); got != 0 {
		t.Fatalf("page = %d, want 0 clamp (no uint→int wrap)", got)
	}
}

func TestParseIntQuery_GarbageFallsBackToZero(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/incident-reports/stored?page=abc", nil)
	if got := parseIntQuery(c, "page"); got != 0 {
		t.Fatalf("garbage page = %d, want 0", got)
	}
}

func TestParseIntQuery_InRangePasses(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/incident-reports/stored?page=2&pageSize=20", nil)
	if got := parseIntQuery(c, "page"); got != 2 {
		t.Fatalf("page = %d, want 2", got)
	}
	if got := parseIntQuery(c, "pageSize"); got != 20 {
		t.Fatalf("pageSize = %d, want 20", got)
	}
}
