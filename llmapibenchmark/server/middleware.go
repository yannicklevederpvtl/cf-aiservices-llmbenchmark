package server

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns default CORS configuration
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "accept", "origin", "Cache-Control", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours
	}
}

// LoadCORSConfigFromEnv loads CORS configuration from environment variables
func LoadCORSConfigFromEnv() CORSConfig {
	config := DefaultCORSConfig()

	// Check for custom CORS origins (prioritize CORS_ORIGIN for CF deployment)
	if origins := os.Getenv("CORS_ORIGIN"); origins != "" {
		config.AllowOrigins = strings.Split(origins, ",")
		for i, origin := range config.AllowOrigins {
			config.AllowOrigins[i] = strings.TrimSpace(origin)
		}
	} else if origins := os.Getenv("CORS_ALLOW_ORIGINS"); origins != "" {
		config.AllowOrigins = strings.Split(origins, ",")
		for i, origin := range config.AllowOrigins {
			config.AllowOrigins[i] = strings.TrimSpace(origin)
		}
	}

	// Check for custom CORS methods
	if methods := os.Getenv("CORS_ALLOW_METHODS"); methods != "" {
		config.AllowMethods = strings.Split(methods, ",")
		for i, method := range config.AllowMethods {
			config.AllowMethods[i] = strings.TrimSpace(method)
		}
	}

	// Production mode: restrict CORS if not explicitly configured
	if os.Getenv("GIN_MODE") == "release" && len(config.AllowOrigins) == 1 && config.AllowOrigins[0] == "*" {
		// In production, default to allowing only the CF app domain
		// This will be overridden by explicit CORS_ORIGIN setting
		AppLogger.Warn("CORS is set to allow all origins in production mode. Consider setting CORS_ORIGIN environment variable.")
	}

	return config
}

// CORSMiddleware adds CORS headers to allow frontend access
func CORSMiddleware() gin.HandlerFunc {
	config := LoadCORSConfigFromEnv()

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Set CORS headers
		if len(config.AllowOrigins) == 1 && config.AllowOrigins[0] == "*" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			// Check if origin is allowed
			for _, allowedOrigin := range config.AllowOrigins {
				if allowedOrigin == origin || allowedOrigin == "*" {
					c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
		c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))
		c.Writer.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))

		if config.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// LoggingMiddleware logs request details with structured format
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		startTime := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate request duration
		duration := time.Since(startTime)

		// Get status code
		statusCode := c.Writer.Status()

		// Determine log level based on status code
		logLevel := "INFO"
		if statusCode >= 500 {
			logLevel = "ERROR"
		} else if statusCode >= 400 {
			logLevel = "WARN"
		}

		// Build log message
		logMsg := fmt.Sprintf(
			"[%s] %s | %s %s | Status: %d | Duration: %v | IP: %s | User-Agent: %s",
			logLevel,
			time.Now().Format("2006-01-02 15:04:05"),
			c.Request.Method,
			path,
			statusCode,
			duration,
			c.ClientIP(),
			c.Request.UserAgent(),
		)

		if query != "" {
			logMsg += fmt.Sprintf(" | Query: %s", query)
		}

		// Add error message if present
		if len(c.Errors) > 0 {
			logMsg += fmt.Sprintf(" | Errors: %s", c.Errors.String())
		}

		AppLogger.Info(logMsg)
	}
}

// ErrorHandlingMiddleware handles errors and formats them as JSON
func ErrorHandlingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			// Determine status code
			statusCode := c.Writer.Status()
			if statusCode == http.StatusOK {
				statusCode = http.StatusInternalServerError
			}

			// Format error response
			errorResponse := ErrorResponse{
				Error:   http.StatusText(statusCode),
				Message: err.Error(),
				Code:    statusCode,
			}

			c.JSON(statusCode, errorResponse)
		}
	}
}

// RecoveryMiddleware recovers from panics and returns a 500 error
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				AppLogger.ErrorWithFields("PANIC RECOVERED", map[string]interface{}{
				"error": err,
				"stack": string(debug.Stack()),
			})

				// Return 500 error
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Internal Server Error",
					Message: "An unexpected error occurred. Please try again later.",
					Code:    http.StatusInternalServerError,
				})

				c.Abort()
			}
		}()

		c.Next()
	}
}

// RequestValidationMiddleware validates common request requirements
func RequestValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate Content-Type for POST/PUT requests
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			contentType := c.GetHeader("Content-Type")
			
			// Check if it's a JSON endpoint
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				if !strings.Contains(contentType, "application/json") {
					c.JSON(http.StatusUnsupportedMediaType, ErrorResponse{
						Error:   "Unsupported Media Type",
						Message: "Content-Type must be application/json",
						Code:    http.StatusUnsupportedMediaType,
					})
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}

// SecurityHeadersMiddleware adds security-related HTTP headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Only add HSTS in production
		if os.Getenv("GIN_MODE") == "release" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}

