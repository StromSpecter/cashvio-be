package handler

import (
	"net/http"

	"github.com/cashvio/cashvio-be/internal/config"
	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InvestmentHandler struct {
	svc    service.InvestmentService
	config *config.Config
}

func NewInvestmentHandler(svc service.InvestmentService, cfg *config.Config) *InvestmentHandler {
	return &InvestmentHandler{svc: svc, config: cfg}
}

func (h *InvestmentHandler) CreateInvestment(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	var req model.CreateInvestmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	inv, err := h.svc.CreateInvestment(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": inv})
}

func (h *InvestmentHandler) GetPrices(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	prices, err := h.svc.GetPrices(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": prices})
}

func (h *InvestmentHandler) GetInvestments(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	q := model.NewInvestmentQuery()
	q = service.ParseInvestmentQuery(q,
		c.Query("limit"),
		c.Query("offset"),
		c.Query("search"),
		c.Query("sort_by"),
		c.Query("order"),
		c.Query("type"),
	)

	investments, total, err := h.svc.GetAllInvestments(c.Request.Context(), q, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       investments,
		"pagination": model.NewPagination(total, q.Limit, q.Offset),
	})
}

func (h *InvestmentHandler) GetInvestment(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid investment id"})
		return
	}

	inv, err := h.svc.GetInvestmentByID(c.Request.Context(), id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": inv})
}

func (h *InvestmentHandler) UpdateInvestment(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid investment id"})
		return
	}

	var req model.UpdateInvestmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	inv, err := h.svc.UpdateInvestment(c.Request.Context(), id, userID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": inv})
}

func (h *InvestmentHandler) DeleteInvestment(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid investment id"})
		return
	}

	if err := h.svc.DeleteInvestment(c.Request.Context(), id, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
