package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/zjunaidz/auditd/internal/db"
	"github.com/zjunaidz/auditd/internal/model"
)

func (h *Handler) PostEvent(c *gin.Context) {
	var input model.EventInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if input.Metadata == nil {
		input.Metadata = make(map[string]interface{})
	}

	tenant := c.MustGet("tenant").(*db.GetTenantByAPIKeyRow)
	tenantID, _ := uuid.FromBytes(tenant.ID.Bytes[:])

	payload := model.IngestPayload{
		Input:     input,
		TenantID:  tenantID,
		Timestamp: time.Now().UTC(),
		ID:        uuid.New(),
	}

	if !h.queue.Enqueue(payload) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Event queue is full, please try again later"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": payload.ID.String(), "status": "accepted"})
}

func (h *Handler) ListEvents(c *gin.Context) {
	tenant := c.MustGet("tenant").(*db.GetTenantByAPIKeyRow)
	tenantID, _ := uuid.FromBytes(tenant.ID.Bytes[:])

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	events, err := h.svc.ListEvents(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list events", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": events, "count": len(events)})
}
