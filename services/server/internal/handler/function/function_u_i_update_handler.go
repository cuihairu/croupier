// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/function"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 更新函数UI配置
func FunctionUIUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(rawBody))

		var req types.FunctionUIUpdateRequest
		if err := httpx.Parse(r, &req); err != nil {
			// Fallback path/body parse to avoid parser edge cases on dynamic schema payloads.
			logx.WithContext(r.Context()).Errorf("httpx.Parse failed for function ui update: %v", err)
			if fallbackReq, ok := parseFunctionUIUpdateFallback(r.URL.Path, rawBody); ok {
				req = fallbackReq
			} else {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}

		l := function.NewFunctionUIUpdateLogic(r.Context(), svcCtx)
		resp, err := l.FunctionUIUpdate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func parseFunctionUIUpdateFallback(path string, rawBody []byte) (types.FunctionUIUpdateRequest, bool) {
	req := types.FunctionUIUpdateRequest{}
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasSuffix(path, "/ui") {
		return req, false
	}
	prefix := "/api/v1/functions/"
	if !strings.HasPrefix(path, prefix) {
		return req, false
	}
	functionID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/ui")
	functionID = strings.Trim(functionID, "/")
	if functionID == "" {
		return req, false
	}
	req.ID = functionID

	var payload struct {
		Schema     interface{} `json:"schema"`
		Layout     interface{} `json:"layout"`
		Components interface{} `json:"components"`
	}
	if len(rawBody) > 0 {
		dec := json.NewDecoder(bytes.NewReader(rawBody))
		dec.UseNumber()
		if err := dec.Decode(&payload); err != nil && err.Error() != "EOF" {
			return req, false
		}
	}
	req.Schema = payload.Schema
	req.Layout = payload.Layout
	req.Components = payload.Components
	return req, true
}
