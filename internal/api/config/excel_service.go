// Excel import/compile endpoints (excel-online-design.md §5).
package config

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// ExcelService compiles Excel sources into ConfigVersion records.
type ExcelService struct {
	svcCtx *svc.ServiceContext
}

// NewExcelService creates an excel config service.
func NewExcelService(svcCtx *svc.ServiceContext) *ExcelService {
	return &ExcelService{svcCtx: svcCtx}
}

// ImportXLSXRequest carries an uploaded workbook.
type ImportXLSXRequest struct {
	Data    []byte
	GameID  string
	Env     string
	Message string
}

// CompileSnapshotRequest carries the Univer snapshot from the web editor.
type CompileSnapshotRequest struct {
	Snapshot json.RawMessage `json:"snapshot"`
	GameID   string          `json:"gameId,optional"`
	Env      string          `json:"env,optional"`
	Key      string          `json:"key,optional"`
	Message  string          `json:"message,optional"`
}

// ExcelCompileResponse is the registered version.
type ExcelCompileResponse struct {
	Key     string `json:"key"`
	Version int    `json:"version"`
	Sheets  int    `json:"sheets"`
	Rows    int    `json:"rows"`
}

// ImportXLSX parses the uploaded workbook, compiles it and registers a new
// ConfigVersion (namespace=gameplay).
func (s *ExcelService) ImportXLSX(ctx context.Context, req *ImportXLSXRequest) (*ExcelCompileResponse, error) {
	if len(req.Data) == 0 {
		return nil, errorx.NewBadRequest("缺少 xlsx 文件")
	}
	if len(req.Data) > 4*1024*1024 {
		return nil, errorx.NewBadRequest("文件超过 4MB 上限")
	}
	wb, err := CompileXLSX(req.Data)
	if err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	return s.register(ctx, "excel", wb, req.GameID, req.Env, req.Message, currentExcelUser(ctx))
}

// CompileSnapshot compiles the Univer snapshot and registers the version.
func (s *ExcelService) CompileSnapshot(ctx context.Context, req *CompileSnapshotRequest) (*ExcelCompileResponse, error) {
	if len(req.Snapshot) == 0 {
		return nil, errorx.NewBadRequest("缺少快照内容")
	}
	wb, err := CompileSnapshot(req.Snapshot)
	if err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		key = "excel.workbook"
	}
	return s.register(ctx, key, wb, req.GameID, req.Env, req.Message, currentExcelUser(ctx))
}

// register writes the compiled workbook as a new gameplay config version.
func (s *ExcelService) register(ctx context.Context, key string, wb *ExcelWorkbook, gameID, env, message, createdBy string) (*ExcelCompileResponse, error) {
	value, err := json.Marshal(wb)
	if err != nil {
		return nil, err
	}
	rows := 0
	for _, sheet := range wb.Sheets {
		rows += len(sheet.Rows)
	}
	rec, err := s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:       key,
		Content:   string(value),
		Format:    "json",
		GameID:    gameID,
		Env:       env,
		Message:   message,
		Namespace: model.ConfigNamespaceGameplay,
	}, createdBy)
	if err != nil {
		return nil, err
	}
	return &ExcelCompileResponse{
		Key: rec.Key, Version: rec.Version, Sheets: len(wb.Sheets), Rows: rows,
	}, nil
}

func currentExcelUser(ctx context.Context) string {
	if name, err := utils.CurrentUsername(ctx); err == nil && name != "" {
		return name
	}
	return "system"
}
