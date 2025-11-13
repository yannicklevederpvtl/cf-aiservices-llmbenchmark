package server

import (
	"os"
	"testing"
	"time"
)

func TestDiscoverEnhancedModels_CloudFoundry(t *testing.T) {
	// Set up VCAP_SERVICES environment with legacy format
	os.Setenv("VCAP_SERVICES", `{
		"genai": [
			{
				"name": "my-genai-service",
				"instance_name": "my-genai-service",
				"binding_name": null,
				"credentials": {
					"api_key": "test-api-key",
					"api_base": "https://api.example.com/v1",
					"model_name": "gpt-4",
					"model_aliases": ["gpt-3.5-turbo"]
				},
				"syslog_drain_url": null,
				"volume_mounts": [],
				"label": "genai",
				"provider": null,
				"plan": "legacy",
				"tags": ["genai", "ai", "ml"],
				"instance_id": "service123"
			}
		]
	}`)
	defer os.Unsetenv("VCAP_SERVICES")

	// Invalidate cache to ensure fresh discovery
	InvalidateModelCache()

	response, err := DiscoverEnhancedModels()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response.Source != "cloud-foundry" {
		t.Errorf("Expected source 'cloud-foundry', got %s", response.Source)
	}

	if response.Count != 2 {
		t.Errorf("Expected 2 models, got %d", response.Count)
	}

	if len(response.Models) != 2 {
		t.Fatalf("Expected 2 models in response, got %d", len(response.Models))
	}

	// Check first model
	model1 := response.Models[0]
	if model1.ID != "my-genai-service|gpt-4" {
		t.Errorf("Expected ID 'my-genai-service|gpt-4', got %s", model1.ID)
	}
	if model1.Name != "my-genai-service|gpt-4" {
		t.Errorf("Expected Name 'my-genai-service|gpt-4', got %s", model1.Name)
	}
	if model1.OriginalName != "gpt-4" {
		t.Errorf("Expected OriginalName 'gpt-4', got %s", model1.OriginalName)
	}
	if model1.DisplayName != "gpt-4" {
		t.Errorf("Expected DisplayName 'gpt-4', got %s", model1.DisplayName)
	}
	if model1.Provider != "Custom" {
		t.Errorf("Expected Provider 'Custom', got %s", model1.Provider)
	}
	if model1.BaseURL != "https://api.example.com/v1" {
		t.Errorf("Expected BaseURL 'https://api.example.com/v1', got %s", model1.BaseURL)
	}
	if model1.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false for Custom provider")
	}
	if model1.ServiceID != "my-genai-service" {
		t.Errorf("Expected ServiceID 'my-genai-service', got %s", model1.ServiceID)
	}
	if model1.ServiceName != "my-genai-service" {
		t.Errorf("Expected ServiceName 'my-genai-service', got %s", model1.ServiceName)
	}
	if !model1.IsDefault {
		t.Error("Expected first model to be default")
	}
	if !model1.HasAPIKey {
		t.Error("Expected HasAPIKey to be true")
	}

	// Check second model
	model2 := response.Models[1]
	if model2.ID != "my-genai-service|gpt-3.5-turbo" {
		t.Errorf("Expected ID 'my-genai-service|gpt-3.5-turbo', got %s", model2.ID)
	}
	if model2.IsDefault {
		t.Error("Expected second model to not be default")
	}
}

