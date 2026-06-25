package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/zjunaidz/auditd/internal/db"
)

func (h *Handler) ExportEvents(c *gin.Context) {
	tenant := c.MustGet("tenant").(db.GetTenantByAPIKeyRow)

	fromStr := c.Query("from")
	toStr := c.Query("to")

	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'from' or 'to' query parameters"})
		return
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'from' timestamp format"})
		return
	}

	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'to' timestamp format"})
		return
	}

	// we stream instead of loading entire query into memory
	format := c.DefaultQuery("format", "csv")

	switch format {
	case "csv":
		h.streamCSV(c, tenant, from, to)
	case "jsonl":
		h.streamJSONL(c, tenant, from, to)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format"})
	}
}

func (h *Handler) streamCSV(c *gin.Context, tenant db.GetTenantByAPIKeyRow, from, to time.Time) {
	fileName := fmt.Sprintf("audit-export-%s.csv", time.Now().Format("20060102-150405"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))

	rows, err := h.svc.StreamEventsForExport(c.Request.Context(), tenant.ID.Bytes, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stream events"})
		return
	}
	defer rows.Close()

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	// first row is header
	w.Write([]string{
		"id", "timestamp", "actor_id", "actor_type", "action",
		"resource_type", "resource_id", "metadata", "hash",
	})
	flusher, canFlush := c.Writer.(http.Flusher)

	for rows.Next(){
		var e db.AuditEvent
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.ActorID, &e.ActorType, &e.Action,
			&e.ResourceType, &e.ResourceID, &e.Metadata,
			&e.Timestamp, &e.PrevHash, &e.Hash, &e.CreatedAt,
		); err != nil {
			// log error and continue
			continue
		}
		w.Write([]string{
			uuidString(e.ID),
			e.Timestamp.Time.Format(time.RFC3339),
			e.ActorID,
			e.ActorType,
			e.Action,
			e.ResourceType,
			e.ResourceID,
			string(e.Metadata),
			e.Hash,
		})
		w.Flush()	
		if canFlush {
			flusher.Flush()
		}
	}
}

func (h *Handler) streamJSONL(c *gin.Context, tenant db.GetTenantByAPIKeyRow, from, to time.Time) {
	filename := fmt.Sprintf("audit-export-%s.ndjson", time.Now().Format("20060102-150405"))
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Content-Disposition", "attachment; filename="+filename)

	rows, err := h.svc.StreamEventsForExport(c.Request.Context(), tenant.ID.Bytes, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "export failed"})
		return
	}
	defer rows.Close()

	flusher, canFlush := c.Writer.(http.Flusher)
	enc := json.NewEncoder(c.Writer)

	for rows.Next() {
		var e db.AuditEvent
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.ActorID, &e.ActorType, &e.Action,
			&e.ResourceType, &e.ResourceID, &e.Metadata,
			&e.Timestamp, &e.PrevHash, &e.Hash, &e.CreatedAt,
		); err != nil {
			log.Print(err)
			continue
		}
		enc.Encode(e) // each call writes one line + newline
		if canFlush {
			flusher.Flush()
		}
	}
}

func uuidString(id pgtype.UUID) string {
	u, _ := id.UUIDValue()
	_ = u
	return id.String()
}