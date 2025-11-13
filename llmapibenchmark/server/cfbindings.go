package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// VCAPService represents a Cloud Foundry service binding
type VCAPService struct {
	InstanceGUID  string                 `json:"instance_guid"`
	InstanceName  string                 `json:"instance_name"`
	Name          string                 `json:"name"`
	Plan          string                 `json:"plan"`
	Credentials   map[string]interface{} `json:"credentials"`
	Tags          []string               `json:"tags"`
	Label         string                 `json:"label"`
}

// VCAPServices represents the complete VCAP_SERVICES structure
type VCAPServices struct {
	GenAI []VCAPService `json:"genai"`
}

// ServiceEndpoint represents the endpoint configuration for multi-plan services
type ServiceEndpoint struct {
	APIKey   string `json:"api_key"`
	APIBase  string `json:"api_base"`
	ConfigURL string `json:"config_url"`
}

// AdvertisedModel represents a model from the config URL
type AdvertisedModel struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
}

// ConfigResponse represents the response from the config URL
type ConfigResponse struct {
	AdvertisedModels []AdvertisedModel `json:"advertisedModels"`
}

// EnhancedModel represents a model with service metadata
type EnhancedModel struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	OriginalName     string   `json:"original_name"`
	DisplayName      string   `json:"display_name"`
	IsDefault        bool     `json:"is_default"`
	Capabilities     []string `json:"capabilities"`
	ServiceID        string   `json:"service_id"`
	ServiceName      string   `json:"service_name"`
	Provider         string   `json:"provider"`
	BaseURL          string   `json:"baseUrl"`
	SupportsStreaming bool    `json:"supportsStreaming"`
	HasAPIKey        bool     `json:"has_api_key"`
}

// ServiceInfo represents a discovered service
type ServiceInfo struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Plan      string          `json:"plan"`
	BaseURL   string          `json:"base_url"`
	Models    []EnhancedModel `json:"models"`
	HasAPIKey bool            `json:"has_api_key"`
	APIKey    string          `json:"-"` // Don't serialize API key to JSON for security
}

// fetchModelsFromConfig fetches models from a config URL for multi-plan services
func fetchModelsFromConfig(configURL, apiKey string) ([]AdvertisedModel, error) {
	// Create HTTP client
	client := &http.Client{}

	req, err := http.NewRequest("GET", configURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("config URL returned status %d", resp.StatusCode)
	}

	var configResp ConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&configResp); err != nil {
		return nil, fmt.Errorf("failed to decode config response: %w", err)
	}

	return configResp.AdvertisedModels, nil
}

// parseServiceEndpoint extracts endpoint configuration from credentials
func parseServiceEndpoint(credentials map[string]interface{}) (*ServiceEndpoint, error) {
	endpointData, exists := credentials["endpoint"]
	if !exists {
		return nil, fmt.Errorf("endpoint not found in credentials")
	}

	endpointMap, ok := endpointData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("endpoint is not a valid object")
	}

	endpoint := &ServiceEndpoint{}

	if apiKey, ok := endpointMap["api_key"].(string); ok {
		endpoint.APIKey = apiKey
	}

	if apiBase, ok := endpointMap["api_base"].(string); ok {
		endpoint.APIBase = apiBase
	}

	if configURL, ok := endpointMap["config_url"].(string); ok {
		endpoint.ConfigURL = configURL
	}

	return endpoint, nil
}

// parseLegacyCredentials extracts credentials from legacy format
func parseLegacyCredentials(credentials map[string]interface{}) (string, string, []string, error) {
	var apiKey, baseURL string
	var models []string

	// Extract API key
	if key, ok := credentials["api_key"].(string); ok {
		apiKey = key
	}

	// Extract base URL
	if url, ok := credentials["api_base"].(string); ok {
		baseURL = url
	} else if url, ok := credentials["base_url"].(string); ok {
		baseURL = url
	}

	// Extract primary model
	if modelName, ok := credentials["model_name"].(string); ok {
		models = append(models, modelName)
	}

	// Extract model aliases
	if aliases, ok := credentials["model_aliases"].([]interface{}); ok {
		for _, alias := range aliases {
			if aliasStr, ok := alias.(string); ok {
				// Avoid duplicates
				found := false
				for _, existing := range models {
					if existing == aliasStr {
						found = true
						break
					}
				}
				if !found {
					models = append(models, aliasStr)
				}
			}
		}
	}

	return apiKey, baseURL, models, nil
}

