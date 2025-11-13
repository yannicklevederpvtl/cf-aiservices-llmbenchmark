package server

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SimpleHandlers contains simple HTTP handlers for the SSE approach
type SimpleHandlers struct {
	jobManager *SimpleJobManager
}

// NewSimpleHandlers creates new simple handlers
func NewSimpleHandlers(jobManager *SimpleJobManager) *SimpleHandlers {
	return &SimpleHandlers{
		jobManager: jobManager,
	}
}

// StartBenchmark starts a new benchmark job and returns the job ID
func (h *SimpleHandlers) StartBenchmark(c *gin.Context) {
	AppLogger.InfoWithFields("StartBenchmark received request", map[string]interface{}{
		"clientIP": c.ClientIP(),
	})
	AppLogger.DebugWithFields("StartBenchmark request headers", map[string]interface{}{
		"headers": c.Request.Header,
	})
	
	// Log raw request body
	body, _ := c.GetRawData()
	AppLogger.DebugWithFields("StartBenchmark raw request body", map[string]interface{}{
		"body": string(body),
	})
	
	// Reset body for binding
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	
	var request BenchmarkRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		AppLogger.Error("StartBenchmark failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	AppLogger.DebugWithFields("StartBenchmark parsed request", map[string]interface{}{
		"request": request,
	})
	AppLogger.DebugWithFields("StartBenchmark Model1", map[string]interface{}{
		"model1": request.Model1,
	})
	AppLogger.DebugWithFields("StartBenchmark Model2", map[string]interface{}{
		"model2": request.Model2,
	})
	AppLogger.DebugWithFields("StartBenchmark ConcurrencyLevels", map[string]interface{}{
		"concurrencyLevels": request.ConcurrencyLevels,
	})
	AppLogger.DebugWithFields("StartBenchmark MaxTokens", map[string]interface{}{
		"maxTokens": request.MaxTokens,
	})
	AppLogger.DebugWithFields("StartBenchmark Prompt", map[string]interface{}{
		"prompt": request.Prompt,
	})
	AppLogger.DebugWithFields("StartBenchmark NumWords", map[string]interface{}{
		"numWords": request.NumWords,
	})

	// Validate request
	if request.Model1.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model 1 is required"})
		return
	}

	if len(request.ConcurrencyLevels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one concurrency level is required"})
		return
	}

	// Create job
	jobID := h.jobManager.CreateJob(request)
	AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Created job for asynchronous benchmark")

	// Start the benchmark execution immediately (don't wait for SSE connection)
	go h.jobManager.RunBenchmark(jobID, request)
	AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Started benchmark execution for job")

	// Return job ID and SSE endpoint
	c.JSON(http.StatusAccepted, gin.H{
		"jobId": jobID,
		"message": "Benchmark job started successfully",
		"status": "started",
		"sse": gin.H{
			"url": "/api/jobs/" + jobID + "/stream",
			"message": "Connect to SSE endpoint for real-time progress updates",
		},
	})
}

// GetJobStatus returns the current status of a job
func (h *SimpleHandlers) GetJobStatus(c *gin.Context) {
	jobID := c.Param("jobId")
	
	job, exists := h.jobManager.GetJob(jobID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// ListJobs returns all jobs
func (h *SimpleHandlers) ListJobs(c *gin.Context) {
	jobs := h.jobManager.ListJobs()
	c.JSON(http.StatusOK, gin.H{
		"jobs": jobs,
		"count": len(jobs),
	})
}

// CancelJob cancels a running job (enhanced error handling for Task 15.3)
func (h *SimpleHandlers) CancelJob(c *gin.Context) {
	jobID := c.Param("jobId")
	
	AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Received cancellation request for job")
	
	// Use the new CancelJob method that actually cancels the context
	if h.jobManager.CancelJob(jobID) {
		AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Successfully cancelled job")
		c.JSON(http.StatusOK, gin.H{
			"message": "Job cancelled successfully",
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
}

// CleanupJobs removes old jobs
func (h *SimpleHandlers) CleanupJobs(c *gin.Context) {
	h.jobManager.CleanupOldJobs()
	c.JSON(http.StatusOK, gin.H{"message": "Old jobs cleaned up"})
}
