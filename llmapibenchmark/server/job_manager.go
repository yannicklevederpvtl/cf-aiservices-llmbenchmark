package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"llmapibenchmark/internal/utils"
	"github.com/schollz/progressbar/v3"
)

// JobManager manages asynchronous benchmark jobs
type JobManager struct {
	jobs   map[string]*BenchmarkJob
	mutex  sync.RWMutex
	hub    *Hub
}

// BenchmarkJob represents an asynchronous benchmark job
type BenchmarkJob struct {
	ID          string
	Status      string // "queued", "running", "completed", "failed", "cancelled"
	Request     BenchmarkRequest
	Progress    *ProgressTracker
	Results     interface{}
	Error       error
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	cancelChan  chan bool
}

// NewJobManager creates a new job manager
func NewJobManager(hub *Hub) *JobManager {
	return &JobManager{
		jobs: make(map[string]*BenchmarkJob),
		hub:  hub,
	}
}

// CreateJob creates a new benchmark job and returns its ID
func (jm *JobManager) CreateJob(request BenchmarkRequest) string {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	jobID := uuid.New().String()

	// Calculate total steps for progress tracking
	totalSteps := len(request.ConcurrencyLevels)
	if request.Model2 != nil {
		totalSteps *= 2 // Two models
	}

	job := &BenchmarkJob{
		ID:        jobID,
		Status:    "queued",
		Request:   request,
		Progress:  NewProgressTracker(jobID, totalSteps, jm.hub),
		CreatedAt: time.Now(),
		cancelChan: make(chan bool, 1),
	}

	jm.jobs[jobID] = job

	log.Printf("Created benchmark job: %s", jobID)
	return jobID
}

// GetJob retrieves a job by ID
func (jm *JobManager) GetJob(jobID string) (*BenchmarkJob, bool) {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	job, exists := jm.jobs[jobID]
	return job, exists
}

// StartJob starts executing a benchmark job asynchronously
func (jm *JobManager) StartJob(jobID string) error {
	jm.mutex.Lock()
	job, exists := jm.jobs[jobID]
	if !exists {
		jm.mutex.Unlock()
		return fmt.Errorf("job %s not found", jobID)
	}

	if job.Status != "queued" {
		jm.mutex.Unlock()
		return fmt.Errorf("job %s is not in queued status (current: %s)", jobID, job.Status)
	}

	job.Status = "running"
	now := time.Now()
	job.StartedAt = &now
	jm.mutex.Unlock()

	// Start the benchmark execution in a goroutine
	log.Printf("DEBUG: Starting benchmark job goroutine: %s", jobID)
	go jm.executeJob(job)

	log.Printf("Started benchmark job: %s", jobID)
	return nil
}

// CancelJob cancels a running job
func (jm *JobManager) CancelJob(jobID string) error {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	job, exists := jm.jobs[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	if job.Status != "running" {
		return fmt.Errorf("job %s is not running (current: %s)", jobID, job.Status)
	}

	// Send cancellation signal
	select {
	case job.cancelChan <- true:
	default:
	}

	job.Status = "cancelled"
	now := time.Now()
	job.CompletedAt = &now

	// Broadcast cancellation
	job.Progress.Cancel("User requested cancellation")

	log.Printf("Cancelled benchmark job: %s", jobID)
	return nil
}

// executeJob runs the benchmark execution logic
func (jm *JobManager) executeJob(job *BenchmarkJob) {
	log.Printf("DEBUG: executeJob started for job: %s", job.ID)
	
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Job %s panicked: %v", job.ID, r)
			job.Progress.Fail("Internal error", fmt.Sprintf("Panic: %v", r))
		}
	}()

	// Broadcast job start
	log.Printf("DEBUG: Setting job status to running for job: %s", job.ID)
	job.Progress.SetStatus("running", "Benchmark started")

	// Execute the benchmark with progress tracking
	log.Printf("DEBUG: Starting runBenchmarkWithProgress for job: %s", job.ID)
	results, err := jm.runBenchmarkWithProgress(job)

	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	now := time.Now()
	job.CompletedAt = &now

	if err != nil {
		job.Status = "failed"
		job.Error = err
		job.Progress.Fail("Benchmark execution failed", err.Error())
		log.Printf("Job %s failed: %v", job.ID, err)
	} else {
		job.Status = "completed"
		job.Results = results
		job.Progress.Complete(results)
		log.Printf("Job %s completed successfully", job.ID)
	}
}

