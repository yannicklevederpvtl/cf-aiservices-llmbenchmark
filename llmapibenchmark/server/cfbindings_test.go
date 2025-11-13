package server

import (
	"os"
	"testing"
)

// Test data for various VCAP_SERVICES scenarios
const (
	// Multi-plan service (GenAI 10.2+)
	multiPlanVCAP = `{
		"genai": [
			{
				"instance_guid": "12345678-1234-1234-1234-123456789abc",
				"instance_name": "my-genai-service",
				"name": "genai-service",
				"plan": "multi",
				"credentials": {
					"endpoint": {
						"api_key": "sk-test-multi-plan-key",
						"api_base": "https://api.example.com/v1",
						"config_url": "https://config.example.com/models"
					}
				},
				"tags": ["genai", "multi-model"],
				"label": "genai"
			}
		]
	}`

	// Legacy single model service
	legacyVCAP = `{
		"genai": [
			{
				"instance_guid": "87654321-4321-4321-4321-cba987654321",
				"instance_name": "legacy-openai-service",
				"name": "openai-service",
				"plan": "standard",
				"credentials": {
					"api_key": "sk-test-legacy-key",
					"api_base": "https://api.openai.com/v1",
					"model_name": "gpt-4",
					"model_aliases": ["gpt-4-turbo", "gpt-4o"]
				},
				"tags": ["openai", "chat"],
				"label": "genai"
			}
		]
	}`

	// Multiple services
	multipleServicesVCAP = `{
		"genai": [
			{
				"instance_guid": "11111111-1111-1111-1111-111111111111",
				"instance_name": "openai-service",
				"name": "openai",
				"plan": "standard",
				"credentials": {
					"api_key": "sk-openai-key",
					"api_base": "https://api.openai.com/v1",
					"model_name": "gpt-4"
				}
			},
			{
				"instance_guid": "22222222-2222-2222-2222-222222222222",
				"instance_name": "anthropic-service",
				"name": "anthropic",
				"plan": "standard",
				"credentials": {
					"api_key": "sk-ant-anthropic-key",
					"api_base": "https://api.anthropic.com/v1",
					"model_name": "claude-3-opus"
				}
			}
		]
	}`

	// Malformed JSON
	malformedVCAP = `{
		"genai": [
			{
				"instance_guid": "malformed-service",
				"credentials": {
					"api_key": "sk-malformed-key"
					// Missing comma and other fields
				}
			}
		]
	}`

	// Empty services
	emptyVCAP = `{
		"genai": []
	}`

	// Missing credentials
	noCredentialsVCAP = `{
		"genai": [
			{
				"instance_guid": "no-creds-service",
				"instance_name": "no-credentials",
				"plan": "standard"
			}
		]
	}`
)

func TestDiscoverServicesFromVCAP_MultiPlan(t *testing.T) {
	// Set up test environment
	originalVCAP := os.Getenv("VCAP_SERVICES")
	defer func() {
		if originalVCAP != "" {
			os.Setenv("VCAP_SERVICES", originalVCAP)
		} else {
			os.Unsetenv("VCAP_SERVICES")
		}
	}()

	// Test multi-plan service
	os.Setenv("VCAP_SERVICES", multiPlanVCAP)

	services, err := DiscoverServicesFromVCAP()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(services))
	}

	service := services[0]
	if service.ID != "12345678-1234-1234-1234-123456789abc" {
		t.Errorf("Expected service ID '12345678-1234-1234-1234-123456789abc', got '%s'", service.ID)
	}

	if service.Name != "my-genai-service" {
		t.Errorf("Expected service name 'my-genai-service', got '%s'", service.Name)
	}

	if service.Plan != "multi" {
		t.Errorf("Expected plan 'multi', got '%s'", service.Plan)
	}

	if service.BaseURL != "https://api.example.com/v1" {
		t.Errorf("Expected base URL 'https://api.example.com/v1', got '%s'", service.BaseURL)
	}

	if !service.HasAPIKey {
		t.Error("Expected service to have API key")
	}
}

func TestDiscoverServicesFromVCAP_Legacy(t *testing.T) {
	// Set up test environment
	originalVCAP := os.Getenv("VCAP_SERVICES")
	defer func() {
		if originalVCAP != "" {
			os.Setenv("VCAP_SERVICES", originalVCAP)
		} else {
			os.Unsetenv("VCAP_SERVICES")
		}
	}()

	// Test legacy service
	os.Setenv("VCAP_SERVICES", legacyVCAP)

	services, err := DiscoverServicesFromVCAP()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(services))
	}

	service := services[0]
	if service.Plan != "standard" {
		t.Errorf("Expected plan 'standard', got '%s'", service.Plan)
	}

	if service.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected base URL 'https://api.openai.com/v1', got '%s'", service.BaseURL)
	}

	// Check models
	if len(service.Models) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(service.Models))
	}

	// Check first model (primary)
	primaryModel := service.Models[0]
	if primaryModel.OriginalName != "gpt-4" {
		t.Errorf("Expected primary model 'gpt-4', got '%s'", primaryModel.OriginalName)
	}

	if !primaryModel.IsDefault {
		t.Error("Expected primary model to be default")
	}

	// Check model aliases
	foundAliases := 0
	for _, model := range service.Models {
		if model.OriginalName == "gpt-4-turbo" || model.OriginalName == "gpt-4o" {
			foundAliases++
		}
	}

	if foundAliases != 2 {
		t.Errorf("Expected 2 model aliases, found %d", foundAliases)
	}
}

