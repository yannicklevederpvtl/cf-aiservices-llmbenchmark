package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"llmapibenchmark/internal/utils"
	"github.com/schollz/progressbar/v3"
)

// SSEHandler handles Server-Sent Events for benchmark progress
type SSEHandler struct {
	jobManager *SimpleJobManager
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(jobManager *SimpleJobManager) *SSEHandler {
	return &SSEHandler{
		jobManager: jobManager,
	}
}

// StreamJobProgress streams benchmark progress via SSE
func (h *SSEHandler) StreamJobProgress(c *gin.Context) {
	jobID := c.Param("jobId")
	
	// Get the job
	job, exists := h.jobManager.GetJob(jobID)
	if !exists {
		c.JSON(404, gin.H{"error": "Job not found"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")
	c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
	c.Header("Access-Control-Expose-Headers", "Content-Type")

	// Send initial status
	c.Writer.WriteString(job.ToSSEMessage())
	c.Writer.Flush()

	// If job is already completed, just send the final result
	if job.Status == "completed" || job.Status == "failed" {
		c.Writer.WriteString(job.ToSSEMessage())
		c.Writer.Flush()
		return
	}

	// Don't start the benchmark here - it's already started in StartBenchmark handler
	// The benchmark is running in SimpleJobManager.RunBenchmark()

	// Create a channel for job updates
	updateChan := make(chan *SimpleJob, 10)
	
	// Register this connection for updates
	h.jobManager.RegisterSSEListener(jobID, updateChan)
	defer h.jobManager.UnregisterSSEListener(jobID, updateChan)

	// Listen for updates with keep-alive
	ctx := c.Request.Context()
	ticker := time.NewTicker(30 * time.Second) // Send keep-alive every 30 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "SSE connection closed for job")
			return
		case <-ticker.C:
			// Send keep-alive ping
			c.Writer.WriteString("data: {\"type\":\"ping\",\"timestamp\":\"" + time.Now().Format(time.RFC3339) + "\"}\n\n")
			c.Writer.Flush()
		case updatedJob := <-updateChan:
			// Send update
			message := updatedJob.ToSSEMessage()
			c.Writer.WriteString(message)
			c.Writer.Flush()

			// If job is completed or failed, wait longer before closing
			if updatedJob.Status == "completed" || updatedJob.Status == "failed" {
				// Give the frontend time to process the completion message
				time.Sleep(3 * time.Second)
				// Don't return immediately - let the stream stay open
			}
		}
	}
}

// runBenchmarkWithSSE runs the benchmark and updates job progress via SSE
func (h *SSEHandler) runBenchmarkWithSSE(jobID string, request BenchmarkRequest) {
	AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Starting benchmark for job")
	AppLogger.InfoWithFields("Request details", map[string]interface{}{
		"jobId": jobID,
		"model": request.Model1.Name,
		"concurrency": request.ConcurrencyLevels,
		"maxTokens": request.MaxTokens,
	})

	// Update progress: Starting
	AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Updating progress: 10%% - Initializing benchmark...")
	h.jobManager.UpdateJobProgress(jobID, 10, "Initializing benchmark...")

	// Test latency first (skip for Cloud Foundry deployments)
	AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Updating progress: 20%% - Testing latency...")
	h.jobManager.UpdateJobProgress(jobID, 20, "Testing latency...")
	
	// Skip latency test for Cloud Foundry deployments as the proxy endpoint may not respond to simple GET requests
	var latency float64
	if os.Getenv("VCAP_SERVICES") != "" {
		AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Skipping latency test for Cloud Foundry deployment")
		latency = 0.0 // Set to 0 for Cloud Foundry
	} else {
		var err error
		latency, err = utils.MeasureLatency(request.Model1.BaseURL, 5)
		if err != nil {
			AppLogger.ErrorWithContext(&LogContext{JobID: jobID}, "Latency test failed: %v", err)
			h.jobManager.FailJob(jobID, fmt.Sprintf("Latency test failed: %v", err))
			return
		}
		AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Latency test completed: %v", latency)
	}

	// Prepare results
	var results []ConcurrencyResult
	totalSteps := len(request.ConcurrencyLevels)
	if request.Model2 != nil {
		totalSteps *= 2
	}

	// Run benchmarks for each concurrency level
	for i, concurrency := range request.ConcurrencyLevels {
		progress := 30 + (i * 60 / totalSteps)
		AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Updating progress: %d%% - Testing Model 1 concurrency %d...", progress, concurrency)
		h.jobManager.UpdateJobProgress(jobID, progress, fmt.Sprintf("Testing Model 1 concurrency %d...", concurrency))

		// Create progress bar for this concurrency level
		expectedTokens := concurrency * request.MaxTokens
		bar := progressbar.NewOptions(expectedTokens,
			progressbar.OptionSetWriter(os.Stderr), // Use stderr for progress bar output
			progressbar.OptionSetDescription(fmt.Sprintf("Model1 Concurrency %d", concurrency)),
			progressbar.OptionSetWidth(40),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetItsString("tokens"),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetRenderBlankState(true),
		)

		// Create speed measurement setup
		// Use API key from environment variables for security
		apiKey := getAPIKeyForModel(request.Model1)
		setup := utils.SpeedMeasurement{
			BaseUrl:        request.Model1.BaseURL,
			ApiKey:         apiKey,
			ModelName:      request.Model1.Name,
			Prompt:         request.Prompt,
			UseRandomInput: false, // We're using custom prompt
			NumWords:       request.NumWords,
			MaxTokens:      request.MaxTokens,
			Latency:        latency,
			Concurrency:    concurrency,
		}

		// Run the benchmark (this is the working code!)
		AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Running benchmark for concurrency %d...", concurrency)
		result, err := setup.Run(context.Background(), bar)
		if err != nil {
			AppLogger.ErrorWithContext(&LogContext{JobID: jobID}, "Benchmark failed for concurrency %d: %v", concurrency, err)
			h.jobManager.FailJob(jobID, fmt.Sprintf("Benchmark failed for concurrency %d: %v", concurrency, err))
			bar.Close()
			return
		}

		// Clean up progress bar
		bar.Finish()
		bar.Close()

		// Convert SpeedResult to ConcurrencyResult for frontend compatibility
		concurrencyResult := ConcurrencyResult{
			Concurrency:          result.Concurrency,
			GenerationThroughput: result.GenerationSpeed,
			PromptThroughput:     result.PromptThroughput,
			MinTTFT:              result.MinTtft,
			MaxTTFT:              result.MaxTtft,
		}
		results = append(results, concurrencyResult)
		AppLogger.InfoWithFields("Completed benchmark for concurrency", map[string]interface{}{
			"jobId": jobID,
			"concurrency": concurrency,
			"generationSpeed": result.GenerationSpeed,
		})
	}

	// Store Model1 results
	model1Results := results
	var model2Results []ConcurrencyResult

	// Process Model2 if provided
	if request.Model2 != nil {
		AppLogger.InfoWithContext(&LogContext{JobID: jobID, Model: request.Model2.Name}, "Starting Model 2 benchmark")
		h.jobManager.UpdateJobProgress(jobID, 70, fmt.Sprintf("Testing Model 2: %s", request.Model2.Name))
		
		// Reset results for Model2
		results = []ConcurrencyResult{}
		
		// Run benchmarks for Model2
		for i, concurrency := range request.ConcurrencyLevels {
			progress := 70 + (i * 20 / len(request.ConcurrencyLevels))
			AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Updating progress: %d%% - Testing Model 2 concurrency %d...", progress, concurrency)
			h.jobManager.UpdateJobProgress(jobID, progress, fmt.Sprintf("Testing Model 2 concurrency %d...", concurrency))

			// Create progress bar for this concurrency level
			expectedTokens := concurrency * request.MaxTokens
			bar := progressbar.NewOptions(expectedTokens,
				progressbar.OptionSetWriter(os.Stderr),
				progressbar.OptionSetDescription(fmt.Sprintf("Model2 Concurrency %d", concurrency)),
				progressbar.OptionSetWidth(40),
				progressbar.OptionShowCount(),
				progressbar.OptionShowIts(),
				progressbar.OptionSetItsString("tokens"),
				progressbar.OptionSpinnerType(14),
				progressbar.OptionSetRenderBlankState(true),
			)

			// Create speed measurement setup for Model2
			apiKey := getAPIKeyForModel(*request.Model2)
			setup := utils.SpeedMeasurement{
				BaseUrl:        request.Model2.BaseURL,
				ApiKey:         apiKey,
				ModelName:      request.Model2.Name,
				Prompt:         request.Prompt,
				UseRandomInput: false,
				NumWords:       request.NumWords,
				MaxTokens:      request.MaxTokens,
				Latency:        latency,
				Concurrency:    concurrency,
			}

			// Run the benchmark for Model2
			AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Running Model 2 benchmark for concurrency %d...", concurrency)
			result, err := setup.Run(context.Background(), bar)
			if err != nil {
				AppLogger.ErrorWithContext(&LogContext{JobID: jobID}, "Model 2 benchmark failed for concurrency %d: %v", concurrency, err)
				h.jobManager.FailJob(jobID, fmt.Sprintf("Model 2 benchmark failed for concurrency %d: %v", concurrency, err))
				bar.Close()
				return
			}

			// Clean up progress bar
			bar.Finish()
			bar.Close()

			// Convert SpeedResult to ConcurrencyResult for frontend compatibility
			concurrencyResult := ConcurrencyResult{
				Concurrency:          result.Concurrency,
				GenerationThroughput: result.GenerationSpeed,
				PromptThroughput:     result.PromptThroughput,
				MinTTFT:              result.MinTtft,
				MaxTTFT:              result.MaxTtft,
			}
			results = append(results, concurrencyResult)
			AppLogger.InfoWithFields("Completed Model 2 benchmark for concurrency", map[string]interface{}{
				"jobId": jobID,
				"concurrency": concurrency,
				"generationSpeed": result.GenerationSpeed,
			})
		}
		
		model2Results = results
		AppLogger.InfoWithContext(&LogContext{JobID: jobID, Model: request.Model2.Name}, "Model 2 benchmark completed")
	}

	// Update progress: Completed
	AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Updating progress: 90%% - Processing results...")
	h.jobManager.UpdateJobProgress(jobID, 90, "Processing results...")

	// Create final result
	benchmarkResult := map[string]interface{}{
		"model1": map[string]interface{}{
			"model":   request.Model1.Name,
			"results": model1Results,
		},
		"timestamp": time.Now(),
	}
	
	// Add Model2 results if available
	if request.Model2 != nil {
		benchmarkResult["model2"] = map[string]interface{}{
			"model":   request.Model2.Name,
			"results": model2Results,
		}
	}

	// Mark job as completed
	AppLogger.InfoWithFields("Marking job as completed with results", map[string]interface{}{
		"jobId": jobID,
		"results": benchmarkResult,
	})
	h.jobManager.CompleteJob(jobID, benchmarkResult)
	
	// Wait for completion message to be sent to SSE stream
	time.Sleep(1 * time.Second)
	
	AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Benchmark completed successfully for job")
}

// StreamSystemStatus streams global system status via SSE
func (h *SSEHandler) StreamSystemStatus(c *gin.Context) {
	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")
	c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
	c.Header("Access-Control-Expose-Headers", "Content-Type")

	// Register system status listener
	listener := h.jobManager.RegisterSystemStatusListener()
	defer h.jobManager.UnregisterSystemStatusListener(listener)

	AppLogger.Info("System status SSE connection established")

	// Keep connection alive and stream status updates
	for {
		select {
		case status, ok := <-listener:
			if !ok {
				AppLogger.Info("System status SSE connection closed")
				return
			}

			// Serialize status to JSON
			statusJSON, err := json.Marshal(status)
			if err != nil {
				AppLogger.Error("Error marshaling system status: %v", err)
				continue
			}

			// Send system status as SSE message
			message := fmt.Sprintf("data: %s\n\n", string(statusJSON))
			if _, err := c.Writer.WriteString(message); err != nil {
				AppLogger.Error("Error writing system status SSE: %v", err)
				return
			}
			c.Writer.Flush()

		case <-c.Request.Context().Done():
			AppLogger.Info("System status SSE connection closed by client")
			return
		}
	}
}
