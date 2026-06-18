package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateTenant(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}
	tenant, apiKey, err := h.svc.CreateTenant(c.Request.Context(), body.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tenant", "details": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":      tenant.ID.String(),
		"tenant":  tenant,
		"api_key": apiKey,
	})
}