func TestDiscoverServicesFromVCAP_MultipleServices(t *testing.T) {
	// Set up test environment
	originalVCAP := os.Getenv("VCAP_SERVICES")
	defer func() {
		if originalVCAP != "" {
			os.Setenv("VCAP_SERVICES", originalVCAP)
		} else {
			os.Unsetenv("VCAP_SERVICES")
		}
	}()

	// Test multiple services
	os.Setenv("VCAP_SERVICES", multipleServicesVCAP)

	services, err := DiscoverServicesFromVCAP()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(services) != 2 {
		t.Fatalf("Expected 2 services, got %d", len(services))
	}

	// Check OpenAI service
	openaiService := services[0]
	if openaiService.Name != "openai-service" {
		t.Errorf("Expected OpenAI service name 'openai-service', got '%s'", openaiService.Name)
	}

	if openaiService.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected OpenAI base URL 'https://api.openai.com/v1', got '%s'", openaiService.BaseURL)
	}

	// Check Anthropic service
	anthropicService := services[1]
	if anthropicService.Name != "anthropic-service" {
		t.Errorf("Expected Anthropic service name 'anthropic-service', got '%s'", anthropicService.Name)
	}

	if anthropicService.BaseURL != "https://api.anthropic.com/v1" {
		t.Errorf("Expected Anthropic base URL 'https://api.anthropic.com/v1', got '%s'", anthropicService.BaseURL)
	}
}

func TestDiscoverServicesFromVCAP_ErrorHandling(t *testing.T) {
	// Set up test environment
	originalVCAP := os.Getenv("VCAP_SERVICES")
	defer func() {
		if originalVCAP != "" {
			os.Setenv("VCAP_SERVICES", originalVCAP)
		} else {
			os.Unsetenv("VCAP_SERVICES")
		}
	}()

	// Test missing VCAP_SERVICES
	os.Unsetenv("VCAP_SERVICES")
	_, err := DiscoverServicesFromVCAP()
	if err == nil {
		t.Error("Expected error for missing VCAP_SERVICES")
	}

	// Test malformed JSON
	os.Setenv("VCAP_SERVICES", malformedVCAP)
	_, err = DiscoverServicesFromVCAP()
	if err == nil {
		t.Error("Expected error for malformed JSON")
	}

	// Test empty services
	os.Setenv("VCAP_SERVICES", emptyVCAP)
	services, err := DiscoverServicesFromVCAP()
	if err != nil {
		t.Fatalf("Expected no error for empty services, got: %v", err)
	}

	if len(services) != 0 {
		t.Errorf("Expected 0 services for empty VCAP, got %d", len(services))
	}

	// Test missing credentials
	os.Setenv("VCAP_SERVICES", noCredentialsVCAP)
	services, err = DiscoverServicesFromVCAP()
	if err != nil {
		t.Fatalf("Expected no error for missing credentials, got: %v", err)
	}

	if len(services) != 0 {
		t.Errorf("Expected 0 services for missing credentials, got %d", len(services))
	}
}

func TestGetProvider(t *testing.T) {
	testCases := []struct {
		baseURL  string
		expected string
	}{
		{"https://api.openai.com/v1", "OpenAI"},
		{"https://api.anthropic.com/v1", "Anthropic"},
		{"https://generativelanguage.googleapis.com/v1", "Google"},
		{"https://api.cohere.ai/v1", "Cohere"},
		{"https://custom-api.example.com", "Custom"},
		{"", "Custom"},
	}

	for _, tc := range testCases {
		result := getProvider(tc.baseURL)
		if result != tc.expected {
			t.Errorf("getProvider(%s) = %s, expected %s", tc.baseURL, result, tc.expected)
		}
	}
}

func TestSupportsStreaming(t *testing.T) {
	testCases := []struct {
		provider    string
		capabilities []string
		expected    bool
	}{
		{"OpenAI", []string{}, true},
		{"Anthropic", []string{}, true},
		{"Google", []string{}, true},
		{"Custom", []string{}, false},
		{"Custom", []string{"streaming"}, true},
		{"Custom", []string{"chat", "streaming"}, true},
		{"Custom", []string{"chat"}, false},
	}

	for _, tc := range testCases {
		result := supportsStreaming(tc.provider, tc.capabilities)
		if result != tc.expected {
			t.Errorf("supportsStreaming(%s, %v) = %v, expected %v", tc.provider, tc.capabilities, result, tc.expected)
		}
	}
}

