package audit

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// exportRowLimit is the maximum number of rows per export (memory and response
// body protection).
const exportRowLimit = 50000

// Export handles GET /api/v1/audit/export?format=json|csv.
//
// Permissions and filters match GetAuditLogs exactly (shared query builder);
// the export is capped at exportRowLimit rows, beyond which it is truncated and
// flagged via the X-Truncated response header.
func (h *Handler) Export(c *gin.Context) {
	var req AuditRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	format := normalizeExportFormat(c.Query("format"))

	svcItems, truncated, err := h.service.ExportRows(c.Request.Context(), &req, exportRowLimit)
	if err != nil {
		response.Error(c, errorx.NewInternalError("导出失败: "+err.Error()))
		return
	}

	filename := fmt.Sprintf("audit-export-%s.%s", time.Now().UTC().Format("20060102T150405Z"), format)
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	if truncated {
		c.Header("X-Truncated", "true")
	}

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv; charset=utf-8")
		writeCSV(c.Writer, svcItems)
	default:
		c.Header("Content-Type", "application/json; charset=utf-8")
		enc := json.NewEncoder(c.Writer)
		enc.SetIndent("", "  ")
		_ = enc.Encode(gin.H{
			"items":       svcItems,
			"count":       len(svcItems),
			"truncated":   truncated,
			"generatedAt": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// VerifyChain handles GET /api/v1/audit/chain/verify:
// it checks hash chain integrity (startSeq~endSeq, the full range by default)
// and returns the first break point.
func (h *Handler) VerifyChain(c *gin.Context) {
	result, err := h.service.VerifyChain(c.Request.Context())
	if err != nil {
		response.Error(c, errorx.NewInternalError("校验失败: "+err.Error()))
		return
	}
	response.Success(c, result)
}

func normalizeExportFormat(v string) string {
	if v == "csv" {
		return "csv"
	}
	return "json"
}

func writeCSV(w gin.ResponseWriter, items []AuditItem) {
	cw := csv.NewWriter(w)
	// BOM: helps Excel recognize UTF-8.
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	header := []string{"id", "timestamp", "action", "result", "userId", "gameId", "env", "target", "traceId"}
	_ = cw.Write(header)
	for i := range items {
		it := &items[i]
		_ = cw.Write([]string{
			it.ID, it.CreatedAt, it.Action, it.Result, it.UserID, it.GameID, it.Env, it.Target, it.TraceID,
		})
	}
	cw.Flush()
}
