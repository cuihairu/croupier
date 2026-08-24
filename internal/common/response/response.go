package response

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/cuihairu/croupier/internal/common/errorx"
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
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":   "internal_error",
		"message": err.Error(),
	})
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
