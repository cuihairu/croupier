package function

import (
	"log/slog"
	"strconv"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/gin-gonic/gin"
)

// B2：函数契约变更历史 REST 面（scoped by X-Game-ID/X-Env）。

func (h *Handler) ContractVersions(c *gin.Context) {
	logic, gameID, env, functionID, err := h.contractVersionsScope(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 20)
	resp, err := logic.List(gameID, env, functionID, page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	// 存量函数惰性回填（B2 版本历史晚于注册面引入，0029 前注册的契约没有
	// created 记录，历史页为空）：首次查看时补一条 created 初始快照再重查。
	// 历史是衍生审计数据，回填失败降级为日志、不影响列表响应。
	if resp != nil && resp.Total == 0 {
		if created, bfErr := contractsvc.NewContractService(h.service.SvcCtx().DB).
			BackfillInitialContractVersion(c.Request.Context(), gameID, env, functionID); bfErr != nil {
			slog.Default().Warn("backfill initial contract version failed",
				"functionId", functionID, "gameId", gameID, "env", env, "err", bfErr)
		} else if created {
			if resp, err = logic.List(gameID, env, functionID, page, pageSize); err != nil {
				response.Error(c, err)
				return
			}
		}
	}
	response.Success(c, resp)
}

func (h *Handler) ContractVersionDetail(c *gin.Context) {
	logic, gameID, env, functionID, err := h.contractVersionsScope(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	seq, err := parseVersionSeq(c.Param("seq"))
	if err != nil {
		response.Error(c, err)
		return
	}
	resp, err := logic.Detail(gameID, env, functionID, seq)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ContractVersionDiff(c *gin.Context) {
	logic, gameID, env, functionID, err := h.contractVersionsScope(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	fromSeq, err := parseVersionSeq(c.Query("from"))
	if err != nil {
		response.Error(c, errorx.NewBadRequest("from 必须是版本 seq"))
		return
	}
	toSeq, err := parseVersionSeq(c.Query("to"))
	if err != nil {
		response.Error(c, errorx.NewBadRequest("to 必须是版本 seq"))
		return
	}
	resp, err := logic.Diff(gameID, env, functionID, fromSeq, toSeq)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) contractVersionsScope(c *gin.Context) (*logicfunction.ContractVersionsLogic, string, string, string, error) {
	logic := logicfunction.NewContractVersionsLogic(c.Request.Context(), h.service.SvcCtx())
	if err := logic.CheckAccess(); err != nil {
		return nil, "", "", "", err
	}
	scope := logic.RequireScope()
	if scope.GameID == "" || scope.Env == "" {
		return nil, "", "", "", errorx.NewBadRequest("X-Game-ID/X-Env is required")
	}
	functionID := c.Param("id")
	if functionID == "" {
		return nil, "", "", "", errorx.NewBadRequest("id is required")
	}
	return logic, scope.GameID, scope.Env, functionID, nil
}

func parsePositiveInt(raw string, fallback int) int {
	if v, err := strconv.Atoi(raw); err == nil && v > 0 {
		return v
	}
	return fallback
}

func parseVersionSeq(raw string) (int64, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0, errorx.NewBadRequest("seq 必须是正整数")
	}
	return v, nil
}
