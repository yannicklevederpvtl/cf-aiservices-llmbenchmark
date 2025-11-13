package server

import (
	"encoding/json"
	"time"
)

// WebSocket message types
const (
	MessageTypeProgress    = "progress"
	MessageTypeStatus      = "status"
	MessageTypeError       = "error"
	MessageTypeComplete    = "complete"
	MessageTypeCancelled   = "cancelled"
	MessageTypePing        = "ping"
	MessageTypePong        = "pong"
)

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type      string      `json:"type"`
	JobID     string      `json:"jobId,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// ProgressUpdate represents benchmark progress information
type ProgressUpdate struct {
	JobID                   string  `json:"jobId"`
	Status                  string  `json:"status"` // "running", "completed", "failed", "cancelled"
	CurrentModel            string  `json:"currentModel,omitempty"`
	CurrentConcurrency      int     `json:"currentConcurrency,omitempty"`
	Progress                float64 `json:"progress"`                // 0-100
	ElapsedTime             float64 `json:"elapsedTime"`             // seconds
	EstimatedTimeRemaining  float64 `json:"estimatedTimeRemaining"`  // seconds
	CurrentStep             string  `json:"currentStep,omitempty"`
	TotalSteps              int     `json:"totalSteps,omitempty"`
	CurrentStepNumber       int     `json:"currentStepNumber,omitempty"`
}

// StatusUpdate represents job status information
type StatusUpdate struct {
	JobID     string `json:"jobId"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ErrorMessage represents error information
type ErrorMessage struct {
	JobID    string `json:"jobId"`
	Error    string `json:"error"`
	Message  string `json:"message"`
	Code     int    `json:"code,omitempty"`
	Details  string `json:"details,omitempty"`
}

// CompletionMessage represents benchmark completion information
type CompletionMessage struct {
	JobID     string      `json:"jobId"`
	Status    string      `json:"status"`
	Results   interface{} `json:"results,omitempty"`
	Duration  float64     `json:"duration"` // total duration in seconds
	Completed time.Time   `json:"completed"`
}

// CancellationMessage represents benchmark cancellation information
type CancellationMessage struct {
	JobID      string    `json:"jobId"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	Cancelled  time.Time `json:"cancelled"`
	Reason     string    `json:"reason,omitempty"`
}

// Helper functions for creating WebSocket messages

// NewProgressMessage creates a progress update message
func NewProgressMessage(jobID string, progress ProgressUpdate) *WebSocketMessage {
	return &WebSocketMessage{
		Type:      MessageTypeProgress,
		JobID:     jobID,
		Timestamp: time.Now(),
		Data:      progress,
	}
}

// NewStatusMessage creates a status update message
func NewStatusMessage(jobID string, status StatusUpdate) *WebSocketMessage {
	return &WebSocketMessage{
		Type:      MessageTypeStatus,
		JobID:     jobID,
		Timestamp: time.Now(),
		Data:      status,
	}
}

// NewErrorMessage creates an error message
func NewErrorMessage(jobID string, error ErrorMessage) *WebSocketMessage {
	return &WebSocketMessage{
		Type:      MessageTypeError,
		JobID:     jobID,
		Timestamp: time.Now(),
		Data:      error,
	}
}

// NewCompletionMessage creates a completion message
func NewCompletionMessage(jobID string, completion CompletionMessage) *WebSocketMessage {
	return &WebSocketMessage{
		Type:      MessageTypeComplete,
		JobID:     jobID,
		Timestamp: time.Now(),
		Data:      completion,
	}
}

// NewCancellationMessage creates a cancellation message
func NewCancellationMessage(jobID string, cancellation CancellationMessage) *WebSocketMessage {
	return &WebSocketMessage{
		Type:      MessageTypeCancelled,
		JobID:     jobID,
		Timestamp: time.Now(),
		Data:      cancellation,
	}
}

// ToJSON converts a WebSocket message to JSON bytes
func (m *WebSocketMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON creates a WebSocket message from JSON bytes
func FromJSON(data []byte) (*WebSocketMessage, error) {
	var msg WebSocketMessage
	err := json.Unmarshal(data, &msg)
	return &msg, err
}
