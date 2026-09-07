package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/stretchr/testify/assert"
)

// UpdateFunction 在 service.Get 成功之后、service.Update 的 Exists 检查之前，
// 若函数被并发请求删除，service.Update 返回 ErrNotFound，
// handler 的 response.Error(c, err) 分支对非 errorx 错误以 500 兜底返回。
// 该分支仅能通过 Get 与 Exists 之间的 TOCTOU 窗口触达，故用高频并发删除验证。
func TestHandler_UpdateFunction_DeletedBetweenGetAndUpdate(t *testing.T) {
	router, service := setupTestRouter()

	if err := service.Register(testCtx(), &functionv1.FunctionMetadata{Id: "race.update", Name: "Race"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = service.Delete(testCtx(), "race.update")
				_ = service.Register(testCtx(), &functionv1.FunctionMetadata{Id: "race.update"})
			}
		}()
	}

	hitInternalError := false
	for i := 0; i < 10000 && !hitInternalError; i++ {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/metadata/functions/race.update", strings.NewReader(`{"name":"Updated"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		switch w.Code {
		case http.StatusInternalServerError:
			// Get 成功但 Update 时函数已被删除 → service.Update 返回 ErrNotFound → 500
			hitInternalError = true
		case http.StatusOK, http.StatusNotFound:
			// 200 = 全程成功；404 = Get 阶段函数已不存在；均为竞争中间态，继续重试
		default:
			t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
		}
	}
	close(stop)
	wg.Wait()

	assert.True(t, hitInternalError, "expected TOCTOU window to trigger service.Update error path (500)")
}
