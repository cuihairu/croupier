package incident

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 单条事故行级作用域守卫回归（AUD-COV-001 P0：Get/Update/Transition 按 id
// 直查直改可跨 game/env 越权）。作用域经 GameDBMiddleware 注入 context，
// 测试用 WithGameScope 模拟。
func scopeCtx(gameID, env string) context.Context {
	return svc.WithGameScope(context.Background(), svc.GameScope{GameID: gameID, Env: env})
}

func seedScopedIncident(t *testing.T, s *Service, gameID, env string) *IncidentDTO {
	t.Helper()
	req := &IncidentCreateRequest{
		Title:      "scope-probe",
		CategoryID: 1,
		Severity:   model.IncidentSeverityInfo,
		GameID:     gameID,
		Env:        env,
	}
	dto, created, err := s.CreateIncident(context.Background(), req, "seed")
	require.NoError(t, err)
	require.True(t, created)
	return dto
}

func TestCreateIncidentForcesCurrentScope(t *testing.T) {
	s := setupService(t)
	// body 伪造其他 game/env：归属必须落当前 scope
	req := &IncidentCreateRequest{
		Title:      "spoofed-attribution",
		CategoryID: 1,
		GameID:     "other-game",
		Env:        "other-env",
	}
	dto, created, err := s.CreateIncident(scopeCtx("demo-game", "demo"), req, "op")
	require.NoError(t, err)
	require.True(t, created)
	assert.Equal(t, "demo-game", dto.GameID)
	assert.Equal(t, "demo", dto.Env)
}

func TestGetIncidentRejectsForeignScope(t *testing.T) {
	s := setupService(t)
	dto := seedScopedIncident(t, s, "game-a", "prod")

	_, err := s.GetIncident(scopeCtx("game-b", "prod"), dto.ID)
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expected *errorx.CodeError, got %T", err)
	assert.Contains(t, codeErr.Message, "无权访问该事故")

	// 同 scope 正常读
	got, err := s.GetIncident(scopeCtx("game-a", "prod"), dto.ID)
	require.NoError(t, err)
	assert.Equal(t, dto.ID, got.ID)
}

func TestUpdateIncidentRejectsForeignScopeAndCrossScopeMove(t *testing.T) {
	s := setupService(t)
	dto := seedScopedIncident(t, s, "game-a", "prod")

	// 跨 scope 更新被拒
	_, err := s.UpdateIncident(scopeCtx("game-b", "prod"), dto.ID, &IncidentUpdateRequest{
		Title: strPtr("hijacked"),
	})
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Contains(t, codeErr.Message, "无权访问该事故")

	// 同 scope 内把归属迁出当前 scope 也被拒
	_, err = s.UpdateIncident(scopeCtx("game-a", "prod"), dto.ID, &IncidentUpdateRequest{
		GameID: strPtr("game-b"),
	})
	require.Error(t, err)
	codeErr, ok = err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Contains(t, codeErr.Message, "归属迁移只允许落在当前游戏/环境内")

	// 行内归属未被改动
	row, err := model.NewIncidentModel(s.svcCtx.DB).Get(context.Background(), dto.ID)
	require.NoError(t, err)
	assert.Equal(t, "game-a", row.GameID)
}

func TestTransitionIncidentRejectsForeignScope(t *testing.T) {
	s := setupService(t)
	dto := seedScopedIncident(t, s, "game-a", "prod")

	_, err := s.TransitionIncident(scopeCtx("game-a", "stage"), dto.ID, &IncidentStatusRequest{
		Status: model.IncidentStatusResolved,
	})
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Contains(t, codeErr.Message, "无权访问该事故")

	// 同 scope 流转正常
	out, err := s.TransitionIncident(scopeCtx("game-a", "prod"), dto.ID, &IncidentStatusRequest{
		Status: model.IncidentStatusResolved,
	})
	require.NoError(t, err)
	assert.Equal(t, model.IncidentStatusResolved, out.Status)
}
