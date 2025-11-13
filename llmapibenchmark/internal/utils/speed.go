package utils

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"llmapibenchmark/internal/api"

	"github.com/sashabaranov/go-openai"
	"github.com/schollz/progressbar/v3"
)

type SpeedMeasurement struct {
	BaseUrl        string
	ApiKey         string
	ModelName      string
	Prompt         string
	UseRandomInput bool
	NumWords       int
	MaxTokens      int
	Latency        float64
	Concurrency    int
}

type SpeedResult struct {
	Concurrency      int     `json:"concurrency" yaml:"concurrency"`
	GenerationSpeed  float64 `json:"generation_speed" yaml:"generation-speed"`
	PromptThroughput float64 `json:"prompt_throughput" yaml:"prompt-throughput"`
	MaxTtft          float64 `json:"max_ttft" yaml:"max-ttft"`
	MinTtft          float64 `json:"min_ttft" yaml:"min-ttft"`
}

func roundToTwoDecimals(f float64) float64 {
	return math.Round(f*100) / 100
}

// Run measures API generation throughput and TTFT.
func (setup *SpeedMeasurement) Run(ctx context.Context, bar *progressbar.ProgressBar) (SpeedResult, error) {
	config := openai.DefaultConfig(setup.ApiKey)
	
	// Ensure Cloud Foundry GenAI services have the correct /v1 path
	baseURL := setup.BaseUrl
	if strings.Contains(baseURL, "genai-proxy") && !strings.Contains(baseURL, "/v1") {
		// For Cloud Foundry GenAI services, ensure /v1 is included
		if strings.HasSuffix(baseURL, "/openai") {
			// Single-model service: already has /openai, just add /v1
			baseURL = baseURL + "/v1"
			log.Printf("🔧 Adjusted base URL for Cloud Foundry single-model service: %s", baseURL)
		} else if strings.Contains(baseURL, "tanzu-") {
			// Multi-model service: needs /openai/v1
			baseURL = baseURL + "/openai/v1"
			log.Printf("🔧 Adjusted base URL for Cloud Foundry multi-model service: %s", baseURL)
		}
	}
	
	config.BaseURL = baseURL
	client := openai.NewClientWithConfig(config)

	var wg sync.WaitGroup
	var responseTokens sync.Map
	var promptTokens sync.Map
	var ttfts sync.Map
	var threadErrors sync.Map

	start := time.Now()

	// Check for cancellation before starting goroutines
	select {
	case <-ctx.Done():
		return SpeedResult{}, ctx.Err()
	default:
	}

	// Send requests concurrently (restored from debugging version)
	for i := 0; i < setup.Concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			// Check for cancellation in each goroutine before making API call
			select {
			case <-ctx.Done():
				threadErrors.Store(index, ctx.Err())
				return
			default:
			}
			
			var ttft float64
			var completionTokens, inputTokens int
			var err error
			if setup.UseRandomInput {
				ttft, completionTokens, inputTokens, err = api.AskOpenAiRandomInput(ctx, client, setup.ModelName, setup.NumWords, setup.MaxTokens, bar)
			} else {
				ttft, completionTokens, inputTokens, err = api.AskOpenAi(ctx, client, setup.ModelName, setup.Prompt, setup.MaxTokens, bar)
			}
			if err != nil {
				threadErrors.Store(index, err)
				return
			}
			ttfts.Store(index, ttft)
			responseTokens.Store(index, completionTokens)
			promptTokens.Store(index, inputTokens)
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Check if any errors occurred
	var errSlice []error
	threadErrors.Range(func(key, value interface{}) bool {
		errSlice = append(errSlice, value.(error))
		return true
	})
	if len(errSlice) > 0 {
		return SpeedResult{}, fmt.Errorf("error measuring speed: %v", errSlice)
	}

	// Calculate total tokens
	totalResponseTokens := 0
	responseTokens.Range(func(_, value interface{}) bool {
		totalResponseTokens += value.(int)
		return true
	})

	totalPromptTokens := 0
	promptTokens.Range(func(_, value interface{}) bool {
		totalPromptTokens += value.(int)
		return true
	})

	measurement := SpeedResult{}
	measurement.Concurrency = setup.Concurrency

	// Calculate max and min TTFT
	measurement.MaxTtft = 0.0
	measurement.MinTtft = math.Inf(1)
	ttfts.Range(func(_, value interface{}) bool {
		ttft := value.(float64)
		if ttft > measurement.MaxTtft {
			measurement.MaxTtft = ttft
		}
		if ttft < measurement.MinTtft {
			measurement.MinTtft = ttft
		}
		return true
	})
	measurement.MaxTtft = roundToTwoDecimals(measurement.MaxTtft)
	measurement.MinTtft = roundToTwoDecimals(measurement.MinTtft)

	// Calculate speed (tokens/second)
	measurement.GenerationSpeed = roundToTwoDecimals(float64(totalResponseTokens) / (duration.Seconds() - setup.Latency/1000))

	// Calculate Prompt Throughput
	measurement.PromptThroughput = roundToTwoDecimals(float64(totalPromptTokens) / (measurement.MaxTtft - setup.Latency/1000))

	return measurement, nil
}
