package analytics

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type hotQuestionService interface {
	HotQuestions(ctx context.Context, limit int) ([]HotQuestion, error)
}

type Handler struct {
	service hotQuestionService
}

func NewHandler(service hotQuestionService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/hot-questions", h.HotQuestions)
}

func (h *Handler) HotQuestions(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    50000,
			"message": ErrServiceNotConfigured.Error(),
			"data":    nil,
		})
		return
	}

	limit := 0
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    40000,
				"message": "invalid limit",
				"data":    nil,
			})
			return
		}
		limit = parsed
	}

	items, err := h.service.HotQuestions(c.Request.Context(), limit)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrServiceNotConfigured) || errors.Is(err, ErrRepositoryNotConfigured) {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{
			"code":    50000,
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items": items,
		},
	})
}
