package server

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"
)

// EnvironmentConfig represents configuration from environment variables
type EnvironmentConfig struct {
	Source      string          `json:"source"`
	Services    []ServiceInfo   `json:"services"`
	LastUpdated time.Time       `json:"last_updated"`
}

// ModelConfig represents a single model configuration from environment variables
type ModelConfig struct {
	Name        string
	BaseURL     string
	APIKey      string
	ServiceID   string
	ServiceName string
	IsDefault   bool
}

// DiscoverServicesFromEnvironment discovers services from environment variables
func DiscoverServicesFromEnvironment() ([]ServiceInfo, error) {
	var services []ServiceInfo

	// Try Pattern 1: MODEL1/MODEL2 configuration
	model1Config, err1 := parseModel1Config()
	model2Config, err2 := parseModel2Config()

	if err1 == nil && model1Config != nil {
		// Create service for MODEL1
		service1 := createServiceFromModelConfig(model1Config, "model1", "Model 1")
		services = append(services, service1)
	}

	if err2 == nil && model2Config != nil {
		// Create service for MODEL2
		service2 := createServiceFromModelConfig(model2Config, "model2", "Model 2")
		services = append(services, service2)
	}

	// If we have MODEL1/MODEL2 configs, return them
	if len(services) > 0 {
		return services, nil
	}

	// Try Pattern 2: Generic configuration (BASE_URL/API_KEY/MODELS)
	genericServices, err := parseGenericConfig()
	if err == nil && len(genericServices) > 0 {
		return genericServices, nil
	}

	// If no environment configuration found, return empty slice
	return []ServiceInfo{}, nil
}

// parseModel1Config parses MODEL1_* environment variables
func parseModel1Config() (*ModelConfig, error) {
	name := os.Getenv("MODEL1_NAME")
	if name == "" {
		return nil, fmt.Errorf("MODEL1_NAME not set")
	}

	baseURL := os.Getenv("MODEL1_BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("MODEL1_BASE_URL not set")
	}

	// Validate URL format
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("invalid MODEL1_BASE_URL: %w", err)
	}

	apiKey := os.Getenv("MODEL1_API_KEY")
	if apiKey == "" {
		log.Printf("⚠️ MODEL1_API_KEY not set for model %s", name)
	}

	return &ModelConfig{
		Name:        name,
		BaseURL:     baseURL,
		APIKey:      apiKey,
		ServiceID:   "model1",
		ServiceName: "Model 1",
		IsDefault:   true,
	}, nil
}

// parseModel2Config parses MODEL2_* environment variables
func parseModel2Config() (*ModelConfig, error) {
	name := os.Getenv("MODEL2_NAME")
	if name == "" {
		return nil, fmt.Errorf("MODEL2_NAME not set")
	}

	baseURL := os.Getenv("MODEL2_BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("MODEL2_BASE_URL not set")
	}

	// Validate URL format
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("invalid MODEL2_BASE_URL: %w", err)
	}

	apiKey := os.Getenv("MODEL2_API_KEY")
	if apiKey == "" {
		log.Printf("⚠️ MODEL2_API_KEY not set for model %s", name)
	}

	return &ModelConfig{
		Name:        name,
		BaseURL:     baseURL,
		APIKey:      apiKey,
		ServiceID:   "model2",
		ServiceName: "Model 2",
		IsDefault:   false,
	}, nil
}

