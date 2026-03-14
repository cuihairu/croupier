package response

import (
	"errors"
	"net/http"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/gin-gonic/gin"
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
	var codeErr *errorx.CodeError
	if errors.As(err, &codeErr) {
		c.JSON(codeErr.Code, gin.H{
			"error":   codeErr.ErrorCode(),
			"message": codeErr.Message,
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
