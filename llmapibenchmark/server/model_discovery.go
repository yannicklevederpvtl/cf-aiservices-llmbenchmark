package server

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Note: EnhancedModel is already defined in cfbindings.go

// EnhancedModelsResponse represents the enhanced response for model discovery
type EnhancedModelsResponse struct {
	Models    []EnhancedModel `json:"models"`
	Count     int             `json:"count"`
	Source    string          `json:"source"`    // "cloud-foundry", "environment", or "default"
	Timestamp time.Time       `json:"timestamp"`
}

// ModelDiscoveryCache provides caching for model discovery
type ModelDiscoveryCache struct {
	models    []EnhancedModel
	source    string
	timestamp time.Time
	mutex     sync.RWMutex
	ttl       time.Duration
}

var (
	modelCache = &ModelDiscoveryCache{
		ttl: 5 * time.Minute, // Cache for 5 minutes
	}
)

// DiscoverEnhancedModels discovers models with comprehensive metadata from all sources
func DiscoverEnhancedModels() (*EnhancedModelsResponse, error) {
	// Check cache first
	if cached := modelCache.get(); cached != nil {
		log.Printf("📋 Using cached model discovery (age: %v)", time.Since(cached.timestamp))
		return &EnhancedModelsResponse{
			Models:    cached.models,
			Count:     len(cached.models),
			Source:    cached.source,
			Timestamp: cached.timestamp,
		}, nil
	}

	log.Printf("🔍 Discovering models from all configuration sources...")
	
	// Get unified configuration from all sources
	config, err := GetUnifiedConfiguration()
	if err != nil {
		log.Printf("⚠️ Failed to get unified configuration: %v", err)
		// Fallback to legacy behavior
		return discoverEnhancedModelsLegacy()
	}

	// Convert to enhanced models
	var enhancedModels []EnhancedModel
	source := "default"
	
	if IsVCAPServicesAvailable() {
		source = "cloud-foundry"
		log.Printf("☁️ Using Cloud Foundry VCAP_SERVICES configuration")
	} else {
		source = "environment"
		log.Printf("🏠 Using environment variable configuration")
	}

	for _, service := range config.Services {
		for _, enhancedModel := range service.Models {
			// Check if API key is available
			hasAPIKey := false
			if IsVCAPServicesAvailable() {
				if apiKey, err := GetAPIKeyForService(service.ID); err == nil && apiKey != "" {
					hasAPIKey = true
				}
			} else {
				if apiKey, err := GetAPIKeyForEnvironmentModel(service.ID); err == nil && apiKey != "" {
					hasAPIKey = true
				}
			}

			enhanced := EnhancedModel{
				ID:               enhancedModel.ID,
				Name:             enhancedModel.ID, // Use full ID as name for API calls
				OriginalName:     enhancedModel.OriginalName,
				DisplayName:      enhancedModel.DisplayName,
				Provider:         enhancedModel.Provider,
				BaseURL:          enhancedModel.BaseURL,
				SupportsStreaming: enhancedModel.SupportsStreaming,
				Capabilities:     enhancedModel.Capabilities,
				ServiceID:        service.ID,
				ServiceName:      service.Name,
				IsDefault:        false, // Will be set below for default models
				HasAPIKey:        hasAPIKey,
			}
			enhancedModels = append(enhancedModels, enhanced)
		}
	}

	// If no models found, add default models
	if len(enhancedModels) == 0 {
		log.Printf("📝 No models found, adding default OpenAI models")
		enhancedModels = getDefaultEnhancedModels()
		source = "default"
	}

	// Mark first model as default if we have models
	if len(enhancedModels) > 0 {
		enhancedModels[0].IsDefault = true
	}

	// Cache the results
	timestamp := time.Now()
	modelCache.set(enhancedModels, source, timestamp)

	log.Printf("✅ Discovered %d models from %s source", len(enhancedModels), source)

	return &EnhancedModelsResponse{
		Models:    enhancedModels,
		Count:     len(enhancedModels),
		Source:    source,
		Timestamp: timestamp,
	}, nil
}

