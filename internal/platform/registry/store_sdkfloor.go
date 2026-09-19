package registry

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/platform/registry/sdkversion"
)

// SDK 滑动版本门槛的高水位存取（判定逻辑见 sdkversion 包）。
// DB 模式：高水位落 game 库 sdk_version_highwatermarks 表（经 scopeContext
// 解析 per-game 连接，对齐契约物化路径）。表模型在 internal/model
// （migration/0028），但 registry 不能 import model（agent_session_model
// 反向依赖本包，writeToDB 同款 import cycle），故此处用匿名 struct +
// Table() 直达表——列集与 model.SDKVersionHighwatermark 保持一致。
// 无 DB（内存 registry）退化为进程内 map：重启丢失，见文档「已知边界」。

// sdkVersionHwmRow mirrors model.SDKVersionHighwatermark（列集变更需两侧同步）.
type sdkVersionHwmRow struct {
	ID          uint64 `gorm:"primaryKey"`
	GameID      string `gorm:"size:64;uniqueIndex:idx_sdk_hwm_game_env_lang"`
	Env         string `gorm:"size:64;uniqueIndex:idx_sdk_hwm_game_env_lang"`
	SdkLanguage string `gorm:"size:32;uniqueIndex:idx_sdk_hwm_game_env_lang"`
	SdkVersion  string `gorm:"size:32"`
	UpdatedAt   time.Time
}

func (sdkVersionHwmRow) TableName() string { return "sdk_version_highwatermarks" }

func sdkHwmKey(gameID, env, language string) string {
	return gameID + "\x00" + env + "\x00" + language
}

// GetSDKVersionFloor returns the highest SDK version recorded for
// (game_id, env, sdk_language), or "" when nothing has been observed yet.
func (s *Store) GetSDKVersionFloor(gameID, env, language string) string {
	if s == nil {
		return ""
	}
	language = strings.TrimSpace(language)
	if language == "" {
		return ""
	}
	if s.db != nil {
		var row sdkVersionHwmRow
		err := dbctx.Resolve(s.rebuildContext(gameID, env), s.db).
			Where("game_id = ? AND env = ? AND sdk_language = ?", gameID, env, language).
			First(&row).Error
		if err == nil {
			return row.SdkVersion
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Warn("sdk version floor lookup failed", "game_id", gameID, "env", env, "language", language, "error", err)
		}
		return ""
	}
	s.sdkHwmMu.Lock()
	defer s.sdkHwmMu.Unlock()
	return s.sdkHwm[sdkHwmKey(gameID, env, language)]
}

// ObserveSDKVersion raises the recorded high watermark when version parses
// strictly higher than the stored one. 不可解析的自报版本（"unknown"）不
// 参与抬升；写失败降级为警告——门槛状态缺失只影响下次判定，不阻断注册。
func (s *Store) ObserveSDKVersion(gameID, env, language, version string) {
	if s == nil {
		return
	}
	language = strings.TrimSpace(language)
	if language == "" || !sdkversion.Parseable(version) {
		return
	}
	if s.db != nil {
		db := dbctx.Resolve(s.rebuildContext(gameID, env), s.db)
		var row sdkVersionHwmRow
		err := db.Where("game_id = ? AND env = ? AND sdk_language = ?", gameID, env, language).
			First(&row).Error
		switch {
		case err == nil:
			if !sdkversion.Higher(version, row.SdkVersion) {
				return
			}
			if err := db.Model(&sdkVersionHwmRow{}).
				Where("id = ?", row.ID).
				Update("sdk_version", version).Error; err != nil {
				slog.Warn("sdk version floor raise failed", "game_id", gameID, "env", env, "language", language, "error", err)
			}
			return
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := db.Create(&sdkVersionHwmRow{
				GameID: gameID, Env: env, SdkLanguage: language, SdkVersion: version,
			}).Error; err != nil {
				slog.Warn("sdk version floor create failed", "game_id", gameID, "env", env, "language", language, "error", err)
			}
			return
		default:
			slog.Warn("sdk version floor lookup failed", "game_id", gameID, "env", env, "language", language, "error", err)
			return
		}
	}
	s.sdkHwmMu.Lock()
	defer s.sdkHwmMu.Unlock()
	key := sdkHwmKey(gameID, env, language)
	if existing, ok := s.sdkHwm[key]; !ok || sdkversion.Higher(version, existing) {
		s.sdkHwm[key] = version
	}
}
