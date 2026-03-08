// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/workspace"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 保存 Workspace 配置（创建或更新）
func WorkspaceConfigSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(rawBody))

		if err := requireWorkspacePermission(r.Context(), svcCtx, "edit"); err != nil {
			writeWorkspaceError(w, r, err, "save")
			return
		}
		req, err := parseWorkspaceConfigSaveRequest(r.URL.Path, rawBody)
		if err != nil {
			writeWorkspaceError(w, r, err, "save")
			return
		}

		ctx := withWorkspaceRequestID(r.Context(), resolveRequestIDFromRequest(r))
		l := workspace.NewWorkspaceConfigSaveLogic(ctx, svcCtx)
		resp, err := l.WorkspaceConfigSave(&req)
		if err != nil {
			writeWorkspaceError(w, r, err, "save")
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func parseWorkspaceConfigSaveRequest(path string, rawBody []byte) (types.WorkspaceConfigSaveRequest, error) {
	req := types.WorkspaceConfigSaveRequest{}
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasSuffix(path, "/config") {
		return req, fmt.Errorf("invalid workspace save path: %s", path)
	}
	prefix := "/api/v1/workspaces/"
	if !strings.HasPrefix(path, prefix) {
		return req, fmt.Errorf("invalid workspace save path: %s", path)
	}
	objectKey := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/config")
	objectKey = strings.Trim(objectKey, "/")
	if objectKey == "" {
		return req, fmt.Errorf("objectKey is required")
	}
	req.ObjectKey = objectKey

	var payload struct {
		Title       string      `json:"title"`
		Description string      `json:"description"`
		Layout      interface{} `json:"layout"`
		Status      string      `json:"status"`
		MenuOrder   int         `json:"menuOrder"`
	}
	if len(rawBody) > 0 {
		dec := json.NewDecoder(bytes.NewReader(rawBody))
		dec.UseNumber()
		if err := dec.Decode(&payload); err != nil && err.Error() != "EOF" {
			return req, err
		}
	}

	req.Title = payload.Title
	req.Description = payload.Description
	req.Layout = payload.Layout
	req.Status = payload.Status
	req.MenuOrder = payload.MenuOrder
	return req, nil
}