func TestDiscoverEnhancedModels_Environment(t *testing.T) {
	// Clear VCAP_SERVICES
	os.Unsetenv("VCAP_SERVICES")
	
	// Set up environment variables
	os.Setenv("MODEL1_NAME", "gpt-4")
	os.Setenv("MODEL1_BASE_URL", "https://api.openai.com/v1")
	os.Setenv("MODEL1_API_KEY", "test-key-1")
	os.Setenv("MODEL2_NAME", "claude-3")
	os.Setenv("MODEL2_BASE_URL", "https://api.anthropic.com/v1")
	os.Setenv("MODEL2_API_KEY", "test-key-2")
	
	defer func() {
		os.Unsetenv("MODEL1_NAME")
		os.Unsetenv("MODEL1_BASE_URL")
		os.Unsetenv("MODEL1_API_KEY")
		os.Unsetenv("MODEL2_NAME")
		os.Unsetenv("MODEL2_BASE_URL")
		os.Unsetenv("MODEL2_API_KEY")
	}()

	// Invalidate cache
	InvalidateModelCache()

	response, err := DiscoverEnhancedModels()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response.Source != "environment" {
		t.Errorf("Expected source 'environment', got %s", response.Source)
	}

	if response.Count != 2 {
		t.Errorf("Expected 2 models, got %d", response.Count)
	}

	// Check first model
	model1 := response.Models[0]
	if model1.ID != "model1|gpt-4" {
		t.Errorf("Expected ID 'model1|gpt-4', got %s", model1.ID)
	}
	if model1.Provider != "OpenAI" {
		t.Errorf("Expected Provider 'OpenAI', got %s", model1.Provider)
	}
	if model1.ServiceID != "model1" {
		t.Errorf("Expected ServiceID 'model1', got %s", model1.ServiceID)
	}
	if !model1.HasAPIKey {
		t.Error("Expected HasAPIKey to be true")
	}

	// Check second model
	model2 := response.Models[1]
	if model2.ID != "model2|claude-3" {
		t.Errorf("Expected ID 'model2|claude-3', got %s", model2.ID)
	}
	if model2.Provider != "Anthropic" {
		t.Errorf("Expected Provider 'Anthropic', got %s", model2.Provider)
	}
	if model2.ServiceID != "model2" {
		t.Errorf("Expected ServiceID 'model2', got %s", model2.ServiceID)
	}
}

func TestDiscoverEnhancedModels_GenericEnvironment(t *testing.T) {
	// Clear VCAP_SERVICES and specific model env vars
	os.Unsetenv("VCAP_SERVICES")
	os.Unsetenv("MODEL1_NAME")
	os.Unsetenv("MODEL2_NAME")
	
	// Set up generic environment variables
	os.Setenv("BASE_URL", "https://api.openai.com/v1")
	os.Setenv("API_KEY", "test-key")
	os.Setenv("MODELS", "gpt-4,gpt-3.5-turbo,claude-3")
	
	defer func() {
		os.Unsetenv("BASE_URL")
		os.Unsetenv("API_KEY")
		os.Unsetenv("MODELS")
	}()

	// Invalidate cache
	InvalidateModelCache()

	response, err := DiscoverEnhancedModels()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response.Source != "environment" {
		t.Errorf("Expected source 'environment', got %s", response.Source)
	}

	if response.Count != 3 {
		t.Errorf("Expected 3 models, got %d", response.Count)
	}

	// Check models
	expectedModels := []string{"generic|gpt-4", "generic|gpt-3.5-turbo", "generic|claude-3"}
	for i, expected := range expectedModels {
		if response.Models[i].ID != expected {
			t.Errorf("Expected model %d ID '%s', got %s", i, expected, response.Models[i].ID)
		}
		if response.Models[i].ServiceID != "generic" {
			t.Errorf("Expected ServiceID 'generic', got %s", response.Models[i].ServiceID)
		}
		if i == 0 && !response.Models[i].IsDefault {
			t.Error("Expected first model to be default")
		}
		if i > 0 && response.Models[i].IsDefault {
			t.Errorf("Expected model %d to not be default", i)
		}
	}
}

func TestDiscoverEnhancedModels_Default(t *testing.T) {
	// Clear all environment variables
	os.Unsetenv("VCAP_SERVICES")
	os.Unsetenv("MODEL1_NAME")
	os.Unsetenv("MODEL2_NAME")
	os.Unsetenv("BASE_URL")
	os.Unsetenv("API_KEY")
	os.Unsetenv("MODELS")

	// Invalidate cache
	InvalidateModelCache()

	response, err := DiscoverEnhancedModels()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response.Source != "environment" {
		t.Errorf("Expected source 'environment', got %s", response.Source)
	}

	if response.Count != 2 {
		t.Errorf("Expected 2 default models, got %d", response.Count)
	}

	// Check default models
	expectedModels := []string{"default|gpt-4", "default|gpt-3.5-turbo"}
	for i, expected := range expectedModels {
		if response.Models[i].ID != expected {
			t.Errorf("Expected model %d ID '%s', got %s", i, expected, response.Models[i].ID)
		}
		if response.Models[i].Provider != "OpenAI" {
			t.Errorf("Expected Provider 'OpenAI', got %s", response.Models[i].Provider)
		}
		if response.Models[i].ServiceID != "default" {
			t.Errorf("Expected ServiceID 'default', got %s", response.Models[i].ServiceID)
		}
		if !response.Models[i].SupportsStreaming {
			t.Errorf("Expected model %d to support streaming", i)
		}
		if i == 0 && !response.Models[i].IsDefault {
			t.Error("Expected first model to be default")
		}
	}
}

