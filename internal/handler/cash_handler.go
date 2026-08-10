package handler

import (
	"net/http"

	"github.com/cashvio/cashvio-be/internal/config"
	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CashHandler struct {
	svc    service.CashService
	config *config.Config
}

func NewCashHandler(svc service.CashService, cfg *config.Config) *CashHandler {
	return &CashHandler{svc: svc, config: cfg}
}

func (h *CashHandler) GetCash(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	cash, err := h.svc.GetCash(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cash})
}

func (h *CashHandler) CreateWithdrawal(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	var req model.CreateCashWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	withdrawal, err := h.svc.CreateWithdrawal(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": withdrawal})
}

func (h *CashHandler) GetWithdrawals(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	q := model.NewCashWithdrawalQuery()
	q = service.ParseCashWithdrawalQuery(q,
		c.Query("limit"),
		c.Query("offset"),
		c.Query("search"),
		c.Query("sort_by"),
		c.Query("order"),
	)

	withdrawals, total, err := h.svc.GetAllWithdrawals(c.Request.Context(), q, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       withdrawals,
		"pagination": model.NewPagination(total, q.Limit, q.Offset),
	})
}

func (h *CashHandler) GetWithdrawal(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid withdrawal id"})
		return
	}

	withdrawal, err := h.svc.GetWithdrawalByID(c.Request.Context(), id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": withdrawal})
}

func (h *CashHandler) DeleteWithdrawal(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid withdrawal id"})
		return
	}

	if err := h.svc.DeleteWithdrawal(c.Request.Context(), id, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
