package handler

import (
	"net/http"

	"github.com/cashvio/cashvio-be/internal/config"
	"github.com/cashvio/cashvio-be/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DashboardHandler struct {
	svc    service.DashboardService
	config *config.Config
}

func NewDashboardHandler(svc service.DashboardService, cfg *config.Config) *DashboardHandler {
	return &DashboardHandler{svc: svc, config: cfg}
}

func (h *DashboardHandler) GetOverview(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	overview, err := h.svc.GetOverview(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": overview})
}
