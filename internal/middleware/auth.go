package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zjunaidz/auditd/internal/service"
)

func TenantAuthMiddleware(svc *service.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "Missing Authorization header"})
			return
		}
		apiKey := strings.TrimPrefix(authHeader, "Bearer ")
		if apiKey == authHeader {
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid Authorization header format"})
			return
		}

		tenant, err := svc.ResolveTenant(c.Request.Context(), apiKey)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid API key"})
			return
		}

		c.Set("tenant", tenant)
		c.Next()
	}
}