// parseGenericConfig parses BASE_URL/API_KEY/MODELS environment variables
func parseGenericConfig() ([]ServiceInfo, error) {
	// Check if any generic configuration is present
	baseURL := os.Getenv("BASE_URL")
	apiKey := os.Getenv("API_KEY")
	modelsStr := os.Getenv("MODELS")

	// If no generic configuration is present, return empty
	if baseURL == "" && apiKey == "" && modelsStr == "" {
		return nil, fmt.Errorf("no generic configuration found")
	}

	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// Validate URL format
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("invalid BASE_URL: %w", err)
	}

	if apiKey == "" {
		log.Printf("⚠️ API_KEY not set for generic configuration")
	}

	if modelsStr == "" {
		// Default models if none specified
		modelsStr = "gpt-4,gpt-3.5-turbo"
	}

	modelNames := strings.Split(modelsStr, ",")
	var models []EnhancedModel

	for i, name := range modelNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		modelID := fmt.Sprintf("generic|%s", name)
		enhancedModel := EnhancedModel{
			ID:                modelID,
			Name:              modelID,
			OriginalName:      name,
			DisplayName:       name,
			IsDefault:         i == 0,
			Capabilities:      []string{},
			ServiceID:         "generic",
			ServiceName:       "Generic Service",
			Provider:          getProvider(baseURL),
			BaseURL:           baseURL,
			SupportsStreaming: supportsStreaming(getProvider(baseURL), []string{}),
			HasAPIKey:         apiKey != "",
		}
		models = append(models, enhancedModel)
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no valid models found in MODELS configuration")
	}

	service := ServiceInfo{
		ID:        "generic",
		Name:      "Generic Service",
		Type:      "environment",
		Plan:      "default",
		BaseURL:   baseURL,
		Models:    models,
		HasAPIKey: apiKey != "",
	}

	return []ServiceInfo{service}, nil
}

// createServiceFromModelConfig creates a ServiceInfo from a ModelConfig
func createServiceFromModelConfig(config *ModelConfig, serviceID, serviceName string) ServiceInfo {
	modelID := fmt.Sprintf("%s|%s", serviceID, config.Name)
	enhancedModel := EnhancedModel{
		ID:                modelID,
		Name:              modelID,
		OriginalName:      config.Name,
		DisplayName:       config.Name,
		IsDefault:         config.IsDefault,
		Capabilities:      []string{},
		ServiceID:         serviceID,
		ServiceName:       serviceName,
		Provider:          getProvider(config.BaseURL),
		BaseURL:           config.BaseURL,
		SupportsStreaming: supportsStreaming(getProvider(config.BaseURL), []string{}),
		HasAPIKey:         config.APIKey != "",
	}

	return ServiceInfo{
		ID:        serviceID,
		Name:      serviceName,
		Type:      "environment",
		Plan:      "default",
		BaseURL:   config.BaseURL,
		Models:    []EnhancedModel{enhancedModel},
		HasAPIKey: config.APIKey != "",
	}
}

// GetAPIKeyForEnvironmentModel retrieves API key for environment-configured models
func GetAPIKeyForEnvironmentModel(serviceID string) (string, error) {
	switch serviceID {
	case "model1":
		apiKey := os.Getenv("MODEL1_API_KEY")
		if apiKey == "" {
			return "", fmt.Errorf("MODEL1_API_KEY not set")
		}
		return apiKey, nil
	case "model2":
		apiKey := os.Getenv("MODEL2_API_KEY")
		if apiKey == "" {
			return "", fmt.Errorf("MODEL2_API_KEY not set")
		}
		return apiKey, nil
	case "generic":
		apiKey := os.Getenv("API_KEY")
		if apiKey == "" {
			return "", fmt.Errorf("API_KEY not set")
		}
		return apiKey, nil
	default:
		return "", fmt.Errorf("unknown service ID: %s", serviceID)
	}
}

// IsEnvironmentConfigAvailable checks if environment configuration is available
func IsEnvironmentConfigAvailable() bool {
	// Check for MODEL1 configuration
	if os.Getenv("MODEL1_NAME") != "" && os.Getenv("MODEL1_BASE_URL") != "" {
		return true
	}

	// Check for MODEL2 configuration
	if os.Getenv("MODEL2_NAME") != "" && os.Getenv("MODEL2_BASE_URL") != "" {
		return true
	}

	// Check for generic configuration
	if os.Getenv("API_KEY") != "" || os.Getenv("BASE_URL") != "" || os.Getenv("MODELS") != "" {
		return true
	}

	return false
}