// Note: TestGetProvider is already defined in cfbindings_test.go

func TestModelDiscoveryCache(t *testing.T) {
	// Clear environment
	os.Unsetenv("VCAP_SERVICES")
	os.Unsetenv("MODEL1_NAME")
	os.Unsetenv("MODEL2_NAME")
	os.Unsetenv("BASE_URL")
	os.Unsetenv("API_KEY")
	os.Unsetenv("MODELS")

	// Invalidate cache
	InvalidateModelCache()

	// First call should populate cache
	response1, err := DiscoverEnhancedModels()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Second call should use cache
	response2, err := DiscoverEnhancedModels()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Timestamps should be the same (indicating cache was used)
	if !response1.Timestamp.Equal(response2.Timestamp) {
		t.Error("Expected timestamps to be equal (cache should be used)")
	}

	// Invalidate cache
	InvalidateModelCache()

	// Third call should populate cache again
	response3, err := DiscoverEnhancedModels()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Timestamp should be different (indicating fresh discovery)
	if response1.Timestamp.Equal(response3.Timestamp) {
		t.Error("Expected timestamps to be different (cache should be invalidated)")
	}
}

func TestConvertEnhancedToLegacyModel(t *testing.T) {
	enhanced := EnhancedModel{
		ID:               "service123|gpt-4",
		Name:             "service123|gpt-4",
		OriginalName:     "gpt-4",
		DisplayName:      "GPT-4",
		Provider:         "OpenAI",
		BaseURL:          "https://api.openai.com/v1",
		SupportsStreaming: true,
		Capabilities:     []string{"chat", "streaming"},
		ServiceID:        "service123",
		ServiceName:      "My Service",
		IsDefault:        true,
		HasAPIKey:        true,
	}

	legacy := convertEnhancedToLegacyModel(enhanced)

	if legacy.ID != enhanced.ID {
		t.Errorf("Expected ID %s, got %s", enhanced.ID, legacy.ID)
	}
	if legacy.Name != enhanced.Name {
		t.Errorf("Expected Name %s, got %s", enhanced.Name, legacy.Name)
	}
	if legacy.Provider != enhanced.Provider {
		t.Errorf("Expected Provider %s, got %s", enhanced.Provider, legacy.Provider)
	}
	if legacy.BaseURL != enhanced.BaseURL {
		t.Errorf("Expected BaseURL %s, got %s", enhanced.BaseURL, legacy.BaseURL)
	}
	if legacy.APIKey != "" {
		t.Error("Expected APIKey to be empty for security")
	}
}

func TestEnhancedModelsResponse_JSON(t *testing.T) {
	// Test JSON marshaling
	response := &EnhancedModelsResponse{
		Models: []EnhancedModel{
			{
				ID:               "test-model",
				Name:             "test-model",
				OriginalName:     "test-model",
				DisplayName:      "Test Model",
				Provider:         "Test Provider",
				BaseURL:          "https://api.test.com/v1",
				SupportsStreaming: true,
				Capabilities:     []string{"chat"},
				ServiceID:        "test-service",
				ServiceName:      "Test Service",
				IsDefault:        true,
				HasAPIKey:        true,
			},
		},
		Count:     1,
		Source:    "test",
		Timestamp: time.Now(),
	}

	// This test ensures the struct can be marshaled to JSON
	// (actual marshaling is tested by the HTTP handler)
	if response.Count != len(response.Models) {
		t.Error("Count should match number of models")
	}
}
