package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cashvio/cashvio-be/internal/config"
	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TransactionHandler struct {
	svc    service.TransactionService
	config *config.Config
}

func NewTransactionHandler(svc service.TransactionService, cfg *config.Config) *TransactionHandler {
	return &TransactionHandler{svc: svc, config: cfg}
}

func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	var req model.CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txn, err := h.svc.CreateTransaction(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": txn})
}

func (h *TransactionHandler) GetTransactions(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	q := model.NewTransactionQuery()
	q = service.ParseTransactionQuery(q,
		c.Query("limit"),
		c.Query("offset"),
		c.Query("search"),
		c.Query("sort_by"),
		c.Query("order"),
		c.Query("type"),
		c.Query("category"),
		c.Query("status"),
	)

	transactions, total, err := h.svc.GetAllTransactions(c.Request.Context(), q, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       transactions,
		"pagination": model.NewPagination(total, q.Limit, q.Offset),
	})
}

func (h *TransactionHandler) DownloadTransactions(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	q := model.NewTransactionQuery()
	q = service.ParseTransactionQuery(q,
		c.Query("limit"),
		c.Query("offset"),
		c.Query("search"),
		c.Query("sort_by"),
		c.Query("order"),
		c.Query("type"),
		c.Query("category"),
		c.Query("status"),
	)

	transactions, err := h.svc.ExportTransactions(c.Request.Context(), q, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("transactions_%s.csv", time.Now().Format("20060102-150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	c.Writer.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(c.Writer)
	w.Comma = ';'
	defer w.Flush()

	if err := w.Write([]string{"date", "name", "category", "type", "status", "account_type", "amount"}); err != nil {
		return
	}
	for _, txn := range transactions {
		record := []string{
			txn.Date.Format("2006-01-02"),
			txn.Name,
			txn.Category,
			txn.Type,
			txn.Status,
			txn.AccountType,
			strconv.FormatFloat(txn.Amount, 'f', -1, 64),
		}
		if err := w.Write(record); err != nil {
			return
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return
	}
}

func (h *TransactionHandler) GetTransaction(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction id"})
		return
	}

	txn, err := h.svc.GetTransactionByID(c.Request.Context(), id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": txn})
}

func (h *TransactionHandler) UpdateTransaction(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction id"})
		return
	}

	var req model.UpdateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txn, err := h.svc.UpdateTransaction(c.Request.Context(), id, userID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": txn})
}

func (h *TransactionHandler) DeleteTransaction(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction id"})
		return
	}

	if err := h.svc.DeleteTransaction(c.Request.Context(), id, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