// discoverEnhancedModelsLegacy provides fallback to legacy implementation with enhanced metadata
func discoverEnhancedModelsLegacy() (*EnhancedModelsResponse, error) {
	log.Printf("🔄 Falling back to legacy model discovery")
	
	var enhancedModels []EnhancedModel
	
	// Check for MODEL1 configuration
	if model1Name := os.Getenv("MODEL1_NAME"); model1Name != "" {
		baseURL := os.Getenv("MODEL1_BASE_URL")
		hasAPIKey := os.Getenv("MODEL1_API_KEY") != ""
		
		enhancedModels = append(enhancedModels, EnhancedModel{
			ID:               model1Name,
			Name:             model1Name,
			OriginalName:     model1Name,
			DisplayName:      model1Name,
			Provider:         getProvider(baseURL),
			BaseURL:          baseURL,
			SupportsStreaming: true, // Assume streaming support
			Capabilities:     []string{"chat", "streaming"},
			ServiceID:        "model1",
			ServiceName:      "Model 1",
			IsDefault:        len(enhancedModels) == 0,
			HasAPIKey:        hasAPIKey,
		})
	}
	
	// Check for MODEL2 configuration
	if model2Name := os.Getenv("MODEL2_NAME"); model2Name != "" {
		baseURL := os.Getenv("MODEL2_BASE_URL")
		hasAPIKey := os.Getenv("MODEL2_API_KEY") != ""
		
		enhancedModels = append(enhancedModels, EnhancedModel{
			ID:               model2Name,
			Name:             model2Name,
			OriginalName:     model2Name,
			DisplayName:      model2Name,
			Provider:         getProvider(baseURL),
			BaseURL:          baseURL,
			SupportsStreaming: true,
			Capabilities:     []string{"chat", "streaming"},
			ServiceID:        "model2",
			ServiceName:      "Model 2",
			IsDefault:        false,
			HasAPIKey:        hasAPIKey,
		})
	}
	
	// Fallback: Check for generic MODELS configuration
	if len(enhancedModels) == 0 {
		if modelsStr := os.Getenv("MODELS"); modelsStr != "" {
			baseURL := os.Getenv("BASE_URL")
			if baseURL == "" {
				baseURL = "https://api.openai.com/v1"
			}
			hasAPIKey := os.Getenv("API_KEY") != ""
			
			modelNames := strings.Split(modelsStr, ",")
			for i, name := range modelNames {
				name = strings.TrimSpace(name)
				if name != "" {
					enhancedModels = append(enhancedModels, EnhancedModel{
						ID:               name,
						Name:             name,
						OriginalName:     name,
						DisplayName:      name,
						Provider:         getProvider(baseURL),
						BaseURL:          baseURL,
						SupportsStreaming: true,
						Capabilities:     []string{"chat", "streaming"},
						ServiceID:        fmt.Sprintf("generic_%d", i),
						ServiceName:      "Generic Service",
						IsDefault:        i == 0,
						HasAPIKey:        hasAPIKey,
					})
				}
			}
		}
	}
	
	// If still no models found, return default models
	if len(enhancedModels) == 0 {
		enhancedModels = getDefaultEnhancedModels()
	}

	// Mark first model as default
	if len(enhancedModels) > 0 {
		enhancedModels[0].IsDefault = true
	}

	timestamp := time.Now()
	source := "environment"
	if len(enhancedModels) == 2 && enhancedModels[0].ID == "gpt-4" {
		source = "default"
	}

	// Cache the results
	modelCache.set(enhancedModels, source, timestamp)

	return &EnhancedModelsResponse{
		Models:    enhancedModels,
		Count:     len(enhancedModels),
		Source:    source,
		Timestamp: timestamp,
	}, nil
}

// getDefaultEnhancedModels returns default OpenAI models with enhanced metadata
func getDefaultEnhancedModels() []EnhancedModel {
	return []EnhancedModel{
		{
			ID:               "gpt-4",
			Name:             "gpt-4",
			OriginalName:     "gpt-4",
			DisplayName:      "GPT-4",
			Provider:         "OpenAI",
			BaseURL:          "https://api.openai.com/v1",
			SupportsStreaming: true,
			Capabilities:     []string{"chat", "streaming", "function-calling"},
			ServiceID:        "default",
			ServiceName:      "Default OpenAI Service",
			IsDefault:        true,
			HasAPIKey:        false, // User needs to provide API key
		},
		{
			ID:               "gpt-3.5-turbo",
			Name:             "gpt-3.5-turbo",
			OriginalName:     "gpt-3.5-turbo",
			DisplayName:      "GPT-3.5 Turbo",
			Provider:         "OpenAI",
			BaseURL:          "https://api.openai.com/v1",
			SupportsStreaming: true,
			Capabilities:     []string{"chat", "streaming"},
			ServiceID:        "default",
			ServiceName:      "Default OpenAI Service",
			IsDefault:        false,
			HasAPIKey:        false,
		},
	}
}

// Note: getProvider is already defined in cfbindings.go

// Cache methods
func (c *ModelDiscoveryCache) get() *cachedModels {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	if c.models == nil || time.Since(c.timestamp) > c.ttl {
		return nil
	}
	
	return &cachedModels{
		models:    c.models,
		source:    c.source,
		timestamp: c.timestamp,
	}
}

func (c *ModelDiscoveryCache) set(models []EnhancedModel, source string, timestamp time.Time) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.models = models
	c.source = source
	c.timestamp = timestamp
}

type cachedModels struct {
	models    []EnhancedModel
	source    string
	timestamp time.Time
}

// InvalidateCache clears the model discovery cache
func InvalidateModelCache() {
	modelCache.mutex.Lock()
	defer modelCache.mutex.Unlock()
	
	modelCache.models = nil
	modelCache.source = ""
	modelCache.timestamp = time.Time{}
	
	log.Printf("🗑️ Model discovery cache invalidated")
}
