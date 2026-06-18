package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zjunaidz/auditd/internal/config"
	"github.com/zjunaidz/auditd/internal/handler"
	"github.com/zjunaidz/auditd/internal/middleware"
	"github.com/zjunaidz/auditd/internal/queue"
	"github.com/zjunaidz/auditd/internal/service"
	"github.com/zjunaidz/auditd/internal/worker"
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

	// Queue + Worker pool
	q := queue.New(100)
	pool4Workers := worker.New(q.Chan(), svc, 4)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	pool4Workers.Start(ctx, &wg)
	h := handler.New(svc, q)

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

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	go func() {
		log.Printf("Server is running on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// graceful shutdown

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exiting...")
	cancel()  // signal workers to drain and stop
	wg.Wait() // wait for workers to finish
	log.Println("All workers stopped, exiting now.")
}