// runBenchmarkWithProgress executes the benchmark while tracking progress
func (jm *JobManager) runBenchmarkWithProgress(job *BenchmarkJob) (interface{}, error) {
	log.Printf("DEBUG: runBenchmarkWithProgress started for job: %s", job.ID)
	
	request := job.Request
	progress := job.Progress

	log.Printf("DEBUG: Request details - Model1: %s, ConcurrencyLevels: %v", request.Model1.Name, request.ConcurrencyLevels)

	// Check for cancellation before starting
	select {
	case <-job.cancelChan:
		log.Printf("DEBUG: Job cancelled before execution: %s", job.ID)
		return nil, fmt.Errorf("job cancelled before execution")
	default:
	}

	// Calculate total steps
	totalSteps := len(request.ConcurrencyLevels)
	if request.Model2 != nil {
		totalSteps *= 2
	}

	log.Printf("DEBUG: Total steps calculated: %d", totalSteps)

	step := 0

	// Run benchmark for Model 1
	progress.UpdateProgress(step, request.Model1.Name, 0, "Starting Model 1 benchmark")
	
	model1Results, err := jm.runSingleModelBenchmark(request.Model1, request.ConcurrencyLevels, request.MaxTokens, request.Prompt, request.NumWords, job.cancelChan, progress, &step, totalSteps)
	if err != nil {
		return nil, fmt.Errorf("model 1 benchmark failed: %w", err)
	}

	// Check for cancellation after Model 1
	select {
	case <-job.cancelChan:
		return nil, fmt.Errorf("job cancelled after model 1")
	default:
	}

	var model2Results interface{}
	var comparison interface{}

	// Run benchmark for Model 2 if provided
	if request.Model2 != nil {
		progress.UpdateProgress(step, request.Model2.Name, 0, "Starting Model 2 benchmark")
		
		model2Results, err = jm.runSingleModelBenchmark(*request.Model2, request.ConcurrencyLevels, request.MaxTokens, request.Prompt, request.NumWords, job.cancelChan, progress, &step, totalSteps)
		if err != nil {
			return nil, fmt.Errorf("model 2 benchmark failed: %w", err)
		}

		// Check for cancellation after Model 2
		select {
		case <-job.cancelChan:
			return nil, fmt.Errorf("job cancelled after model 2")
		default:
		}

		// Generate comparison
		progress.UpdateProgress(step, "", 0, "Generating comparison results")
		comparison = jm.generateComparison(model1Results, model2Results)
	}

	// Create final response
	response := ComparisonResponse{
		Model1: model1Results,
	}
	
	if model2Results != nil {
		response.Model2 = model2Results.(*BenchmarkResult)
		response.Comparison = comparison.(*Comparison)
	}

	return response, nil
}

// runSingleModelBenchmark runs benchmark for a single model with progress tracking
func (jm *JobManager) runSingleModelBenchmark(model Model, concurrencyLevels []int, maxTokens int, prompt string, numWords int, cancelChan chan bool, progress *ProgressTracker, step *int, totalSteps int) (*BenchmarkResult, error) {
	var results []ConcurrencyResult

	for _, concurrency := range concurrencyLevels {
		// Check for cancellation
		select {
		case <-cancelChan:
			return nil, fmt.Errorf("benchmark cancelled")
		default:
		}

		// Update progress
		progress.UpdateProgress(*step, model.Name, concurrency, fmt.Sprintf("Testing %s with concurrency %d", model.Name, concurrency))

		// Run the actual benchmark (this would call the existing benchmark logic)
		result, err := jm.runConcurrencyBenchmark(model, concurrency, maxTokens, prompt, numWords, progress, *step, totalSteps)
		if err != nil {
			return nil, fmt.Errorf("concurrency %d failed: %w", concurrency, err)
		}

		results = append(results, result)
		*step++

		// Small delay to allow progress updates to be processed
		time.Sleep(100 * time.Millisecond)
	}

	// Calculate averages
	avgGenerationThroughput := 0.0
	avgPromptThroughput := 0.0
	avgMinTtft := 0.0
	avgMaxTtft := 0.0

	for _, result := range results {
		avgGenerationThroughput += result.GenerationThroughput
		avgPromptThroughput += result.PromptThroughput
		avgMinTtft += result.MinTTFT
		avgMaxTtft += result.MaxTTFT
	}

	count := float64(len(results))
	avgGenerationThroughput /= count
	avgPromptThroughput /= count
	avgMinTtft /= count
	avgMaxTtft /= count

	return &BenchmarkResult{
		Model:     model.Name,
		Results:   results,
		Timestamp: time.Now(),
	}, nil
}

