package server

import (
	"fmt"
	"sync"
	"time"
)

// ProgressTracker manages progress tracking for benchmark jobs
type ProgressTracker struct {
	JobID              string
	StartTime          time.Time
	TotalSteps         int
	CurrentStep        int
	CurrentModel       string
	CurrentConcurrency int
	Status             string // "running", "completed", "failed", "cancelled"
	Hub                *Hub
	mutex              sync.RWMutex
	lastBroadcast      time.Time
	throttleInterval   time.Duration
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(jobID string, totalSteps int, hub *Hub) *ProgressTracker {
	return &ProgressTracker{
		JobID:            jobID,
		StartTime:        time.Now(),
		TotalSteps:       totalSteps,
		CurrentStep:      0,
		Status:           "running",
		Hub:              hub,
		throttleInterval: 1 * time.Second, // Throttle to max 1 update per second
	}
}

// UpdateProgress updates the current progress and broadcasts if throttling allows
func (pt *ProgressTracker) UpdateProgress(step int, model string, concurrency int, currentStep string) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.CurrentStep = step
	pt.CurrentModel = model
	pt.CurrentConcurrency = concurrency

	// Check if we should broadcast (throttling)
	now := time.Now()
	if now.Sub(pt.lastBroadcast) >= pt.throttleInterval {
		pt.broadcastProgress(currentStep)
		pt.lastBroadcast = now
	}
}

// SetStatus updates the job status and broadcasts immediately
func (pt *ProgressTracker) SetStatus(status string, message string) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.Status = status

	// Always broadcast status changes immediately (no throttling)
	pt.broadcastStatus(message)
}

// GetProgress returns the current progress information
func (pt *ProgressTracker) GetProgress() ProgressUpdate {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	elapsed := time.Since(pt.StartTime).Seconds()
	progress := float64(pt.CurrentStep) / float64(pt.TotalSteps) * 100

	// Estimate remaining time based on current progress
	var estimatedRemaining float64
	if progress > 0 {
		estimatedRemaining = (elapsed / progress) * (100 - progress)
	}

	return ProgressUpdate{
		JobID:                  pt.JobID,
		Status:                 pt.Status,
		CurrentModel:           pt.CurrentModel,
		CurrentConcurrency:     pt.CurrentConcurrency,
		Progress:               progress,
		ElapsedTime:            elapsed,
		EstimatedTimeRemaining: estimatedRemaining,
		CurrentStep:            pt.getCurrentStepDescription(),
		TotalSteps:             pt.TotalSteps,
		CurrentStepNumber:      pt.CurrentStep,
	}
}

// broadcastProgress sends a progress update to all connected clients
func (pt *ProgressTracker) broadcastProgress(currentStep string) {
	progress := pt.GetProgress()
	progress.CurrentStep = currentStep

	message := NewProgressMessage(pt.JobID, progress)
	if data, err := message.ToJSON(); err == nil {
		pt.Hub.BroadcastMessage(data)
	}
}

// broadcastStatus sends a status update to all connected clients
func (pt *ProgressTracker) broadcastStatus(message string) {
	status := StatusUpdate{
		JobID:     pt.JobID,
		Status:    pt.Status,
		Message:   message,
		CreatedAt: pt.StartTime,
		UpdatedAt: time.Now(),
	}

	wsMessage := NewStatusMessage(pt.JobID, status)
	if data, err := wsMessage.ToJSON(); err == nil {
		pt.Hub.BroadcastMessage(data)
	}
}

// getCurrentStepDescription returns a human-readable description of the current step
func (pt *ProgressTracker) getCurrentStepDescription() string {
	if pt.CurrentModel == "" {
		return "Initializing benchmark..."
	}

	if pt.CurrentConcurrency > 0 {
		return fmt.Sprintf("Testing %s with concurrency %d", pt.CurrentModel, pt.CurrentConcurrency)
	}

	return fmt.Sprintf("Testing %s", pt.CurrentModel)
}

// Complete marks the benchmark as completed and broadcasts final results
func (pt *ProgressTracker) Complete(results interface{}) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.Status = "completed"
	pt.CurrentStep = pt.TotalSteps

	duration := time.Since(pt.StartTime).Seconds()

	completion := CompletionMessage{
		JobID:     pt.JobID,
		Status:    "completed",
		Results:   results,
		Duration:  duration,
		Completed: time.Now(),
	}

	message := NewCompletionMessage(pt.JobID, completion)
	if data, err := message.ToJSON(); err == nil {
		pt.Hub.BroadcastMessage(data)
	}
}

// Fail marks the benchmark as failed and broadcasts error information
func (pt *ProgressTracker) Fail(errorMsg string, details string) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.Status = "failed"

	errorMessage := ErrorMessage{
		JobID:   pt.JobID,
		Error:   "Benchmark failed",
		Message: errorMsg,
		Details: details,
	}

	message := NewErrorMessage(pt.JobID, errorMessage)
	if data, err := message.ToJSON(); err == nil {
		pt.Hub.BroadcastMessage(data)
	}
}

// Cancel marks the benchmark as cancelled and broadcasts cancellation information
func (pt *ProgressTracker) Cancel(reason string) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.Status = "cancelled"

	cancellation := CancellationMessage{
		JobID:     pt.JobID,
		Status:    "cancelled",
		Message:   "Benchmark cancelled",
		Cancelled: time.Now(),
		Reason:    reason,
	}

	message := NewCancellationMessage(pt.JobID, cancellation)
	if data, err := message.ToJSON(); err == nil {
		pt.Hub.BroadcastMessage(data)
	}
}
