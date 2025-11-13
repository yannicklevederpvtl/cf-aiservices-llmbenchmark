package main

import "llmapibenchmark/internal/utils"

type Benchmark struct {
	BaseURL           string
	ApiKey            string
	ModelName         string
	Prompt            string
	InputTokens       int
	MaxTokens         int
	ConcurrencyLevels []int
	UseRandomInput    bool
	NumWords          int
}

type BenchmarkResult struct {
	ModelName   string              `json:"model_name" yaml:"model-name"`
	InputTokens int                 `json:"input_tokens" yaml:"input-tokens"`
	MaxTokens   int                 `json:"output_tokens" yaml:"output-tokens"` // Historically been called Output Tokens
	Latency     float64             `json:"latency" yaml:"latency"`
	Results     []utils.SpeedResult `json:"results" yaml:"results"`
}