// getProvider extracts provider name from base URL
func getProvider(baseURL string) string {
	baseURL = strings.ToLower(baseURL)
	if strings.Contains(baseURL, "openai.com") {
		return "OpenAI"
	} else if strings.Contains(baseURL, "anthropic.com") {
		return "Anthropic"
	} else if strings.Contains(baseURL, "generativelanguage.googleapis.com") {
		return "Google"
	} else if strings.Contains(baseURL, "cohere.ai") {
		return "Cohere"
	} else if strings.Contains(baseURL, "genai-proxy") || strings.Contains(baseURL, "tanzu") {
		return "GenAI on Tanzu Platform"
	}
	return "Direct OpenAI Compatible"
}

// supportsStreaming determines if a model supports streaming based on provider and capabilities
func supportsStreaming(provider string, capabilities []string) bool {
	// Check capabilities first
	for _, cap := range capabilities {
		if strings.ToLower(cap) == "streaming" {
			return true
		}
	}

	// Provider-based defaults
	switch provider {
	case "OpenAI", "Anthropic", "Google":
		return true
	default:
		return false
	}
}

// DiscoverServicesFromVCAP parses VCAP_SERVICES and returns discovered services
func DiscoverServicesFromVCAP() ([]ServiceInfo, error) {
	vcapServices := os.Getenv("VCAP_SERVICES")
	if vcapServices == "" {
		return nil, fmt.Errorf("VCAP_SERVICES not found")
	}

	var services VCAPServices
	if err := json.Unmarshal([]byte(vcapServices), &services); err != nil {
		return nil, fmt.Errorf("failed to parse VCAP_SERVICES: %w", err)
	}

	var discoveredServices []ServiceInfo

	for _, service := range services.GenAI {
		if service.Credentials == nil {
			AppLogger.WarnWithFields("Service has no credentials, skipping", map[string]interface{}{
				"serviceName": service.InstanceName,
			})
			continue
		}

		serviceID := service.InstanceGUID
		if serviceID == "" {
			serviceID = service.InstanceName
		}
		if serviceID == "" {
			serviceID = service.Name
		}

		serviceName := service.InstanceName
		if serviceName == "" {
			serviceName = service.Name
		}

		plan := service.Plan
		if plan == "" {
			plan = "unknown"
		}

		var baseURL string
		var models []EnhancedModel
		var hasAPIKey bool
		var apiKey string

		// Check if this is a multi-model service by looking for config_url without model_name
		// Multi-model services have endpoint.config_url but no model_name field
		// Single-model services have both endpoint.config_url and model_name field
		hasConfigURL := false
		hasModelName := false
		
		if endpointData, exists := service.Credentials["endpoint"]; exists {
			if endpointMap, ok := endpointData.(map[string]interface{}); ok {
				if _, hasConfig := endpointMap["config_url"]; hasConfig {
					hasConfigURL = true
				}
			}
		}
		
		if _, hasModel := service.Credentials["model_name"]; hasModel {
			hasModelName = true
		}
		
		// Multi-model service: has config_url but no model_name
		if hasConfigURL && !hasModelName {
			endpoint, err := parseServiceEndpoint(service.Credentials)
			if err != nil {
				AppLogger.WarnWithFields("Failed to parse endpoint for service", map[string]interface{}{
					"serviceName": serviceName,
					"error": err,
				})
				continue
			}

			baseURL = endpoint.APIBase
			hasAPIKey = endpoint.APIKey != ""
			apiKey = endpoint.APIKey

			// Fetch models from config URL
			if endpoint.ConfigURL != "" && endpoint.APIKey != "" {
				advertisedModels, err := fetchModelsFromConfig(endpoint.ConfigURL, endpoint.APIKey)
				if err != nil {
					AppLogger.WarnWithFields("Failed to fetch models for service", map[string]interface{}{
						"serviceName": serviceName,
						"error": err,
					})
				} else {
					// Create enhanced models
					for i, model := range advertisedModels {
						modelID := fmt.Sprintf("%s|%s", serviceID, model.Name)
						enhancedModel := EnhancedModel{
							ID:                modelID,
							Name:              modelID,
							OriginalName:      model.Name,
							DisplayName:       model.Description,
							IsDefault:         i == 0,
							Capabilities:      model.Capabilities,
							ServiceID:         serviceID,
							ServiceName:       serviceName,
							Provider:          "GenAI on Tanzu Platform",
							BaseURL:           baseURL,
							SupportsStreaming: supportsStreaming(getProvider(baseURL), model.Capabilities),
							HasAPIKey:         hasAPIKey,
						}
						models = append(models, enhancedModel)
					}
				}
			}
		} else if hasConfigURL && hasModelName {
			// Single-model service: has both config_url and model_name
			endpoint, err := parseServiceEndpoint(service.Credentials)
			if err != nil {
				AppLogger.WarnWithFields("Failed to parse endpoint for single-model service", map[string]interface{}{
					"serviceName": serviceName,
					"error": err,
				})
				continue
			}

			// Use the top-level api_base if available, otherwise fall back to endpoint.api_base
			if apiBase, ok := service.Credentials["api_base"].(string); ok && apiBase != "" {
				baseURL = apiBase
				AppLogger.DebugWithFields("Using top-level api_base for single-model service", map[string]interface{}{
					"baseURL": baseURL,
				})
			} else {
				baseURL = endpoint.APIBase
				AppLogger.DebugWithFields("Using endpoint.api_base for single-model service", map[string]interface{}{
					"baseURL": baseURL,
				})
			}
			hasAPIKey = endpoint.APIKey != ""
			apiKey = endpoint.APIKey

			// For single-model services, use the model_name from credentials
			if modelName, ok := service.Credentials["model_name"].(string); ok && modelName != "" {
				modelID := fmt.Sprintf("%s|%s", serviceID, modelName)
				enhancedModel := EnhancedModel{
					ID:                modelID,
					Name:              modelID,
					OriginalName:      modelName,
					DisplayName:       modelName,
					IsDefault:         true,
					Capabilities:      []string{"chat"}, // Default capability
					ServiceID:         serviceID,
					ServiceName:       serviceName,
					Provider:          "GenAI on Tanzu Platform",
					BaseURL:           baseURL,
					SupportsStreaming: true, // Assume streaming support
					HasAPIKey:         hasAPIKey,
				}
				models = append(models, enhancedModel)
			}
		} else {
			// Handle legacy format (no config_url)
			apiKey, url, modelNames, err := parseLegacyCredentials(service.Credentials)
			if err != nil {
				AppLogger.WarnWithFields("Failed to parse legacy credentials for service", map[string]interface{}{
					"serviceName": serviceName,
					"error": err,
				})
				continue
			}

			baseURL = url
			hasAPIKey = apiKey != ""

			// Create enhanced models
			for i, modelName := range modelNames {
				modelID := fmt.Sprintf("%s|%s", serviceID, modelName)
				enhancedModel := EnhancedModel{
					ID:                modelID,
					Name:              modelID,
					OriginalName:      modelName,
					DisplayName:       modelName,
					IsDefault:         i == 0,
					Capabilities:      []string{},
					ServiceID:         serviceID,
					ServiceName:       serviceName,
					Provider:          "GenAI on Tanzu Platform",
					BaseURL:           baseURL,
					SupportsStreaming: supportsStreaming(getProvider(baseURL), []string{}),
					HasAPIKey:         hasAPIKey,
				}
				models = append(models, enhancedModel)
			}
		}

		// Add service info
		serviceInfo := ServiceInfo{
			ID:        serviceID,
			Name:      serviceName,
			Type:      "genai",
			Plan:      plan,
			BaseURL:   baseURL,
			Models:    models,
			HasAPIKey: hasAPIKey,
			APIKey:    apiKey,
		}

		discoveredServices = append(discoveredServices, serviceInfo)
		AppLogger.InfoWithFields("Discovered service", map[string]interface{}{
		"serviceName": serviceName,
		"plan": plan,
		"models": len(models),
	})
	}

	return discoveredServices, nil
}

// GetAPIKeyForService retrieves the API key for a specific service
func GetAPIKeyForService(serviceID string) (string, error) {
	services, err := DiscoverServicesFromVCAP()
	if err != nil {
		return "", err
	}

	for _, service := range services {
		if service.ID == serviceID {
			if service.APIKey != "" {
				AppLogger.DebugWithFields("Found API key from discovered service", map[string]interface{}{
					"serviceID": serviceID,
					"servicePlan": service.Plan,
					"keyPreview": service.APIKey[:min(10, len(service.APIKey))]+"...",
				})
				return service.APIKey, nil
			} else {
				AppLogger.WarnWithFields("Service found but has no API key", map[string]interface{}{
					"serviceID": serviceID,
					"servicePlan": service.Plan,
					"hasAPIKey": service.HasAPIKey,
				})
			}
		}
	}

	return "", fmt.Errorf("API key not found for service %s", serviceID)
}

// IsVCAPServicesAvailable checks if VCAP_SERVICES is available
func IsVCAPServicesAvailable() bool {
	return os.Getenv("VCAP_SERVICES") != ""
}
