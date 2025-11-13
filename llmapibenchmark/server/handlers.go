package server

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"llmapibenchmark/internal/utils"
	"github.com/gin-gonic/gin"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}


// HealthHandler returns server health status
func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "ok",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	})
}


// ModelsHandler returns available models from environment or VCAP_SERVICES
func ModelsHandler(c *gin.Context) {
	models := discoverModels()
	
	c.JSON(http.StatusOK, ModelsResponse{
		Models: models,
		Count:  len(models),
	})
}

// discoverModels discovers available models using hybrid approach: local first, then Cloud Foundry fallback
func discoverModels() []Model {
	models := []Model{}
	
	// STEP 1: Try local environment variables first (keep working local logic)
	// Check for MODEL1 configuration
	if model1Name := os.Getenv("MODEL1_NAME"); model1Name != "" {
		models = append(models, Model{
			ID:       model1Name, // Simple ID for local
			Name:     model1Name, // Simple name for local
			Provider: "Direct OpenAI Compatible",
			BaseURL:  os.Getenv("MODEL1_BASE_URL"),
			// APIKey is intentionally not included for security
		})
	}
	
	// Check for MODEL2 configuration
	if model2Name := os.Getenv("MODEL2_NAME"); model2Name != "" {
		models = append(models, Model{
			ID:       model2Name, // Simple ID for local
			Name:     model2Name, // Simple name for local
			Provider: "Direct OpenAI Compatible",
			BaseURL:  os.Getenv("MODEL2_BASE_URL"),
		})
	}
	
	// Fallback: Check for generic MODELS configuration
	if len(models) == 0 {
		if modelsStr := os.Getenv("MODELS"); modelsStr != "" {
			baseURL := os.Getenv("BASE_URL")
			if baseURL == "" {
				baseURL = "https://api.openai.com/v1"
			}
			
			modelNames := strings.Split(modelsStr, ",")
				for _, name := range modelNames {
					name = strings.TrimSpace(name)
					if name != "" {
						models = append(models, Model{
							ID:       name, // Simple ID for local
							Name:     name, // Simple name for local
							Provider: "Direct OpenAI Compatible",
							BaseURL:  baseURL,
						})
					}
				}
		}
	}
	
	// STEP 2: If no local models found, try Cloud Foundry VCAP_SERVICES fallback
	if len(models) == 0 {
		AppLogger.Info("No local models found, trying Cloud Foundry VCAP_SERVICES...")
		enhancedResponse, err := DiscoverEnhancedModels()
		if err == nil && len(enhancedResponse.Models) > 0 {
			AppLogger.InfoWithFields("Found models from Cloud Foundry", map[string]interface{}{
				"count": len(enhancedResponse.Models),
			})
			// Convert enhanced models to legacy format for backward compatibility
			for _, enhanced := range enhancedResponse.Models {
				// Use user-friendly display name, fallback to original name, then complex name
				displayName := enhanced.DisplayName
				if displayName == "" {
					displayName = enhanced.OriginalName
				}
				if displayName == "" {
					displayName = enhanced.Name
				}
				
				models = append(models, Model{
					ID:       enhanced.ID,       // Complex ID: "serviceId|modelName" (for internal use)
					Name:     displayName,       // User-friendly display name
					Provider: enhanced.Provider,
					BaseURL:  enhanced.BaseURL,
					// APIKey is intentionally not included for security
				})
			}
		} else {
			AppLogger.Warn("No Cloud Foundry models found either")
		}
	}
	
	// STEP 3: Final fallback to default OpenAI models
	if len(models) == 0 {
		AppLogger.Info("Using default OpenAI models as final fallback")
		models = []Model{
			{
				ID:       "gpt-4",
				Name:     "GPT-4",
				Provider: "OpenAI",
				BaseURL:  "https://api.openai.com/v1",
			},
			{
				ID:       "gpt-3.5-turbo",
				Name:     "GPT-3.5 Turbo",
				Provider: "OpenAI",
				BaseURL:  "https://api.openai.com/v1",
			},
		}
	}
	
	AppLogger.InfoWithFields("Discovered models total", map[string]interface{}{
		"count": len(models),
	})
	return models
}