func TestGetAPIKeyForService(t *testing.T) {
	// Set up test environment
	originalVCAP := os.Getenv("VCAP_SERVICES")
	defer func() {
		if originalVCAP != "" {
			os.Setenv("VCAP_SERVICES", originalVCAP)
		} else {
			os.Unsetenv("VCAP_SERVICES")
		}
	}()

	// Test with legacy service
	os.Setenv("VCAP_SERVICES", legacyVCAP)

	apiKey, err := GetAPIKeyForService("87654321-4321-4321-4321-cba987654321")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if apiKey != "sk-test-legacy-key" {
		t.Errorf("Expected API key 'sk-test-legacy-key', got '%s'", apiKey)
	}

	// Test with non-existent service
	_, err = GetAPIKeyForService("non-existent-service")
	if err == nil {
		t.Error("Expected error for non-existent service")
	}
}

func TestIsVCAPServicesAvailable(t *testing.T) {
	// Set up test environment
	originalVCAP := os.Getenv("VCAP_SERVICES")
	defer func() {
		if originalVCAP != "" {
			os.Setenv("VCAP_SERVICES", originalVCAP)
		} else {
			os.Unsetenv("VCAP_SERVICES")
		}
	}()

	// Test when VCAP_SERVICES is not set
	os.Unsetenv("VCAP_SERVICES")
	if IsVCAPServicesAvailable() {
		t.Error("Expected VCAP_SERVICES to be unavailable")
	}

	// Test when VCAP_SERVICES is set
	os.Setenv("VCAP_SERVICES", legacyVCAP)
	if !IsVCAPServicesAvailable() {
		t.Error("Expected VCAP_SERVICES to be available")
	}
}

func TestParseServiceEndpoint(t *testing.T) {
	// Valid endpoint
	credentials := map[string]interface{}{
		"endpoint": map[string]interface{}{
			"api_key":    "sk-test-key",
			"api_base":   "https://api.example.com/v1",
			"config_url": "https://config.example.com/models",
		},
	}

	endpoint, err := parseServiceEndpoint(credentials)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if endpoint.APIKey != "sk-test-key" {
		t.Errorf("Expected API key 'sk-test-key', got '%s'", endpoint.APIKey)
	}

	if endpoint.APIBase != "https://api.example.com/v1" {
		t.Errorf("Expected API base 'https://api.example.com/v1', got '%s'", endpoint.APIBase)
	}

	if endpoint.ConfigURL != "https://config.example.com/models" {
		t.Errorf("Expected config URL 'https://config.example.com/models', got '%s'", endpoint.ConfigURL)
	}

	// Missing endpoint
	missingEndpoint := map[string]interface{}{
		"api_key": "sk-test-key",
	}

	_, err = parseServiceEndpoint(missingEndpoint)
	if err == nil {
		t.Error("Expected error for missing endpoint")
	}

	// Invalid endpoint type
	invalidEndpoint := map[string]interface{}{
		"endpoint": "not-an-object",
	}

	_, err = parseServiceEndpoint(invalidEndpoint)
	if err == nil {
		t.Error("Expected error for invalid endpoint type")
	}
}

func TestParseLegacyCredentials(t *testing.T) {
	// Valid legacy credentials
	credentials := map[string]interface{}{
		"api_key":       "sk-test-key",
		"api_base":      "https://api.example.com/v1",
		"model_name":    "gpt-4",
		"model_aliases": []interface{}{"gpt-4-turbo", "gpt-4o"},
	}

	apiKey, baseURL, models, err := parseLegacyCredentials(credentials)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if apiKey != "sk-test-key" {
		t.Errorf("Expected API key 'sk-test-key', got '%s'", apiKey)
	}

	if baseURL != "https://api.example.com/v1" {
		t.Errorf("Expected base URL 'https://api.example.com/v1', got '%s'", baseURL)
	}

	expectedModels := []string{"gpt-4", "gpt-4-turbo", "gpt-4o"}
	if len(models) != len(expectedModels) {
		t.Fatalf("Expected %d models, got %d", len(expectedModels), len(models))
	}

	for i, expected := range expectedModels {
		if models[i] != expected {
			t.Errorf("Expected model %d to be '%s', got '%s'", i, expected, models[i])
		}
	}

	// Test with base_url instead of api_base
	credentialsAlt := map[string]interface{}{
		"api_key":    "sk-test-key",
		"base_url":   "https://api.example.com/v1",
		"model_name": "gpt-4",
	}

	_, baseURLAlt, _, err := parseLegacyCredentials(credentialsAlt)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if baseURLAlt != "https://api.example.com/v1" {
		t.Errorf("Expected base URL 'https://api.example.com/v1', got '%s'", baseURLAlt)
	}
}

// Benchmark tests
func BenchmarkDiscoverServicesFromVCAP(b *testing.B) {
	// Set up test environment
	originalVCAP := os.Getenv("VCAP_SERVICES")
	defer func() {
		if originalVCAP != "" {
			os.Setenv("VCAP_SERVICES", originalVCAP)
		} else {
			os.Unsetenv("VCAP_SERVICES")
		}
	}()

	os.Setenv("VCAP_SERVICES", multipleServicesVCAP)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DiscoverServicesFromVCAP()
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}
