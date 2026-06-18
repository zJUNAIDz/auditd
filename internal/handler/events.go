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

func (h *Handler) ListEventsFiltered(c *gin.Context) {
	tenant := c.MustGet("tenant").(*db.GetTenantByAPIKeyRow)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	params := db.ListEventsFileredParams{
		TenantID:   tenant.ID,
		PageLimit:  int32(limit),
		PageOffset: int32(offset),
	}

	if v := c.Query("actor_id"); v != "" {
		params.ActorID = v
	}
	if v := c.Query("action"); v != "" {
		params.Action = v
	}
	if v := c.Query("resource_id"); v != "" {
		params.ResourceID = v
	}
	if v := c.Query("resource_type"); v != "" {
		params.ResourceType = v
	}
	if v := c.Query("start_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.StartTime = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if v := c.Query("end_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.EndTime = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	events, err := h.svc.ListEventsFiltered(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list events", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"events": events, "count": len(events)})
}

func (h *Handler) VerifyChain(c *gin.Context) {
	tenant := c.MustGet("tenant").(*db.GetTenantByAPIKeyRow)
	tenantID, _ := uuid.FromBytes(tenant.ID.Bytes[:])

	result, err := h.svc.VerifyChain(c.Request.Context(), tenantID, tenant.HmacSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify chain", "details": err.Error()})
		return
	}
	status := http.StatusOK
	if !result.Verified {
		status = http.StatusConflict
	}
	c.JSON(status, result)
}	