// discoverModelsLegacy provides fallback to original implementation
func discoverModelsLegacy() []Model {
	models := []Model{}
	
	// Check for MODEL1 configuration
	if model1Name := os.Getenv("MODEL1_NAME"); model1Name != "" {
		models = append(models, Model{
			ID:       model1Name,
			Name:     model1Name,
			Provider: "Direct OpenAI Compatible",
			BaseURL:  os.Getenv("MODEL1_BASE_URL"),
			// APIKey is intentionally not included for security
		})
	}
	
	// Check for MODEL2 configuration
	if model2Name := os.Getenv("MODEL2_NAME"); model2Name != "" {
		models = append(models, Model{
			ID:       model2Name,
			Name:     model2Name,
			Provider: "Direct OpenAI Compatible",
			BaseURL:  os.Getenv("MODEL2_BASE_URL"),
		})
	}
	
	// Fallback: Check for generic MODELS configuration
	if len(models) == 0 {
		if modelsStr := os.Getenv("MODELS"); modelsStr != "" {
			baseURL := os.Getenv("BASE_URL")
			if baseURL == "" {
				baseURL = "https://api.openai.com/v1"
			}
			
			modelNames := strings.Split(modelsStr, ",")
			for _, name := range modelNames {
				name = strings.TrimSpace(name)
				if name != "" {
					models = append(models, Model{
						ID:       name,
						Name:     name,
						Provider: "Direct OpenAI Compatible",
						BaseURL:  baseURL,
					})
				}
			}
		}
	}
	
	// If no models found, return default OpenAI models
	if len(models) == 0 {
		models = []Model{
			{
				ID:       "gpt-4",
				Name:     "GPT-4",
				Provider: "OpenAI",
				BaseURL:  "https://api.openai.com/v1",
			},
			{
				ID:       "gpt-3.5-turbo",
				Name:     "GPT-3.5 Turbo",
				Provider: "OpenAI",
				BaseURL:  "https://api.openai.com/v1",
			},
		}
	}
	
	return models
}


