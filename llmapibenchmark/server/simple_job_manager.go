package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
	"llmapibenchmark/internal/utils"
)

// Singleton pattern for JobManager (Task 15.2 compliance)
var (
	jobManagerInstance *SimpleJobManager
	jobManagerOnce     sync.Once
)

// SimpleJob represents a benchmark job with basic status tracking
type SimpleJob struct {
	ID          string                 `json:"id"`
	Status      string                 `json:"status"` // "running", "completed", "failed", "cancelled"
	Progress    int                    `json:"progress"` // 0-100
	Message     string                 `json:"message"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	CompletedAt *time.Time             `json:"completedAt,omitempty"`
	Request     BenchmarkRequest       `json:"request"`
	// Context and cancellation for proper job cancellation
	ctx         context.Context        `json:"-"`
	cancelFunc  context.CancelFunc     `json:"-"`
}

// JobState represents the state of a job (for Task 15.2 compliance)
type JobState struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

// SimpleJobManager manages benchmark jobs with minimal complexity
type SimpleJobManager struct {
	jobs                    map[string]*SimpleJob
	listeners               map[string][]chan *SimpleJob
	systemStatusListeners   []chan map[string]interface{} // For system status SSE
	activeJobCount          int // Global counter for active jobs
	mutex                   sync.RWMutex
}

// NewSimpleJobManager creates a new simple job manager
func NewSimpleJobManager() *SimpleJobManager {
	return &SimpleJobManager{
		jobs:      make(map[string]*SimpleJob),
		listeners: make(map[string][]chan *SimpleJob),
	}
}

// GetJobManager returns the singleton JobManager instance (Task 15.2 compliance)
func GetJobManager() *SimpleJobManager {
	jobManagerOnce.Do(func() {
		jobManagerInstance = NewSimpleJobManager()
		AppLogger.Info("Singleton JobManager instance created")
	})
	return jobManagerInstance
}

// CreateJob creates a new job and returns its ID
func (jm *SimpleJobManager) CreateJob(request BenchmarkRequest) string {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	jobID := uuid.New().String()
	job := &SimpleJob{
		ID:        jobID,
		Status:    "running",
		Progress:  0,
		Message:   "Starting benchmark...",
		CreatedAt: time.Now(),
		Request:   request,
	}

	jm.jobs[jobID] = job
	jm.activeJobCount++
	AppLogger.InfoWithFields("Job created", map[string]interface{}{
		"jobId": jobID,
		"activeJobs": jm.activeJobCount,
	})
	
	// Broadcast system status change
	go jm.broadcastSystemStatus()
	
	return jobID
}

// GetJob retrieves a job by ID
func (jm *SimpleJobManager) GetJob(jobID string) (*SimpleJob, bool) {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	job, exists := jm.jobs[jobID]
	return job, exists
}

// SetJobContext sets the context and cancel function for a job
func (jm *SimpleJobManager) SetJobContext(jobID string, ctx context.Context, cancelFunc context.CancelFunc) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	if job, exists := jm.jobs[jobID]; exists {
		job.ctx = ctx
		job.cancelFunc = cancelFunc
		AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Job context set for cancellation")
	}
}

// GetJobContext retrieves the context for a job
func (jm *SimpleJobManager) GetJobContext(jobID string) (context.Context, bool) {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	if job, exists := jm.jobs[jobID]; exists && job.ctx != nil {
		return job.ctx, true
	}
	return nil, false
}

// UpdateJobProgress updates job progress and message
func (jm *SimpleJobManager) UpdateJobProgress(jobID string, progress int, message string) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	if job, exists := jm.jobs[jobID]; exists {
		job.Progress = progress
		job.Message = message
		
		AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Job progress updated: %d%% - %s", progress, message)
		
		// Broadcast update to SSE listeners
		jm.broadcastUpdate(jobID, job)
	} else {
		AppLogger.ErrorWithContext(&LogContext{JobID: jobID}, "Job not found for progress update")
	}
}

// CompleteJob marks a job as completed with results
func (jm *SimpleJobManager) CompleteJob(jobID string, result interface{}) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	if job, exists := jm.jobs[jobID]; exists {
		job.Status = "completed"
		job.Progress = 100
		job.Message = "Benchmark completed successfully"
		job.Result = result
		now := time.Now()
		job.CompletedAt = &now
		
		// Decrement active job counter
		if jm.activeJobCount > 0 {
			jm.activeJobCount--
		}
		
		AppLogger.InfoWithFields("Job completed successfully", map[string]interface{}{
			"jobId": jobID,
			"status": job.Status,
			"progress": job.Progress,
			"message": job.Message,
			"activeJobs": jm.activeJobCount,
		})
		
		// Broadcast update to SSE listeners
		jm.broadcastUpdate(jobID, job)
		
		// Broadcast system status change
		go jm.broadcastSystemStatus()
	} else {
		AppLogger.ErrorWithContext(&LogContext{JobID: jobID}, "Job not found for completion")
	}
}

// FailJob marks a job as failed with error message
func (jm *SimpleJobManager) FailJob(jobID string, errorMsg string) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	if job, exists := jm.jobs[jobID]; exists {
		job.Status = "failed"
		job.Message = "Benchmark failed"
		job.Error = errorMsg
		now := time.Now()
		job.CompletedAt = &now
		
		// Decrement active job counter
		if jm.activeJobCount > 0 {
			jm.activeJobCount--
		}
		
		AppLogger.ErrorWithFields("Job failed", map[string]interface{}{
			"jobId": jobID,
			"status": job.Status,
			"message": job.Message,
			"error": job.Error,
			"activeJobs": jm.activeJobCount,
		})
		
		// Broadcast update to SSE listeners
		jm.broadcastUpdate(jobID, job)
		
		// Broadcast system status change
		go jm.broadcastSystemStatus()
	} else {
		AppLogger.ErrorWithContext(&LogContext{JobID: jobID}, "Job not found for failure")
	}
}

// CancelJob cancels a running job by cancelling its context
func (jm *SimpleJobManager) CancelJob(jobID string) bool {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	if job, exists := jm.jobs[jobID]; exists {
		if job.Status == "running" && job.cancelFunc != nil {
		// Cancel the context to stop the benchmark execution
		job.cancelFunc()
		job.Status = "cancelled"
		job.Message = "Job cancelled by user"
		job.Error = "Job cancelled by user"
		now := time.Now()
		job.CompletedAt = &now
		jm.activeJobCount--
			AppLogger.InfoWithFields("Job cancelled", map[string]interface{}{
				"jobId": jobID,
				"activeJobs": jm.activeJobCount,
			})
		
		// Broadcast cancellation update to SSE listeners
		AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Broadcasting cancellation to SSE listeners")
		jm.broadcastUpdate(jobID, job)
			
			// Broadcast system status change
			go jm.broadcastSystemStatus()
			
			return true
		} else {
			AppLogger.WarnWithContext(&LogContext{JobID: jobID}, "Job cannot be cancelled (status: %s)", job.Status)
			return false
		}
	} else {
		AppLogger.ErrorWithContext(&LogContext{JobID: jobID}, "Job not found for cancellation")
		return false
	}
}

// AddJob adds a job with context and cancellation function (Task 15.2 compliance)
func (jm *SimpleJobManager) AddJob(jobID string, ctx context.Context, cancelFunc context.CancelFunc) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()
	
	if job, exists := jm.jobs[jobID]; exists {
		job.ctx = ctx
		job.cancelFunc = cancelFunc
		job.Status = "running"
		AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Job context and cancellation function added")
	} else {
		AppLogger.WarnWithContext(&LogContext{JobID: jobID}, "Job not found for AddJob")
	}
}

// GetJobState returns job state information (Task 15.2 compliance)
func (jm *SimpleJobManager) GetJobState(jobID string) (JobState, bool) {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()
	
	if job, exists := jm.jobs[jobID]; exists {
		return JobState{
			ID:        job.ID,
			Status:    job.Status,
			Progress:  job.Progress,
			Message:   job.Message,
			CreatedAt: job.CreatedAt,
		}, true
	}
	return JobState{}, false
}

// RemoveJob removes a job from the registry (Task 15.2 compliance)
func (jm *SimpleJobManager) RemoveJob(jobID string) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()
	
	if job, exists := jm.jobs[jobID]; exists {
		// Clean up context if still running
		if job.Status == "running" && job.cancelFunc != nil {
			job.cancelFunc()
			AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Job cancelled during removal")
		}
		delete(jm.jobs, jobID)
		if jm.activeJobCount > 0 {
			jm.activeJobCount--
		}
		AppLogger.InfoWithFields("Job removed from registry", map[string]interface{}{
			"jobId": jobID,
			"activeJobs": jm.activeJobCount,
		})
		
		// Broadcast system status change
		go jm.broadcastSystemStatus()
	} else {
		AppLogger.WarnWithContext(&LogContext{JobID: jobID}, "Job not found for removal")
	}
}

// ListActiveJobs returns list of running job IDs (Task 15.2 compliance)
func (jm *SimpleJobManager) ListActiveJobs() []string {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()
	
	var activeJobs []string
	for id, job := range jm.jobs {
		if job.Status == "running" {
			activeJobs = append(activeJobs, id)
		}
	}
	return activeJobs
}

// GetJobCount returns total number of active jobs (Task 15.2 compliance)
func (jm *SimpleJobManager) GetJobCount() int {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()
	return jm.activeJobCount
}

// ListJobs returns all jobs
func (jm *SimpleJobManager) ListJobs() []*SimpleJob {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	jobs := make([]*SimpleJob, 0, len(jm.jobs))
	for _, job := range jm.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// CleanupOldJobs removes jobs older than 1 hour
func (jm *SimpleJobManager) CleanupOldJobs() {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	for id, job := range jm.jobs {
		if job.CreatedAt.Before(cutoff) {
			delete(jm.jobs, id)
		}
	}
}

// ToJSON converts job to JSON for SSE streaming
func (job *SimpleJob) ToJSON() ([]byte, error) {
	// Create a copy of the job to sanitize NaN/Inf values
	jobCopy := *job
	
	// Sanitize the result if it contains benchmark data
	if jobCopy.Result != nil {
		if resultMap, ok := jobCopy.Result.(map[string]interface{}); ok {
			AppLogger.DebugWithContext(&LogContext{JobID: job.ID}, "Sanitizing benchmark result before JSON marshal")
			jobCopy.Result = sanitizeBenchmarkResult(resultMap)
		} else {
			AppLogger.DebugWithContext(&LogContext{JobID: job.ID}, "Result is not a map[string]interface{}, type: %T", jobCopy.Result)
		}
	} else {
		AppLogger.DebugWithContext(&LogContext{JobID: job.ID}, "Result is nil")
	}
	
	// Try to marshal and catch any remaining +Inf/NaN values
	data, err := json.Marshal(jobCopy)
	if err != nil {
		AppLogger.ErrorWithContext(&LogContext{JobID: job.ID}, "JSON marshal failed: %v", err)
		// Try a more aggressive sanitization
		jobCopy = *job
		jobCopy.Result = sanitizeAnyValue(jobCopy.Result)
		data, err = json.Marshal(jobCopy)
		if err != nil {
			AppLogger.ErrorWithContext(&LogContext{JobID: job.ID}, "JSON marshal failed even after aggressive sanitization: %v", err)
			// Last resort: create a minimal job object without the problematic result
			minimalJob := SimpleJob{
				ID:          job.ID,
				Status:      job.Status,
				Progress:    job.Progress,
				Message:     job.Message,
				Error:       "Results contain invalid values and could not be serialized",
				CreatedAt:   job.CreatedAt,
				CompletedAt: job.CompletedAt,
			}
			data, err = json.Marshal(minimalJob)
			if err != nil {
				AppLogger.ErrorWithContext(&LogContext{JobID: job.ID}, "Even minimal job marshal failed: %v", err)
			}
		}
	}
	
	return data, err
}

// sanitizeBenchmarkResult sanitizes NaN and Inf values in benchmark results
func sanitizeBenchmarkResult(result map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	
	for key, value := range result {
		switch v := value.(type) {
		case map[string]interface{}:
			// Recursively sanitize nested maps
			sanitized[key] = sanitizeBenchmarkResult(v)
		case []interface{}:
			// Sanitize arrays
			sanitizedArray := make([]interface{}, len(v))
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					sanitizedArray[i] = sanitizeBenchmarkResult(itemMap)
				} else {
					sanitizedArray[i] = sanitizeFloatValue(item)
				}
			}
			sanitized[key] = sanitizedArray
		default:
			sanitized[key] = sanitizeFloatValue(value)
		}
	}
	
	return sanitized
}

// sanitizeFloatValue converts NaN and Inf values to null
func sanitizeFloatValue(value interface{}) interface{} {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil
		}
		return v
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil
		}
		return v
	default:
		return value
	}
}

// sanitizeAnyValue aggressively sanitizes any value for JSON marshaling
func sanitizeAnyValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	
	switch v := value.(type) {
	case map[string]interface{}:
		sanitized := make(map[string]interface{})
		for key, val := range v {
			sanitized[key] = sanitizeAnyValue(val)
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(v))
		for i, item := range v {
			sanitized[i] = sanitizeAnyValue(item)
		}
		return sanitized
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			AppLogger.DebugWithFields("Sanitized float64 +Inf/NaN to null", map[string]interface{}{
				"value": v,
				"isNaN": math.IsNaN(v),
				"isInf": math.IsInf(v, 0),
			})
			return nil
		}
		return v
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			AppLogger.DebugWithFields("Sanitized float32 +Inf/NaN to null", map[string]interface{}{
				"value": v,
				"isNaN": math.IsNaN(float64(v)),
				"isInf": math.IsInf(float64(v), 0),
			})
			return nil
		}
		return v
	case int, int8, int16, int32, int64:
		// Convert to float64 to check for infinity (though unlikely)
		f := float64(reflect.ValueOf(v).Int())
		if math.IsInf(f, 0) {
			AppLogger.DebugWithFields("Sanitized int +Inf to null", map[string]interface{}{
				"value": v,
				"float": f,
			})
			return nil
		}
		return v
	case uint, uint8, uint16, uint32, uint64:
		// Convert to float64 to check for infinity (though unlikely)
		f := float64(reflect.ValueOf(v).Uint())
		if math.IsInf(f, 0) {
			AppLogger.DebugWithFields("Sanitized uint +Inf to null", map[string]interface{}{
				"value": v,
				"float": f,
			})
			return nil
		}
		return v
	default:
		// For any other type, try to convert to string and check for "Inf" or "NaN"
		str := fmt.Sprintf("%v", v)
		if str == "+Inf" || str == "-Inf" || str == "Inf" || str == "NaN" {
			AppLogger.DebugWithFields("Sanitized string +Inf/NaN to null", map[string]interface{}{
				"value": v,
				"string": str,
				"type": fmt.Sprintf("%T", v),
			})
			return nil
		}
		return v
	}
}

// ToSSEMessage formats job as SSE message
func (job *SimpleJob) ToSSEMessage() string {
	data, err := job.ToJSON()
	if err != nil {
		AppLogger.ErrorWithContext(&LogContext{JobID: job.ID}, "Failed to marshal job to JSON: %v", err)
		// Return a minimal valid SSE message with error info
		errorMsg := fmt.Sprintf(`{"error":"JSON marshal failed","jobId":"%s","status":"%s"}`, job.ID, job.Status)
		return fmt.Sprintf("data: %s\n\n", errorMsg)
	}
	if len(data) == 0 {
		AppLogger.ErrorWithContext(&LogContext{JobID: job.ID}, "Empty JSON data for job")
		// Return a minimal valid SSE message
		minimalMsg := fmt.Sprintf(`{"jobId":"%s","status":"%s","progress":%d}`, job.ID, job.Status, job.Progress)
		return fmt.Sprintf("data: %s\n\n", minimalMsg)
	}
	return fmt.Sprintf("data: %s\n\n", string(data))
}

// RegisterSSEListener registers a channel to receive job updates
func (jm *SimpleJobManager) RegisterSSEListener(jobID string, updateChan chan *SimpleJob) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()
	
	if jm.listeners[jobID] == nil {
		jm.listeners[jobID] = make([]chan *SimpleJob, 0)
	}
	jm.listeners[jobID] = append(jm.listeners[jobID], updateChan)
}

// UnregisterSSEListener removes a channel from job updates
func (jm *SimpleJobManager) UnregisterSSEListener(jobID string, updateChan chan *SimpleJob) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()
	
	if listeners, exists := jm.listeners[jobID]; exists {
		for i, ch := range listeners {
			if ch == updateChan {
				jm.listeners[jobID] = append(listeners[:i], listeners[i+1:]...)
				close(updateChan)
				break
			}
		}
		if len(jm.listeners[jobID]) == 0 {
			delete(jm.listeners, jobID)
		}
	}
}

// broadcastUpdate sends job updates to all registered listeners
func (jm *SimpleJobManager) broadcastUpdate(jobID string, job *SimpleJob) {
	if listeners, exists := jm.listeners[jobID]; exists {
		AppLogger.DebugWithFields("Broadcasting update to SSE listeners", map[string]interface{}{
			"jobId": jobID,
			"listeners": len(listeners),
			"status": job.Status,
			"progress": job.Progress,
			"message": job.Message,
		})
		for _, ch := range listeners {
			select {
			case ch <- job:
				// Successfully sent update
			default:
				// Channel is full, skip this update
				AppLogger.WarnWithContext(&LogContext{JobID: jobID}, "Channel full, skipping update")
			}
		}
	} else {
		AppLogger.WarnWithContext(&LogContext{JobID: jobID}, "No listeners registered for job")
	}
}

// GetActiveJobCount returns the number of currently running jobs
func (jm *SimpleJobManager) GetActiveJobCount() int {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()
	return jm.activeJobCount
}

// IsSystemBusy returns true if there are any active jobs
func (jm *SimpleJobManager) IsSystemBusy() bool {
	return jm.GetActiveJobCount() > 0
}

// GetSystemStatus returns the global system status
func (jm *SimpleJobManager) GetSystemStatus() map[string]interface{} {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()
	
	return map[string]interface{}{
		"activeJobs":    jm.activeJobCount,
		"isBusy":        jm.activeJobCount > 0,
		"totalJobs":     len(jm.jobs),
		"timestamp":     time.Now(),
	}
}

// RegisterSystemStatusListener registers a listener for system status changes
func (jm *SimpleJobManager) RegisterSystemStatusListener() chan map[string]interface{} {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	listener := make(chan map[string]interface{}, 10) // Buffered channel
	jm.systemStatusListeners = append(jm.systemStatusListeners, listener)
	
	// Send initial status
	go func() {
		listener <- jm.GetSystemStatus()
	}()
	
	return listener
}

// UnregisterSystemStatusListener removes a system status listener
func (jm *SimpleJobManager) UnregisterSystemStatusListener(listener chan map[string]interface{}) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	for i, l := range jm.systemStatusListeners {
		if l == listener {
			jm.systemStatusListeners = append(jm.systemStatusListeners[:i], jm.systemStatusListeners[i+1:]...)
			close(listener)
			break
		}
	}
}

// RunBenchmark runs the benchmark execution for a job
func (jm *SimpleJobManager) RunBenchmark(jobID string, request BenchmarkRequest) {
	// Create a cancellable context for this benchmark job
	ctx, cancelFunc := context.WithCancel(context.Background())
	
	// Set the context in the job for cancellation
	jm.SetJobContext(jobID, ctx, cancelFunc)
	
	AppLogger.InfoWithFields("Starting benchmark", map[string]interface{}{
		"jobId": jobID,
		"model1": request.Model1.Name,
		"concurrency": request.ConcurrencyLevels,
		"maxTokens": request.MaxTokens,
	})

	// Give SSE connection time to establish
	AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Waiting for SSE connection to establish...")
	time.Sleep(2 * time.Second)

	// Update progress: Starting
	AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Updating progress: 10%% - Initializing benchmark...")
	jm.UpdateJobProgress(jobID, 10, "Initializing benchmark...")

	// Test latency first (skip for Cloud Foundry deployments)
	AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Updating progress: 20%% - Testing latency...")
	jm.UpdateJobProgress(jobID, 20, "Testing latency...")
	
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
			jm.FailJob(jobID, fmt.Sprintf("Latency test failed: %v", err))
			return
		}
		AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Latency test completed: %v", latency)
	}

	// Prepare results
	var model1Results []ConcurrencyResult
	var model2Results []ConcurrencyResult
	totalSteps := len(request.ConcurrencyLevels)
	if request.Model2 != nil {
		totalSteps *= 2
	}

	// Run benchmarks for each concurrency level
	for i, concurrency := range request.ConcurrencyLevels {
		// Check for cancellation before each concurrency level
		select {
		case <-ctx.Done():
			AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Job cancelled during Model 1 concurrency %d", concurrency)
			jm.FailJob(jobID, "Job cancelled by user")
			return
		default:
		}
		
		progress := 30 + (i * 60 / totalSteps)
		AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Updating progress: %d%% - Testing Model 1 concurrency %d...", progress, concurrency)
		jm.UpdateJobProgress(jobID, progress, fmt.Sprintf("Testing Model 1 concurrency %d...", concurrency))

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
		if apiKey == "" {
			AppLogger.ErrorWithContext(&LogContext{JobID: jobID, Model: request.Model1.Name}, "No API key found for model")
			jm.FailJob(jobID, fmt.Sprintf("No API key found for model %s", request.Model1.Name))
			return
		}

		// Create speed measurement setup
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

		// Run the benchmark
		AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Running benchmark for concurrency %d...", concurrency)
		result, err := setup.Run(ctx, bar)
		if err != nil {
			AppLogger.ErrorWithContext(&LogContext{JobID: jobID}, "Benchmark failed for concurrency %d: %v", concurrency, err)
			jm.FailJob(jobID, fmt.Sprintf("Benchmark failed for concurrency %d: %v", concurrency, err))
			bar.Close()
			return
		}

		// Store result for Model 1
		concurrencyResult := ConcurrencyResult{
			Concurrency:          concurrency,
			GenerationThroughput: result.GenerationSpeed,
			PromptThroughput:     result.PromptThroughput,
			MinTTFT:              result.MinTtft,
			MaxTTFT:              result.MaxTtft,
		}
		model1Results = append(model1Results, concurrencyResult)

			AppLogger.InfoWithFields("Model 1 concurrency completed", map[string]interface{}{
				"jobId": jobID,
				"concurrency": concurrency,
				"generationSpeed": result.GenerationSpeed,
				"promptThroughput": result.PromptThroughput,
				"minTtft": result.MinTtft,
				"maxTtft": result.MaxTtft,
			})
	}

	// Handle Model 2 if provided
	if request.Model2 != nil {
		for i, concurrency := range request.ConcurrencyLevels {
			// Check for cancellation before each Model 2 concurrency level
			select {
			case <-ctx.Done():
				AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Job cancelled during Model 2 concurrency %d", concurrency)
				jm.FailJob(jobID, "Job cancelled by user")
				return
			default:
			}
			
			progress := 30 + ((len(request.ConcurrencyLevels) + i) * 60 / totalSteps)
			AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Updating progress: %d%% - Testing Model 2 concurrency %d...", progress, concurrency)
			jm.UpdateJobProgress(jobID, progress, fmt.Sprintf("Testing Model 2 concurrency %d...", concurrency))

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

			// Get API key for Model 2
			apiKey := getAPIKeyForModel(*request.Model2)
			if apiKey == "" {
				AppLogger.ErrorWithContext(&LogContext{JobID: jobID, Model: request.Model2.Name}, "No API key found for model")
				jm.FailJob(jobID, fmt.Sprintf("No API key found for model %s", request.Model2.Name))
				return
			}

			// Create speed measurement setup for Model 2
			setup := utils.SpeedMeasurement{
				BaseUrl:        request.Model2.BaseURL,
				ApiKey:         apiKey,
				ModelName:      request.Model2.Name,
				Prompt:         request.Prompt,
				UseRandomInput: false, // We're using custom prompt
				NumWords:       request.NumWords,
				MaxTokens:      request.MaxTokens,
				Latency:        latency,
				Concurrency:    concurrency,
			}

			// Run speed measurement for Model 2
			result, err := setup.Run(ctx, bar)
			if err != nil {
				AppLogger.ErrorWithContext(&LogContext{JobID: jobID}, "Benchmark failed for Model 2 concurrency %d: %v", concurrency, err)
				jm.FailJob(jobID, fmt.Sprintf("Benchmark failed for Model 2 concurrency %d: %v", concurrency, err))
				bar.Close()
				return
			}

			// Store result for Model 2
			concurrencyResult := ConcurrencyResult{
				Concurrency:          concurrency,
				GenerationThroughput: result.GenerationSpeed,
				PromptThroughput:     result.PromptThroughput,
				MinTTFT:              result.MinTtft,
				MaxTTFT:              result.MaxTtft,
			}
			model2Results = append(model2Results, concurrencyResult)

			AppLogger.InfoWithFields("Model 2 concurrency completed", map[string]interface{}{
				"jobId": jobID,
				"concurrency": concurrency,
				"generationSpeed": result.GenerationSpeed,
				"promptThroughput": result.PromptThroughput,
				"minTtft": result.MinTtft,
				"maxTtft": result.MaxTtft,
			})
		}
	}

	// Complete the job
	AppLogger.DebugWithContext(&LogContext{JobID: jobID}, "Updating progress: 100%% - Benchmark completed")
	jm.UpdateJobProgress(jobID, 100, "Benchmark completed")
	
	// Create final result with proper structure
	finalResult := map[string]interface{}{
		"model1": map[string]interface{}{
			"model":   request.Model1.Name,
			"results": model1Results,
		},
		"model2": nil,
		"latency": latency,
		"summary": map[string]interface{}{
			"total_concurrency_levels": len(request.ConcurrencyLevels),
			"total_results": len(model1Results) + len(model2Results),
		},
	}
	
	// Add Model 2 results if available
	if request.Model2 != nil {
		finalResult["model2"] = map[string]interface{}{
			"model":   request.Model2.Name,
			"results": model2Results,
		}
	}

	jm.CompleteJob(jobID, finalResult)
	AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Benchmark job completed successfully")
}

// broadcastSystemStatus sends system status to all listeners
func (jm *SimpleJobManager) broadcastSystemStatus() {
	jm.mutex.RLock()
	status := jm.GetSystemStatus()
	listeners := make([]chan map[string]interface{}, len(jm.systemStatusListeners))
	copy(listeners, jm.systemStatusListeners)
	jm.mutex.RUnlock()

	for _, listener := range listeners {
		select {
		case listener <- status:
		default:
			// Skip if channel is full
		}
	}
}
