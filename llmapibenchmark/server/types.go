package server

import (
	"time"
)

// Model represents a configured LLM model
type Model struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	APIKey   string `json:"apiKey,omitempty"` // Omit from JSON for security
}

// BenchmarkRequest represents the request payload for running benchmarks
type BenchmarkRequest struct {
	Model1            Model  `json:"model1" binding:"required"`
	Model2            *Model `json:"model2"` // Optional - can benchmark single model
	ConcurrencyLevels []int  `json:"concurrencyLevels" binding:"required,min=1"`
	MaxTokens         int    `json:"maxTokens" binding:"required,min=1,max=4096"`
	Prompt            string `json:"prompt" binding:"required,min=1"`
	NumWords          int    `json:"numWords,omitempty"` // For random prompt generation
}

// ConcurrencyResult represents the result for a single concurrency level
type ConcurrencyResult struct {
	Concurrency          int     `json:"concurrency"`
	GenerationThroughput float64 `json:"generationThroughput"`
	PromptThroughput     float64 `json:"promptThroughput"`
	MinTTFT              float64 `json:"minTtft"`
	MaxTTFT              float64 `json:"maxTtft"`
}

// BenchmarkResult represents the result of a single model benchmark
type BenchmarkResult struct {
	Model                string              `json:"model"`
	Results              []ConcurrencyResult `json:"results"`
	Timestamp            time.Time           `json:"timestamp"`
}

// Comparison represents the comparison between two models
type Comparison struct {
	Winner      string             `json:"winner"` // "model1", "model2", or "tie"
	Differences map[string]float64 `json:"differences"`
}

// ComparisonResponse represents the full benchmark comparison response
type ComparisonResponse struct {
	Model1     *BenchmarkResult `json:"model1"`
	Model2     *BenchmarkResult `json:"model2,omitempty"`
	Comparison *Comparison      `json:"comparison,omitempty"`
}

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// ModelsResponse represents the response for model discovery
type ModelsResponse struct {
	Models []Model `json:"models"`
	Count  int     `json:"count"`
}