// BenchmarkHandler executes benchmark tests on one or two models
func BenchmarkHandler(c *gin.Context) {
	var req BenchmarkRequest

	// Parse and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		AppLogger.Error("Failed to parse request JSON: %v", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Bad Request",
			Message: fmt.Sprintf("Invalid request payload: %v", err),
			Code:    http.StatusBadRequest,
		})
		return
	}

	AppLogger.DebugWithFields("Received benchmark request", map[string]interface{}{
		"model1": req.Model1.Name,
		"model2": req.Model2,
	})
	
	// Enhanced validation
	if validationErr := validateBenchmarkRequest(&req); validationErr != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation Error",
			Message: validationErr.Error(),
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Create job with cancellable context (Task 15.3 compliance)
	jobManager := GetJobManager()
	jobID := jobManager.CreateJob(req)
	
	// Create context and set it for cancellation support
	ctx, cancelFunc := context.WithCancel(context.Background())
	jobManager.SetJobContext(jobID, ctx, cancelFunc)
	
	AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Created job for synchronous benchmark")
	AppLogger.InfoWithFields("Starting benchmark for model1", map[string]interface{}{
		"jobId": jobID,
		"model1": req.Model1.Name,
		"concurrencyLevels": req.ConcurrencyLevels,
		"maxTokens": req.MaxTokens,
	})
	if req.Model2 != nil {
		AppLogger.InfoWithContext(&LogContext{JobID: jobID, Model: req.Model2.Name}, "Starting benchmark for model2")
	}
	
	// Run benchmark for model1 with context
	AppLogger.DebugWithFields("Starting benchmark for model1", map[string]interface{}{
		"jobId": jobID,
		"model1": req.Model1.Name,
		"model1Id": req.Model1.ID,
		"model1BaseURL": req.Model1.BaseURL,
	})
	result1, err := runSingleBenchmarkWithContext(ctx, req.Model1, req.ConcurrencyLevels, req.MaxTokens, req.Prompt)
	if err != nil {
		AppLogger.ErrorWithContext(&LogContext{JobID: jobID, Model: req.Model1.Name}, "Failed to benchmark model1: %v", err)
		jobManager.FailJob(jobID, fmt.Sprintf("Failed to benchmark %s: %v", req.Model1.Name, err))
		cancelFunc() // Clean up context
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Benchmark Error",
			Message: fmt.Sprintf("Failed to benchmark %s: %v", req.Model1.Name, err),
			Code:    http.StatusInternalServerError,
		})
		return
	}
	AppLogger.InfoWithContext(&LogContext{JobID: jobID, Model: req.Model1.Name}, "Successfully completed benchmark for model1")
	
	response := ComparisonResponse{
		Model1: result1,
	}

	// Run benchmark for model2 if provided
	if req.Model2 != nil {
		// Check for cancellation before starting model2
		select {
		case <-ctx.Done():
			AppLogger.InfoWithContext(&LogContext{JobID: jobID}, "Job cancelled before model2 benchmark")
			jobManager.FailJob(jobID, "Job cancelled by user")
			c.JSON(http.StatusRequestTimeout, ErrorResponse{
				Error:   "Request Cancelled",
				Message: "Benchmark was cancelled before completion",
				Code:    http.StatusRequestTimeout,
			})
			return
		default:
		}
		
		result2, err := runSingleBenchmarkWithContext(ctx, *req.Model2, req.ConcurrencyLevels, req.MaxTokens, req.Prompt)
		if err != nil {
			AppLogger.ErrorWithContext(&LogContext{JobID: jobID, Model: req.Model2.Name}, "Failed to benchmark model2: %v", err)
			jobManager.FailJob(jobID, fmt.Sprintf("Failed to benchmark %s: %v", req.Model2.Name, err))
			cancelFunc() // Clean up context
			// Return error instead of partial results for consistency
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Benchmark Error",
				Message: fmt.Sprintf("Failed to benchmark %s: %v", req.Model2.Name, err),
				Code:    http.StatusInternalServerError,
			})
			return
		}
		response.Model2 = result2
		response.Comparison = compareResults(result1, result2)
	}

	// Complete the job successfully
	jobManager.CompleteJob(jobID, response)
	cancelFunc() // Clean up context
	
	AppLogger.DebugWithFields("Sending response with Model1", map[string]interface{}{
		"model1": response.Model1,
	})
	if response.Model2 != nil {
		AppLogger.DebugWithFields("Sending response with Model2", map[string]interface{}{
			"model2": response.Model2,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{
		"jobId": jobID,
		"result": response,
		"message": "Benchmark completed successfully",
	})
}

// validateBenchmarkRequest performs enhanced validation on benchmark request
func validateBenchmarkRequest(req *BenchmarkRequest) error {
	// Validate Model1
	if req.Model1.ID == "" {
		return fmt.Errorf("model1.id is required")
	}
	if req.Model1.Name == "" {
		return fmt.Errorf("model1.name is required")
	}
	if req.Model1.BaseURL == "" {
		return fmt.Errorf("model1.baseUrl is required")
	}
	
	// Validate Model2 if provided
	if req.Model2 != nil {
		if req.Model2.ID == "" {
			return fmt.Errorf("model2.id is required when model2 is provided")
		}
		if req.Model2.Name == "" {
			return fmt.Errorf("model2.name is required when model2 is provided")
		}
		if req.Model2.BaseURL == "" {
			return fmt.Errorf("model2.baseUrl is required when model2 is provided")
		}
		
		// Ensure models are different
		if req.Model1.ID == req.Model2.ID {
			return fmt.Errorf("model1 and model2 must be different (both are %s)", req.Model1.ID)
		}
	}
	
	// Validate concurrency levels
	if len(req.ConcurrencyLevels) == 0 {
		return fmt.Errorf("concurrencyLevels cannot be empty")
	}
	for i, concurrency := range req.ConcurrencyLevels {
		if concurrency < 1 {
			return fmt.Errorf("concurrencyLevels[%d] must be at least 1, got %d", i, concurrency)
		}
		if concurrency > 100 {
			return fmt.Errorf("concurrencyLevels[%d] must not exceed 100, got %d", i, concurrency)
		}
	}
	
	// Validate maxTokens
	if req.MaxTokens < 1 {
		return fmt.Errorf("maxTokens must be at least 1, got %d", req.MaxTokens)
	}
	if req.MaxTokens > 4096 {
		return fmt.Errorf("maxTokens must not exceed 4096, got %d", req.MaxTokens)
	}
	
	// Validate prompt
	if len(strings.TrimSpace(req.Prompt)) == 0 {
		return fmt.Errorf("prompt cannot be empty")
	}
	if len(req.Prompt) > 10000 {
		return fmt.Errorf("prompt too long (max 10000 characters), got %d", len(req.Prompt))
	}
	
	// Validate numWords if using random prompt generation
	if req.NumWords > 0 {
		if req.NumWords < 10 {
			return fmt.Errorf("numWords must be at least 10 for random prompts, got %d", req.NumWords)
		}
		if req.NumWords > 10000 {
			return fmt.Errorf("numWords must not exceed 10000, got %d", req.NumWords)
		}
	}
	
	return nil
}

// runSingleBenchmark runs benchmark for a single model across multiple concurrency levels
func runSingleBenchmark(model Model, concurrencyLevels []int, maxTokens int, prompt string) (*BenchmarkResult, error) {
	AppLogger.DebugWithFields("runSingleBenchmark called", map[string]interface{}{
		"model": model.Name,
		"modelId": model.ID,
	})
	
	// Get API key from environment
	apiKey := getAPIKeyForModel(model)
	if apiKey == "" {
		AppLogger.ErrorWithContext(&LogContext{Model: model.Name}, "No API key found for model")
		return nil, fmt.Errorf("no API key found for model %s", model.Name)
	}
	AppLogger.DebugWithFields("Using API key for model", map[string]interface{}{
		"model": model.Name,
		"keyLength": len(apiKey),
	})
	
	var results []ConcurrencyResult
	
	// Run benchmark for each concurrency level
	for _, concurrency := range concurrencyLevels {
		AppLogger.DebugWithFields("Running benchmark for concurrency level", map[string]interface{}{
			"concurrency": concurrency,
		})
		
		// Create speed measurement
		speedMeasurement := utils.SpeedMeasurement{
			BaseUrl:        model.BaseURL,
			ApiKey:         apiKey,
			ModelName:      model.ID,
			Prompt:         prompt,
			UseRandomInput: false,
			MaxTokens:      maxTokens,
			Latency:        0, // TODO: Measure actual latency
			Concurrency:    concurrency,
		}

		AppLogger.DebugWithFields("SpeedMeasurement config", map[string]interface{}{
			"baseURL": speedMeasurement.BaseUrl,
			"modelName": speedMeasurement.ModelName,
			"maxTokens": speedMeasurement.MaxTokens,
		})

		// Run benchmark (without progress bar for API)
		result, err := speedMeasurement.Run(context.Background(), nil)
		if err != nil {
			AppLogger.ErrorWithFields("Benchmark failed for concurrency", map[string]interface{}{
				"concurrency": concurrency,
				"error": err,
			})
			return nil, fmt.Errorf("concurrency %d: %v", concurrency, err)
		}
		
		AppLogger.DebugWithFields("Benchmark completed for concurrency", map[string]interface{}{
			"concurrency": concurrency,
			"generationSpeed": result.GenerationSpeed,
			"promptThroughput": result.PromptThroughput,
		})
		
		// Add result for this concurrency level
		results = append(results, ConcurrencyResult{
			Concurrency:          concurrency,
			GenerationThroughput: sanitizeFloat(result.GenerationSpeed),
			PromptThroughput:     sanitizeFloat(result.PromptThroughput),
			MinTTFT:              sanitizeFloat(result.MinTtft),
			MaxTTFT:              sanitizeFloat(result.MaxTtft),
		})
	}
	
	// Return complete benchmark result
	return &BenchmarkResult{
		Model:     model.Name,
		Results:   results,
		Timestamp: time.Now(),
	}, nil
}

// runSingleBenchmarkWithContext runs a benchmark with context support for cancellation (Task 15.3)
func runSingleBenchmarkWithContext(ctx context.Context, model Model, concurrencyLevels []int, maxTokens int, prompt string) (*BenchmarkResult, error) {
	AppLogger.DebugWithFields("runSingleBenchmarkWithContext called", map[string]interface{}{
		"model": model.Name,
		"modelId": model.ID,
	})
	
	// Check for cancellation before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	// Get API key from environment
	apiKey := getAPIKeyForModel(model)
	if apiKey == "" {
		AppLogger.ErrorWithContext(&LogContext{Model: model.Name}, "No API key found for model")
		return nil, fmt.Errorf("no API key found for model %s", model.Name)
	}
	AppLogger.DebugWithFields("Using API key for model", map[string]interface{}{
		"model": model.Name,
		"keyLength": len(apiKey),
	})
	
	var results []ConcurrencyResult
	
	// Run benchmark for each concurrency level
	for _, concurrency := range concurrencyLevels {
		// Check for cancellation before each concurrency level
		select {
		case <-ctx.Done():
			AppLogger.InfoWithFields("Job cancelled at concurrency level", map[string]interface{}{
				"concurrency": concurrency,
			})
			return nil, ctx.Err()
		default:
		}
		
		AppLogger.DebugWithFields("Running benchmark for concurrency level", map[string]interface{}{
			"concurrency": concurrency,
		})
		
		// Create speed measurement
		speedMeasurement := utils.SpeedMeasurement{
			BaseUrl:        model.BaseURL,
			ApiKey:         apiKey,
			ModelName:      model.ID,
			Prompt:         prompt,
			UseRandomInput: false,
			MaxTokens:      maxTokens,
			Latency:        0, // TODO: Measure actual latency
			Concurrency:    concurrency,
		}

		AppLogger.DebugWithFields("SpeedMeasurement config", map[string]interface{}{
			"baseURL": speedMeasurement.BaseUrl,
			"modelName": speedMeasurement.ModelName,
			"maxTokens": speedMeasurement.MaxTokens,
		})

		// Run benchmark with context for cancellation support
		result, err := speedMeasurement.Run(ctx, nil)
		if err != nil {
			AppLogger.ErrorWithFields("Benchmark failed for concurrency", map[string]interface{}{
				"concurrency": concurrency,
				"error": err,
			})
			return nil, fmt.Errorf("concurrency %d: %v", concurrency, err)
		}
		
		AppLogger.DebugWithFields("Benchmark completed for concurrency", map[string]interface{}{
			"concurrency": concurrency,
			"generationSpeed": result.GenerationSpeed,
			"promptThroughput": result.PromptThroughput,
		})
		
		// Add result for this concurrency level
		results = append(results, ConcurrencyResult{
			Concurrency:          concurrency,
			GenerationThroughput: sanitizeFloat(result.GenerationSpeed),
			PromptThroughput:     sanitizeFloat(result.PromptThroughput),
			MinTTFT:              sanitizeFloat(result.MinTtft),
			MaxTTFT:              sanitizeFloat(result.MaxTtft),
		})
	}
	
	// Return complete benchmark result
	return &BenchmarkResult{
		Model:     model.Name,
		Results:   results,
		Timestamp: time.Now(),
	}, nil
}

// sanitizeFloat ensures float values are JSON-serializable (no Inf or NaN)
func sanitizeFloat(value float64) float64 {
	// Check for infinity
	if math.IsInf(value, 1) {
		return math.MaxFloat64 // Positive infinity -> max float
	}
	if math.IsInf(value, -1) {
		return 0 // Negative infinity -> 0 (shouldn't happen in benchmarks)
	}
	// Check for NaN
	if math.IsNaN(value) {
		return 0
	}
	return value
}

// getAPIKeyForModel retrieves the API key for a given model using hybrid approach
func getAPIKeyForModel(model Model) string {
	// If model has API key, use it
	if model.APIKey != "" {
		return model.APIKey
	}
	
	// STEP 1: Handle simple local model names (e.g., "gpt-4", "Qwen/Qwen3-Coder-30B")
	if model.Name != "" && !strings.Contains(model.Name, "|") {
		// Check MODEL1_NAME and MODEL2_NAME to determine which API key to use
		if model1Name := os.Getenv("MODEL1_NAME"); model1Name != "" && model1Name == model.Name {
			if key := os.Getenv("MODEL1_API_KEY"); key != "" {
				return key
			}
		}
		if model2Name := os.Getenv("MODEL2_NAME"); model2Name != "" && model2Name == model.Name {
			if key := os.Getenv("MODEL2_API_KEY"); key != "" {
				return key
			}
		}
	}
	
	// STEP 2: Handle complex Cloud Foundry model IDs (e.g., "serviceId|modelName")
	if model.ID != "" && strings.Contains(model.ID, "|") {
		parts := strings.SplitN(model.ID, "|", 2)
		if len(parts) == 2 {
			serviceID := parts[0]
			modelName := parts[1]
			
			AppLogger.DebugWithFields("Resolving API key for Cloud Foundry model", map[string]interface{}{
			"serviceId": serviceID,
			"modelName": modelName,
		})
			
			// Try to get API key from VCAP_SERVICES
			if IsVCAPServicesAvailable() {
				if apiKey, err := GetAPIKeyForService(serviceID); err == nil && apiKey != "" {
					AppLogger.InfoWithFields("Found API key from VCAP_SERVICES for service", map[string]interface{}{
						"serviceId": serviceID,
					})
					return apiKey
				}
				AppLogger.WarnWithFields("No API key found in VCAP_SERVICES for service", map[string]interface{}{
					"serviceId": serviceID,
				})
			}
			
			// Try environment variables as fallback
			if apiKey, err := GetAPIKeyForEnvironmentModel(serviceID); err == nil && apiKey != "" {
				AppLogger.InfoWithFields("Found API key from environment for service", map[string]interface{}{
					"serviceId": serviceID,
				})
				return apiKey
			}
			AppLogger.WarnWithFields("No API key found in environment for service", map[string]interface{}{
				"serviceId": serviceID,
			})
		}
	}
	
	// STEP 3: Final fallback to generic API_KEY
	fallbackKey := os.Getenv("API_KEY")
	if fallbackKey != "" {
		AppLogger.Info("Using fallback API_KEY")
	}
	return fallbackKey
}

// convertEnhancedToLegacyModel converts an EnhancedModel to a legacy Model for backward compatibility
func convertEnhancedToLegacyModel(enhanced EnhancedModel) Model {
	return Model{
		ID:       enhanced.ID,
		Name:     enhanced.Name,
		Provider: enhanced.Provider,
		BaseURL:  enhanced.BaseURL,
		// APIKey is intentionally not included for security
	}
}

// compareResults compares two benchmark results across multiple concurrency levels
func compareResults(result1, result2 *BenchmarkResult) *Comparison {
	differences := make(map[string]float64)
	
	// Calculate average metrics across all concurrency levels
	var avgGen1, avgPrompt1, avgTTFT1 float64
	var avgGen2, avgPrompt2, avgTTFT2 float64
	
	if len(result1.Results) > 0 {
		for _, r := range result1.Results {
			avgGen1 += r.GenerationThroughput
			avgPrompt1 += r.PromptThroughput
			avgTTFT1 += (r.MinTTFT + r.MaxTTFT) / 2
		}
		avgGen1 /= float64(len(result1.Results))
		avgPrompt1 /= float64(len(result1.Results))
		avgTTFT1 /= float64(len(result1.Results))
	}
	
	if len(result2.Results) > 0 {
		for _, r := range result2.Results {
			avgGen2 += r.GenerationThroughput
			avgPrompt2 += r.PromptThroughput
			avgTTFT2 += (r.MinTTFT + r.MaxTTFT) / 2
		}
		avgGen2 /= float64(len(result2.Results))
		avgPrompt2 /= float64(len(result2.Results))
		avgTTFT2 /= float64(len(result2.Results))
	}
	
	// Calculate percentage differences
	if avgGen2 > 0 {
		differences["generationThroughput"] = ((avgGen1 - avgGen2) / avgGen2) * 100
	}
	if avgPrompt2 > 0 {
		differences["promptThroughput"] = ((avgPrompt1 - avgPrompt2) / avgPrompt2) * 100
	}
	if avgTTFT2 > 0 {
		differences["timeToFirstToken"] = ((avgTTFT1 - avgTTFT2) / avgTTFT2) * 100
	}
	
	// Determine winner (based on average generation throughput)
	winner := "tie"
	if avgGen1 > avgGen2*1.05 { // 5% threshold
		winner = "model1"
	} else if avgGen2 > avgGen1*1.05 {
		winner = "model2"
	}
	
	return &Comparison{
		Winner:      winner,
		Differences: differences,
	}
}

// ExportJSONHandler exports results as JSON file
func ExportJSONHandler(c *gin.Context) {
	var results ComparisonResponse
	
	// Parse request body
	if err := c.ShouldBindJSON(&results); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Bad Request",
			Message: fmt.Sprintf("Invalid request payload: %v", err),
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Generate filename with timestamp
	filename := fmt.Sprintf("benchmark_results_%s.json", time.Now().Format("20060102_150405"))
	
	// Set headers for file download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")
	
	// Return JSON with proper formatting
	c.JSON(http.StatusOK, results)
}

// ExportCSVHandler exports results as CSV file
func ExportCSVHandler(c *gin.Context) {
	var results ComparisonResponse
	
	// Parse request body
	if err := c.ShouldBindJSON(&results); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Bad Request",
			Message: fmt.Sprintf("Invalid request payload: %v", err),
			Code:    http.StatusBadRequest,
		})
		return
	}
	
	// Generate filename with timestamp
	filename := fmt.Sprintf("benchmark_results_%s.csv", time.Now().Format("20060102_150405"))
	
	// Set headers for file download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv")
	
	// Generate CSV content
	csv := generateCSV(results)
	
	c.String(http.StatusOK, csv)
}

