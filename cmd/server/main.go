package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zjunaidz/auditd/internal/config"
	"github.com/zjunaidz/auditd/internal/handler"
	"github.com/zjunaidz/auditd/internal/middleware"
	"github.com/zjunaidz/auditd/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	// Initialize database connection
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Successfully connected to the database")

	// Migrate database
	m, err := migrate.New("file://db/migrations", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize migration: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Failed to run migration: %v", err)
	}
	log.Println("Database migration completed")

	svc := service.New(pool, cfg.HMACSecret)
	h := handler.New(svc)

	// Router
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Health
	r.GET("/health", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "db unreachable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Protected routes
	v1 := r.Group("/v1", middleware.TenantAuthMiddleware(svc))
	{
		v1.POST("/events", h.PostEvent)
		v1.GET("/events", h.ListEvents)
	}
	log.Printf("Starting server on %s", cfg.Port)

	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
		return
	}
}
