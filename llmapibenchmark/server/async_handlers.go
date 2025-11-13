package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AsyncBenchmarkHandler creates a new benchmark job and returns immediately with job ID
func AsyncBenchmarkHandler(jobManager *JobManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req BenchmarkRequest

		// Parse and validate request
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("ERROR: Failed to parse request JSON: %v", err)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Bad Request",
				Message: fmt.Sprintf("Invalid request payload: %v", err),
				Code:    http.StatusBadRequest,
			})
			return
		}

		log.Printf("DEBUG: Received async benchmark request for model1: %s, model2: %v", req.Model1.Name, req.Model2)
		
		// Enhanced validation
		if validationErr := validateBenchmarkRequest(&req); validationErr != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Validation Error",
				Message: validationErr.Error(),
				Code:    http.StatusBadRequest,
			})
			return
		}

		// Create job
		jobID := jobManager.CreateJob(req)

		// Start job asynchronously
		if err := jobManager.StartJob(jobID); err != nil {
			log.Printf("ERROR: Failed to start job %s: %v", jobID, err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Job Start Error",
				Message: fmt.Sprintf("Failed to start benchmark job: %v", err),
				Code:    http.StatusInternalServerError,
			})
			return
		}

		log.Printf("Started async benchmark job: %s", jobID)

		// Return job information immediately
		c.JSON(http.StatusAccepted, gin.H{
			"jobId":    jobID,
			"status":   "started",
			"message":  "Benchmark job started successfully",
			"websocket": gin.H{
				"url": "/ws",
				"message": "Connect to WebSocket for real-time progress updates",
			},
		})
	}
}

// JobStatusHandler returns the status of a specific job
func JobStatusHandler(jobManager *JobManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID := c.Param("jobId")
		if jobID == "" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Bad Request",
				Message: "Job ID is required",
				Code:    http.StatusBadRequest,
			})
			return
		}

		job, exists := jobManager.GetJob(jobID)
		if !exists {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Not Found",
				Message: fmt.Sprintf("Job %s not found", jobID),
				Code:    http.StatusNotFound,
			})
			return
		}

		response := gin.H{
			"jobId":      job.ID,
			"status":     job.Status,
			"createdAt":  job.CreatedAt,
			"startedAt":  job.StartedAt,
			"completedAt": job.CompletedAt,
		}

		// Add progress information if available
		if job.Progress != nil {
			progress := job.Progress.GetProgress()
			response["progress"] = progress
		}

		// Add results if completed
		if job.Status == "completed" && job.Results != nil {
			response["results"] = job.Results
		}

		// Add error if failed
		if job.Status == "failed" && job.Error != nil {
			response["error"] = job.Error.Error()
		}

		c.JSON(http.StatusOK, response)
	}
}

// CancelJobHandler cancels a running job
func CancelJobHandler(jobManager *JobManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID := c.Param("jobId")
		if jobID == "" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Bad Request",
				Message: "Job ID is required",
				Code:    http.StatusBadRequest,
			})
			return
		}

		if err := jobManager.CancelJob(jobID); err != nil {
			log.Printf("ERROR: Failed to cancel job %s: %v", jobID, err)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Cancellation Error",
				Message: err.Error(),
				Code:    http.StatusBadRequest,
			})
			return
		}

		log.Printf("Cancelled job: %s", jobID)

		c.JSON(http.StatusOK, gin.H{
			"jobId":   jobID,
			"status":  "cancelled",
			"message": "Job cancelled successfully",
		})
	}
}

// ListJobsHandler returns a list of all jobs (for debugging/admin purposes)
func ListJobsHandler(jobManager *JobManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// This would need to be implemented in JobManager
		// For now, return a simple response
		c.JSON(http.StatusOK, gin.H{
			"message": "Job listing not yet implemented",
			"jobs":    []string{},
		})
	}
}

// JobResponse represents the response for job operations
type JobResponse struct {
	JobID     string      `json:"jobId"`
	Status    string      `json:"status"`
	Message   string      `json:"message,omitempty"`
	Progress  interface{} `json:"progress,omitempty"`
	Results   interface{} `json:"results,omitempty"`
	Error     string      `json:"error,omitempty"`
	CreatedAt time.Time   `json:"createdAt"`
	StartedAt *time.Time  `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}
