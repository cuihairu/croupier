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

// 调用函数
func FunctionInvokeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(rawBody))

		var req types.FunctionInvokeRequest
		if err := httpx.Parse(r, &req); err != nil {
			logx.WithContext(r.Context()).Errorf("httpx.Parse failed for function invoke: %v", err)
			if fallbackReq, ok := parseFunctionInvokeFallback(r.URL.Path, rawBody); ok {
				req = fallbackReq
			} else {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}

		l := function.NewFunctionInvokeLogic(r.Context(), svcCtx)
		resp, err := l.FunctionInvoke(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func parseFunctionInvokeFallback(path string, rawBody []byte) (types.FunctionInvokeRequest, bool) {
	req := types.FunctionInvokeRequest{}
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasSuffix(path, "/invoke") {
		return req, false
	}
	prefix := "/api/v1/functions/"
	if !strings.HasPrefix(path, prefix) {
		return req, false
	}
	functionID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/invoke")
	functionID = strings.Trim(functionID, "/")
	if functionID == "" {
		return req, false
	}
	req.ID = functionID

	if len(rawBody) == 0 {
		return req, true
	}

	var payload map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(rawBody))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return req, false
	}

	req.Params = payload["params"]
	req.Payload = payload["payload"]
	req.GameID = asString(payload["gameId"])
	req.Env = asString(payload["env"])
	req.Mode = asString(payload["mode"])
	req.Route = asString(payload["route"])
	req.TargetServiceID = asString(payload["target_service_id"])
	req.HashKey = asString(payload["hash_key"])

	return req, true
}

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}
