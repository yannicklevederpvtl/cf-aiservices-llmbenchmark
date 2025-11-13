package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"llmapibenchmark/server"
)

func Run() error {
	// Initialize structured logger first
	server.AppLogger = server.NewLogger()
	
	// Set Gin mode based on environment
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.DebugMode)
	}

	// Create Gin router without default middleware (we use custom middleware)
	router := gin.New()

	// Setup routes with SSE approach
	server.SetupRoutes(router)

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%s", port),
		Handler:        router,
		ReadTimeout:    5 * time.Minute,  // Increased for long-running requests
		WriteTimeout:   0,                // Disabled for SSE connections
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Start server in goroutine
	go func() {
		server.AppLogger.Info("Server starting on port %s", port)
		server.AppLogger.Info("API endpoints available at http://localhost:%s/api", port)
		server.AppLogger.Info("UI available at http://localhost:%s/ui", port)
		server.AppLogger.Info("WebSocket endpoint available at ws://localhost:%s/ws", port)
		server.AppLogger.Info("Async benchmark endpoint: POST /api/benchmark/async", port)
		
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			server.AppLogger.Fatal("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	server.AppLogger.Info("Shutting down server...")

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		server.AppLogger.Error("Server forced to shutdown: %v", err)
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	server.AppLogger.Info("Server exited gracefully")
	return nil
}

