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

// Accepted returns an async task acceptance result with 202 Accepted.
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
	// Numeric conversion failures for query/uri parameters (e.g. ?page=abc)
	// are client input errors and must return 400 per the contract rather
	// than 500 (historically they fell into the internal_error fallback).
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
	// Platform-layer AppError (dispatcher/registry etc.): its code table
	// carries its own HTTPStatusCode (SERVICE_UNAVAILABLE→503,
	// NOT_FOUND→404, ...); surface it per the contract instead of falling
	// through to the 500 fallback.
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
	// AUD-047: unknown errors uniformly return an opaque 500. err.Error()
	// may carry GORM/driver details (SQL fragments, table names, dialect
	// error codes, internal network topology) and is not returned to the
	// client; the original text goes to server logs via slog, and the
	// response only carries a stable message plus a correlation ID that
	// ops can trace back.
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

// correlationID generates a short random correlation ID (16 hex characters).
// If crypto/rand fails it falls back to a nanosecond timestamp — the same
// readable shape, still satisfying the "logs ↔ response correlation" purpose.
func correlationID() string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

// requestCtx fetches the gin request context; when Request is not wired up
// (some unit test shapes), it falls back to Background so the log line itself
// does not panic.
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