// generateCSV converts benchmark results to CSV format
func generateCSV(results ComparisonResponse) string {
	var csv strings.Builder
	
	// CSV Header
	csv.WriteString("Model,Concurrency,Generation Throughput (tokens/s),Prompt Throughput (tokens/s),Min TTFT (s),Max TTFT (s),Timestamp\n")
	
	// Model 1 data
	if results.Model1 != nil {
		for _, result := range results.Model1.Results {
			csv.WriteString(fmt.Sprintf("%s,%d,%.2f,%.2f,%.2f,%.2f,%s\n",
				escapeCsvField(results.Model1.Model),
				result.Concurrency,
				result.GenerationThroughput,
				result.PromptThroughput,
				result.MinTTFT,
				result.MaxTTFT,
				results.Model1.Timestamp.Format(time.RFC3339),
			))
		}
	}
	
	// Model 2 data
	if results.Model2 != nil {
		for _, result := range results.Model2.Results {
			csv.WriteString(fmt.Sprintf("%s,%d,%.2f,%.2f,%.2f,%.2f,%s\n",
				escapeCsvField(results.Model2.Model),
				result.Concurrency,
				result.GenerationThroughput,
				result.PromptThroughput,
				result.MinTTFT,
				result.MaxTTFT,
				results.Model2.Timestamp.Format(time.RFC3339),
			))
		}
	}
	
	// Add comparison section if available
	if results.Comparison != nil {
		csv.WriteString("\nComparison\n")
		csv.WriteString(fmt.Sprintf("Winner,%s\n", results.Comparison.Winner))
		csv.WriteString("\nMetric,Difference (%%)\n")
		for metric, diff := range results.Comparison.Differences {
			csv.WriteString(fmt.Sprintf("%s,%.2f\n", metric, diff))
		}
	}
	
	return csv.String()
}


// escapeCsvField escapes CSV field if it contains special characters
func escapeCsvField(field string) string {
	if strings.ContainsAny(field, ",\"\n") {
		return fmt.Sprintf(`"%s"`, strings.ReplaceAll(field, `"`, `""`))
	}
	return field
}

// SystemStatusHandler returns the global system status
func SystemStatusHandler(c *gin.Context, jobManager *SimpleJobManager) {
	status := jobManager.GetSystemStatus()
	c.JSON(http.StatusOK, status)
}