// runConcurrencyBenchmark runs a single concurrency benchmark
func (jm *JobManager) runConcurrencyBenchmark(model Model, concurrency int, maxTokens int, prompt string, numWords int, progress *ProgressTracker, step int, totalSteps int) (ConcurrencyResult, error) {
	// Update progress
	progress.UpdateProgress(step, model.Name, 0, fmt.Sprintf("Running benchmark with concurrency %d", concurrency))
	
	log.Printf("DEBUG: Starting benchmark for model %s with concurrency %d", model.Name, concurrency)
	
	// Use the real benchmark logic from the existing codebase
	// Create speed measurement setup
	// Use API key from environment variables for security
	apiKey := getAPIKeyForModel(model)
	setup := utils.SpeedMeasurement{
		BaseUrl:        model.BaseURL,
		ApiKey:         apiKey,
		ModelName:      model.Name,
		Prompt:         prompt,
		UseRandomInput: false, // We're using custom prompt
		NumWords:       numWords,
		Latency:        0, // No additional latency
		Concurrency:    concurrency,
	}
	
	log.Printf("DEBUG: Created speed measurement setup for %s", model.Name)
	log.Printf("DEBUG: About to call setup.Run() for %s", model.Name)
	
	// Create a dummy progress bar for async execution (required by the Run method)
	expectedTokens := concurrency * maxTokens
	dummyBar := progressbar.NewOptions(expectedTokens,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetDescription(fmt.Sprintf("Async Concurrency %d", concurrency)),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("tokens"),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetRenderBlankState(true),
	)
	
	// Run the actual benchmark
	result, err := setup.Run(context.Background(), dummyBar)
	if err != nil {
		log.Printf("ERROR: Benchmark failed for %s: %v", model.Name, err)
		dummyBar.Close()
		return ConcurrencyResult{}, err
	}
	
	// Clean up the progress bar
	dummyBar.Finish()
	dummyBar.Close()
	
	log.Printf("DEBUG: Benchmark completed for %s, result: %+v", model.Name, result)
	
	// Update progress
	progress.UpdateProgress(step, model.Name, 100, fmt.Sprintf("Completed benchmark with concurrency %d", concurrency))
	
	// Convert to our result format
	return ConcurrencyResult{
		Concurrency:           result.Concurrency,
		GenerationThroughput:  result.GenerationSpeed,
		PromptThroughput:      result.PromptThroughput,
		MinTTFT:              result.MinTtft,
		MaxTTFT:              result.MaxTtft,
	}, nil
}

// generateComparison creates comparison data between two models
func (jm *JobManager) generateComparison(model1Results, model2Results interface{}) interface{} {
	// This would contain the actual comparison logic
	// For now, return a simple comparison structure
	return map[string]interface{}{
		"model1_better_at": "generation_throughput",
		"model2_better_at": "prompt_throughput",
		"overall_winner":   "model1",
	}
}

// CleanupOldJobs removes completed jobs older than the specified duration
func (jm *JobManager) CleanupOldJobs(maxAge time.Duration) {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for jobID, job := range jm.jobs {
		if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
			delete(jm.jobs, jobID)
			log.Printf("Cleaned up old job: %s", jobID)
		}
	}
}
