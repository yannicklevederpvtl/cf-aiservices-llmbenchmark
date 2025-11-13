package server

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all HTTP routes for the server with SSE approach
func SetupRoutes(router *gin.Engine) {
	// Use singleton job manager (Task 15.2 compliance)
	jobManager := GetJobManager()
	sseHandler := NewSSEHandler(jobManager)
	simpleHandlers := NewSimpleHandlers(jobManager)
	
	// Apply global middleware in order
	router.Use(RecoveryMiddleware())      // Recover from panics
	router.Use(SecurityHeadersMiddleware()) // Add security headers
	router.Use(CORSMiddleware())          // Handle CORS
	router.Use(LoggingMiddleware())       // Log requests
	router.Use(ErrorHandlingMiddleware()) // Handle errors

	// API routes group
	api := router.Group("/api")
	{
		// Apply request validation middleware to API routes
		api.Use(RequestValidationMiddleware())

		// Health check endpoint
		api.GET("/health", HealthHandler)


		// System status endpoint
		api.GET("/status", func(c *gin.Context) {
			SystemStatusHandler(c, jobManager)
		})

		// Model discovery endpoint
		api.GET("/models", ModelsHandler)

		// Benchmark execution endpoints
		api.POST("/benchmark", BenchmarkHandler)                    // Synchronous (legacy)
		api.POST("/benchmark/async", simpleHandlers.StartBenchmark) // Asynchronous with SSE

		// Task 15.3: Add specific endpoint for benchmark cancellation
		api.POST("/benchmark/:jobId/cancel", func(c *gin.Context) {
			jobID := c.Param("jobId")
			jobManager := GetJobManager()
			
			AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Received cancellation request for job")
			
			if jobManager.CancelJob(jobID) {
				AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Successfully cancelled job")
				c.JSON(http.StatusOK, gin.H{
					"message": "Benchmark cancelled successfully",
					"jobId": jobID,
					"status": "cancelled",
				})
			} else {
				AppLogger.ErrorWithContext(&LogContext{JobID: jobID}, "Failed to cancel job (not found or not cancellable)")
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Job not found or not cancellable",
					"jobId": jobID,
					"status": "not_found",
				})
			}
		})

		// Job management endpoints
		api.GET("/jobs/:jobId", simpleHandlers.GetJobStatus)
		api.POST("/jobs/:jobId/cancel", simpleHandlers.CancelJob)
		api.GET("/jobs", simpleHandlers.ListJobs)
		
		// SSE endpoint for real-time progress (outside validation middleware)
		api.OPTIONS("/jobs/:jobId/stream", func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Cache-Control")
			c.Status(200)
		})
		api.GET("/jobs/:jobId/stream", sseHandler.StreamJobProgress)
		api.GET("/system-status/stream", sseHandler.StreamSystemStatus)

		// Export endpoints
		api.POST("/export/json", ExportJSONHandler)
		api.POST("/export/csv", ExportCSVHandler)
	}

	// Configure static file serving for Vue.js frontend
	staticPath := os.Getenv("STATIC_PATH")
	if staticPath == "" {
		// Default to ./dist relative to the server binary (frontend copied to same directory)
		staticPath = "dist"
	}

	// Root endpoint - redirect to /ui or show API info
	router.GET("/", func(c *gin.Context) {
		// Check if frontend is built
		indexPath := filepath.Join(staticPath, "index.html")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			// Frontend not built, show API info
			c.JSON(200, gin.H{
				"message": "LLM Benchmark API",
				"version": "1.0.0",
				"status":  "ok",
				"endpoints": gin.H{
					"health":    "/api/health",
					"models":    "/api/models",
					"benchmark": "/api/benchmark",
					"export": gin.H{
						"json": "/api/export/json",
						"csv":  "/api/export/csv",
					},
					"ui": "/ui (frontend not built)",
				},
				"hint": "Build the frontend with: cd ../client && npm run build",
			})
			return
		}

		// Redirect to /ui if frontend is built
		c.Redirect(http.StatusMovedPermanently, "/ui/")
	})

	// Serve static files from /ui path
	router.StaticFS("/ui", http.Dir(staticPath))

	// SPA fallback: serve index.html for non-API, non-static routes
	router.NoRoute(func(c *gin.Context) {
		// If it's an API request, return 404 JSON
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Not Found",
				Message: "The requested endpoint does not exist",
				Code:    http.StatusNotFound,
			})
			return
		}

		// For all other routes, serve index.html (SPA fallback) - not needed with StaticFS
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not Found",
			"message": "The requested resource does not exist",
		})
	})
}