// ValidateEnvironmentConfig validates environment variable configuration
func ValidateEnvironmentConfig() []string {
	var errors []string

	// Validate MODEL1 configuration if present
	if model1Name := os.Getenv("MODEL1_NAME"); model1Name != "" {
		if baseURL := os.Getenv("MODEL1_BASE_URL"); baseURL == "" {
			errors = append(errors, "MODEL1_BASE_URL is required when MODEL1_NAME is set")
		} else if !isValidURL(baseURL) {
			errors = append(errors, fmt.Sprintf("Invalid MODEL1_BASE_URL: %s", baseURL))
		}
	}

	// Validate MODEL2 configuration if present
	if model2Name := os.Getenv("MODEL2_NAME"); model2Name != "" {
		if baseURL := os.Getenv("MODEL2_BASE_URL"); baseURL == "" {
			errors = append(errors, "MODEL2_BASE_URL is required when MODEL2_NAME is set")
		} else if !isValidURL(baseURL) {
			errors = append(errors, fmt.Sprintf("Invalid MODEL2_BASE_URL: %s", baseURL))
		}
	}

	// Validate generic configuration if present
	if baseURL := os.Getenv("BASE_URL"); baseURL != "" {
		if !isValidURL(baseURL) {
			errors = append(errors, fmt.Sprintf("Invalid BASE_URL: %s", baseURL))
		}
	}

	return errors
}

// isValidURL validates if a URL is properly formatted
func isValidURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	// Check if it has a scheme and host
	return parsedURL.Scheme != "" && parsedURL.Host != ""
}

// GetUnifiedConfiguration returns configuration from all available sources
func GetUnifiedConfiguration() (*EnvironmentConfig, error) {
	var allServices []ServiceInfo
	source := "none"

	// Priority 1: VCAP_SERVICES (Cloud Foundry)
	if IsVCAPServicesAvailable() {
		cfServices, err := DiscoverServicesFromVCAP()
		if err != nil {
			log.Printf("⚠️ Failed to discover VCAP_SERVICES: %v", err)
		} else {
			allServices = append(allServices, cfServices...)
			source = "cloud-foundry"
		}
	}

	// Priority 2: Environment variables (fallback)
	if len(allServices) == 0 {
		envServices, err := DiscoverServicesFromEnvironment()
		if err != nil {
			log.Printf("⚠️ Failed to discover environment configuration: %v", err)
		} else {
			allServices = append(allServices, envServices...)
			source = "environment"
		}
	}

	// Priority 3: Default models (last resort)
	if len(allServices) == 0 {
		defaultServices := createDefaultServices()
		allServices = append(allServices, defaultServices...)
		source = "default"
	}

	return &EnvironmentConfig{
		Source:      source,
		Services:    allServices,
		LastUpdated: time.Now(),
	}, nil
}

// createDefaultServices creates default OpenAI services when no configuration is found
func createDefaultServices() []ServiceInfo {
	models := []EnhancedModel{
		{
			ID:                "default|gpt-4",
			Name:              "default|gpt-4",
			OriginalName:      "gpt-4",
			DisplayName:       "GPT-4",
			IsDefault:         true,
			Capabilities:      []string{"chat"},
			ServiceID:         "default",
			ServiceName:       "Default OpenAI",
			Provider:          "OpenAI",
			BaseURL:           "https://api.openai.com/v1",
			SupportsStreaming: true,
			HasAPIKey:         false,
		},
		{
			ID:                "default|gpt-3.5-turbo",
			Name:              "default|gpt-3.5-turbo",
			OriginalName:      "gpt-3.5-turbo",
			DisplayName:       "GPT-3.5 Turbo",
			IsDefault:         false,
			Capabilities:      []string{"chat"},
			ServiceID:         "default",
			ServiceName:       "Default OpenAI",
			Provider:          "OpenAI",
			BaseURL:           "https://api.openai.com/v1",
			SupportsStreaming: true,
			HasAPIKey:         false,
		},
	}

	return []ServiceInfo{
		{
			ID:        "default",
			Name:      "Default OpenAI",
			Type:      "default",
			Plan:      "default",
			BaseURL:   "https://api.openai.com/v1",
			Models:    models,
			HasAPIKey: false,
		},
	}
}
