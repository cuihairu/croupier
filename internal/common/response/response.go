package response

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	apperrors "github.com/cuihairu/croupier/internal/errors"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, data)
}

// Accepted 以 202 Accepted 返回异步任务受理结果。
func Accepted(c *gin.Context, data interface{}) {
	c.JSON(http.StatusAccepted, data)
}

func NoContent(c *gin.Context) {
	c.AbortWithStatus(http.StatusNoContent)
}

func Error(c *gin.Context, err error) {
	var valErr *errorx.ValidationError
	if errors.As(err, &valErr) {
		c.JSON(valErr.Code, gin.H{
			"error":   "validation_failed",
			"message": valErr.Message,
			"details": valErr.Details,
		})
		return
	}
	var bindValidationErr validator.ValidationErrors
	if errors.As(err, &bindValidationErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_failed",
			"message": err.Error(),
		})
		return
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		BadRequest(c, err.Error())
		return
	}
	var unmarshalTypeErr *json.UnmarshalTypeError
	if errors.As(err, &unmarshalTypeErr) {
		BadRequest(c, err.Error())
		return
	}
	// query/uri 数值转换失败（?page=abc 之类）是客户端输入错误，
	// 应按契约返回 400 而非 500（历史上落入 internal_error 兜底）。
	var numErr *strconv.NumError
	if errors.As(err, &numErr) {
		BadRequest(c, err.Error())
		return
	}
	var codeErr *errorx.CodeError
	if errors.As(err, &codeErr) {
		code, payload := codeErr.Data()
		c.JSON(code, payload)
		return
	}
	// GORM record-not-found maps to 404 so model-layer First()/RecordNotFound()
	// errors surface correctly instead of leaking as 500 internal_error.
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": "资源不存在",
		})
		return
	}
	// 平台层 AppError（dispatcher/registry 等）：码表自带 HTTPStatusCode
	// （SERVICE_UNAVAILABLE→503、NOT_FOUND→404…），按契约透出而非落 500 兜底。
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		message := appErr.Message
		if message == "" {
			message = err.Error()
		}
		details := gin.H{}
		if appErr.Operation != "" {
			details["operation"] = appErr.Operation
		}
		if appErr.Details != "" {
			details["detail"] = appErr.Details
		}
		c.JSON(appErr.HTTPStatusCode, gin.H{
			"error":   strings.ToLower(string(appErr.Code)),
			"message": message,
			"details": details,
		})
		return
	}
	// AUD-047：未知错误统一 opaque 500。err.Error() 可能携带 GORM/驱动
	// 细节（SQL 片段、表名、方言错误码、内网拓扑），不回传客户端；原文
	// 经 slog 进服务端日志，响应只带稳定文案 + 追踪号，运维凭追踪号回查。
	corr := correlationID()
	slog.ErrorContext(requestCtx(c), "unhandled internal error",
		"correlationId", corr,
		"path", c.FullPath(),
		"error", err)
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":   "internal_error",
		"message": "服务器内部错误，请稍后重试；如需排查请联系管理员并提供追踪号 " + corr,
		"details": gin.H{"correlationId": corr},
	})
}

// correlationID 生成短随机追踪号（16 hex 字符）。crypto/rand 失败时退到
// 纳秒时间戳——可读性一致，仍满足「日志 ↔ 响应可关联」的用途。
func correlationID() string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

// requestCtx 取 gin 请求上下文；Request 未装配（部分单测形态）时退回
// Background，避免日志行本身 panic。
func requestCtx(c *gin.Context) context.Context {
	if c.Request != nil && c.Request.Context() != nil {
		return c.Request.Context()
	}
	return context.Background()
}

type ListResponse struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"pageSize"`
}

func SuccessList(c *gin.Context, items interface{}, total int64, page, size int) {
	Success(c, ListResponse{
		Items: items,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":   "bad_request",
		"message": message,
	})
}

func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"error":   "unauthorized",
		"message": message,
	})
}

func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, gin.H{
		"error":   "forbidden",
		"message": message,
	})
}

func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, gin.H{
		"error":   "not_found",
		"message": message,
	})
}

func InternalServerError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":   "internal_error",
		"message": message,
	})
}

func ServiceUnavailable(c *gin.Context, message string) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "service_unavailable",
		"message": message,
	})
}
