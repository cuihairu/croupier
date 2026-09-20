package registry

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/platform/registry/sdkversion"
)

// 函数级最低 SDK 版本门槛存取（判定见 control_handler.evaluateSDKVersionFloor）。
// DB 模式：落 game 库 function_version_floors 表（经 scopeContext 解析 per-game
// 连接，对齐高水位/契约物化路径）。表模型在 internal/model（migration/0030），
// 但 registry 不能 import model（agent_session_model 反向依赖本包，import
// cycle），故此处用匿名 struct + Table() 直达表——列集与
// model.FunctionVersionFloor 保持一致。无 DB（内存 registry）退化为进程内
// map：重启丢失，见文档「已知边界」。

// functionVersionFloorRow mirrors model.FunctionVersionFloor（列集变更需两侧同步）.
type functionVersionFloorRow struct {
	ID         uint64 `gorm:"primaryKey"`
	GameID     string `gorm:"size:64;uniqueIndex:idx_fn_floor_game_env_fn"`
	Env        string `gorm:"size:64;uniqueIndex:idx_fn_floor_game_env_fn"`
	FunctionID string `gorm:"size:128;uniqueIndex:idx_fn_floor_game_env_fn"`
	MinVersion string `gorm:"size:32"`
	UpdatedBy  string `gorm:"size:64"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (functionVersionFloorRow) TableName() string { return "function_version_floors" }

func fnFloorKey(gameID, env, functionID string) string {
	return gameID + "\x00" + env + "\x00" + functionID
}

// GetFunctionVersionFloors 返回 (game_id, env) 下全部函数级最低版本
// （function_id → min_version）。注册评估每请求调用一次（批量取，避免
// 逐函数查询）；出错降级为空表（门槛缺失只放宽判定，不阻断注册）。
func (s *Store) GetFunctionVersionFloors(gameID, env string) map[string]string {
	out := map[string]string{}
	if s == nil {
		return out
	}
	if s.db != nil {
		var rows []functionVersionFloorRow
		err := dbctx.Resolve(s.rebuildContext(gameID, env), s.db).
			Where("game_id = ? AND env = ?", gameID, env).
			Find(&rows).Error
		if err != nil {
			slog.Warn("function version floors lookup failed", "game_id", gameID, "env", env, "error", err)
			return out
		}
		for _, row := range rows {
			out[row.FunctionID] = row.MinVersion
		}
		return out
	}
	s.fnFloorMu.Lock()
	defer s.fnFloorMu.Unlock()
	prefix := gameID + "\x00" + env + "\x00"
	for key, min := range s.fnFloor {
		if strings.HasPrefix(key, prefix) {
			out[strings.TrimPrefix(key, prefix)] = min
		}
	}
	return out
}

// GetFunctionVersionFloor 返回单函数的最低版本（无配置返回 ""）。设置
// API 回显用；注册评估走批量 GetFunctionVersionFloors。
func (s *Store) GetFunctionVersionFloor(gameID, env, functionID string) string {
	if s == nil || strings.TrimSpace(functionID) == "" {
		return ""
	}
	if s.db != nil {
		var row functionVersionFloorRow
		err := dbctx.Resolve(s.rebuildContext(gameID, env), s.db).
			Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
			First(&row).Error
		if err == nil {
			return row.MinVersion
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Warn("function version floor lookup failed", "game_id", gameID, "env", env, "function_id", functionID, "error", err)
		}
		return ""
	}
	s.fnFloorMu.Lock()
	defer s.fnFloorMu.Unlock()
	return s.fnFloor[fnFloorKey(gameID, env, functionID)]
}

// SetFunctionVersionFloor 设置/更新函数级最低版本。minVersion 必须可解析
// （sdkversion.Parseable），否则返回错误——门槛两端可解析才生效，不可
// 解析值入库是死配置。
func (s *Store) SetFunctionVersionFloor(gameID, env, functionID, minVersion, updatedBy string) error {
	if s == nil {
		return errors.New("registry store is nil")
	}
	functionID = strings.TrimSpace(functionID)
	minVersion = strings.TrimSpace(minVersion)
	if functionID == "" {
		return errors.New("function_id is required")
	}
	if !sdkversion.Parseable(minVersion) {
		return fmt.Errorf("min_version %q is not a parseable semantic version", minVersion)
	}
	if s.db != nil {
		db := dbctx.Resolve(s.rebuildContext(gameID, env), s.db)
		var row functionVersionFloorRow
		err := db.Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
			First(&row).Error
		switch {
		case err == nil:
			return db.Model(&functionVersionFloorRow{}).
				Where("id = ?", row.ID).
				Updates(map[string]any{"min_version": minVersion, "updated_by": updatedBy}).Error
		case errors.Is(err, gorm.ErrRecordNotFound):
			return db.Create(&functionVersionFloorRow{
				GameID: gameID, Env: env, FunctionID: functionID,
				MinVersion: minVersion, UpdatedBy: updatedBy,
			}).Error
		default:
			return err
		}
	}
	s.fnFloorMu.Lock()
	defer s.fnFloorMu.Unlock()
	s.fnFloor[fnFloorKey(gameID, env, functionID)] = minVersion
	return nil
}

// DeleteFunctionVersionFloor 清空函数级最低版本（物理删行：硬删模型，
// 软删残留会占住物理唯一索引——0024 同族教训）。
func (s *Store) DeleteFunctionVersionFloor(gameID, env, functionID string) error {
	if s == nil || strings.TrimSpace(functionID) == "" {
		return nil
	}
	if s.db != nil {
		return dbctx.Resolve(s.rebuildContext(gameID, env), s.db).
			Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
			Delete(&functionVersionFloorRow{}).Error
	}
	s.fnFloorMu.Lock()
	defer s.fnFloorMu.Unlock()
	delete(s.fnFloor, fnFloorKey(gameID, env, functionID))
	return nil
}
