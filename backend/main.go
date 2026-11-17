package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"weather-forecast-database"
	"weather-forecast-system/internal/auth"
	"weather-forecast-system/internal/middleware"
	"weather-forecast-system/internal/server"
)

/*
Package main initializes runtime dependencies, security middleware, and background retention sweeps.
It establishes the operational baseline and lifecycle management required for resilient API serving.
*/

func main() {
	port := getenv("PORT", "8081")
	jwtSecret := []byte(getenv("JWT_SECRET", "atmos-dev-secret-change-me"))
	database.DataDir = getenv("DATA_DIR", "data")
	dbPath := getenv("DB_PATH", filepath.Join("..", "database", "atmos.db"))
	corsOrigin := getenv("CORS_ORIGIN", "http://localhost:5173")

	if err := database.Open(dbPath); err != nil {
		log.Fatalf("database startup failed: %v", err)
	}
	defer database.DB.Close()
	auth.SetSecret(jwtSecret)

	/*
	Periodic retention sweeps prevent unbounded database and disk growth by pruning aged forecast data.
	Running asynchronously ensures routine housekeeping never degrades inbound API response times.
	*/
	go func() {
		t := time.NewTicker(6 * time.Hour)
		defer t.Stop()
		for range t.C {
			database.CleanupRetention()
		}
	}()

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.JSONRecover())
	r.Use(middleware.SecurityHeaders())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{corsOrigin, "http://localhost:5173", "http://127.0.0.1:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	server.RegisterRoutes(r)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Printf("ATMOS backend listening on http://localhost:%s (db: %s)", port, dbPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	/*
	Intercepting process signals allows active in-flight HTTP requests and database handles to flush cleanly.
	This prevents data corruption and broken client connections during server restarts.
	*/
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
}

// getenv provides fallback defaults so the server runs zero-config in dev while remaining cloud configurable.
func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
