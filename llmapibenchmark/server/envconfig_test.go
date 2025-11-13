package server

import (
	"os"
	"testing"
)

func TestDiscoverServicesFromEnvironment_Model1Config(t *testing.T) {
	// Set up test environment
	originalEnv := map[string]string{
		"MODEL1_NAME":     os.Getenv("MODEL1_NAME"),
		"MODEL1_BASE_URL": os.Getenv("MODEL1_BASE_URL"),
		"MODEL1_API_KEY":  os.Getenv("MODEL1_API_KEY"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Test MODEL1 configuration
	os.Setenv("MODEL1_NAME", "gpt-4")
	os.Setenv("MODEL1_BASE_URL", "https://api.openai.com/v1")
	os.Setenv("MODEL1_API_KEY", "sk-test-key")

	services, err := DiscoverServicesFromEnvironment()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(services))
	}

	service := services[0]
	if service.ID != "model1" {
		t.Errorf("Expected service ID 'model1', got '%s'", service.ID)
	}

	if service.Name != "Model 1" {
		t.Errorf("Expected service name 'Model 1', got '%s'", service.Name)
	}

	if service.Type != "environment" {
		t.Errorf("Expected service type 'environment', got '%s'", service.Type)
	}

	if service.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected base URL 'https://api.openai.com/v1', got '%s'", service.BaseURL)
	}

	if !service.HasAPIKey {
		t.Error("Expected service to have API key")
	}

	if len(service.Models) != 1 {
		t.Fatalf("Expected 1 model, got %d", len(service.Models))
	}

	model := service.Models[0]
	if model.OriginalName != "gpt-4" {
		t.Errorf("Expected model name 'gpt-4', got '%s'", model.OriginalName)
	}

	if model.ID != "model1|gpt-4" {
		t.Errorf("Expected model ID 'model1|gpt-4', got '%s'", model.ID)
	}

	if !model.IsDefault {
		t.Error("Expected model to be default")
	}
}

func TestDiscoverServicesFromEnvironment_Model2Config(t *testing.T) {
	// Set up test environment
	originalEnv := map[string]string{
		"MODEL2_NAME":     os.Getenv("MODEL2_NAME"),
		"MODEL2_BASE_URL": os.Getenv("MODEL2_BASE_URL"),
		"MODEL2_API_KEY":  os.Getenv("MODEL2_API_KEY"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Test MODEL2 configuration
	os.Setenv("MODEL2_NAME", "claude-3-opus")
	os.Setenv("MODEL2_BASE_URL", "https://api.anthropic.com/v1")
	os.Setenv("MODEL2_API_KEY", "sk-ant-test-key")

	services, err := DiscoverServicesFromEnvironment()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(services))
	}

	service := services[0]
	if service.ID != "model2" {
		t.Errorf("Expected service ID 'model2', got '%s'", service.ID)
	}

	if service.Name != "Model 2" {
		t.Errorf("Expected service name 'Model 2', got '%s'", service.Name)
	}

	if service.BaseURL != "https://api.anthropic.com/v1" {
		t.Errorf("Expected base URL 'https://api.anthropic.com/v1', got '%s'", service.BaseURL)
	}

	model := service.Models[0]
	if model.OriginalName != "claude-3-opus" {
		t.Errorf("Expected model name 'claude-3-opus', got '%s'", model.OriginalName)
	}

	if model.ID != "model2|claude-3-opus" {
		t.Errorf("Expected model ID 'model2|claude-3-opus', got '%s'", model.ID)
	}

	if model.IsDefault {
		t.Error("Expected model to not be default")
	}
}

func TestDiscoverServicesFromEnvironment_BothModels(t *testing.T) {
	// Set up test environment
	originalEnv := map[string]string{
		"MODEL1_NAME":     os.Getenv("MODEL1_NAME"),
		"MODEL1_BASE_URL": os.Getenv("MODEL1_BASE_URL"),
		"MODEL1_API_KEY":  os.Getenv("MODEL1_API_KEY"),
		"MODEL2_NAME":     os.Getenv("MODEL2_NAME"),
		"MODEL2_BASE_URL": os.Getenv("MODEL2_BASE_URL"),
		"MODEL2_API_KEY":  os.Getenv("MODEL2_API_KEY"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Test both MODEL1 and MODEL2 configuration
	os.Setenv("MODEL1_NAME", "gpt-4")
	os.Setenv("MODEL1_BASE_URL", "https://api.openai.com/v1")
	os.Setenv("MODEL1_API_KEY", "sk-test-key")

	os.Setenv("MODEL2_NAME", "claude-3-opus")
	os.Setenv("MODEL2_BASE_URL", "https://api.anthropic.com/v1")
	os.Setenv("MODEL2_API_KEY", "sk-ant-test-key")

	services, err := DiscoverServicesFromEnvironment()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(services) != 2 {
		t.Fatalf("Expected 2 services, got %d", len(services))
	}

	// Check first service (MODEL1)
	service1 := services[0]
	if service1.ID != "model1" {
		t.Errorf("Expected first service ID 'model1', got '%s'", service1.ID)
	}

	// Check second service (MODEL2)
	service2 := services[1]
	if service2.ID != "model2" {
		t.Errorf("Expected second service ID 'model2', got '%s'", service2.ID)
	}
}

func TestDiscoverServicesFromEnvironment_GenericConfig(t *testing.T) {
	// Set up test environment
	originalEnv := map[string]string{
		"BASE_URL": os.Getenv("BASE_URL"),
		"API_KEY":  os.Getenv("API_KEY"),
		"MODELS":   os.Getenv("MODELS"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Test generic configuration
	os.Setenv("BASE_URL", "https://api.openai.com/v1")
	os.Setenv("API_KEY", "sk-test-key")
	os.Setenv("MODELS", "gpt-4,gpt-3.5-turbo")

	services, err := DiscoverServicesFromEnvironment()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(services))
	}

	service := services[0]
	if service.ID != "generic" {
		t.Errorf("Expected service ID 'generic', got '%s'", service.ID)
	}

	if service.Name != "Generic Service" {
		t.Errorf("Expected service name 'Generic Service', got '%s'", service.Name)
	}

	if service.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected base URL 'https://api.openai.com/v1', got '%s'", service.BaseURL)
	}

	if len(service.Models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(service.Models))
	}

	// Check first model
	model1 := service.Models[0]
	if model1.OriginalName != "gpt-4" {
		t.Errorf("Expected first model name 'gpt-4', got '%s'", model1.OriginalName)
	}

	if model1.ID != "generic|gpt-4" {
		t.Errorf("Expected first model ID 'generic|gpt-4', got '%s'", model1.ID)
	}

	if !model1.IsDefault {
		t.Error("Expected first model to be default")
	}

	// Check second model
	model2 := service.Models[1]
	if model2.OriginalName != "gpt-3.5-turbo" {
		t.Errorf("Expected second model name 'gpt-3.5-turbo', got '%s'", model2.OriginalName)
	}

	if model2.ID != "generic|gpt-3.5-turbo" {
		t.Errorf("Expected second model ID 'generic|gpt-3.5-turbo', got '%s'", model2.ID)
	}

	if model2.IsDefault {
		t.Error("Expected second model to not be default")
	}
}

func TestDiscoverServicesFromEnvironment_GenericConfigDefaults(t *testing.T) {
	// Set up test environment
	originalEnv := map[string]string{
		"BASE_URL": os.Getenv("BASE_URL"),
		"API_KEY":  os.Getenv("API_KEY"),
		"MODELS":   os.Getenv("MODELS"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Test generic configuration with defaults
	os.Setenv("API_KEY", "sk-test-key")
	// Don't set BASE_URL or MODELS to test defaults

	services, err := DiscoverServicesFromEnvironment()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(services))
	}

	service := services[0]
	if service.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected default base URL 'https://api.openai.com/v1', got '%s'", service.BaseURL)
	}

	if len(service.Models) != 2 {
		t.Fatalf("Expected 2 default models, got %d", len(service.Models))
	}

	// Check default models
	expectedModels := []string{"gpt-4", "gpt-3.5-turbo"}
	for i, expected := range expectedModels {
		if service.Models[i].OriginalName != expected {
			t.Errorf("Expected model %d to be '%s', got '%s'", i, expected, service.Models[i].OriginalName)
		}
	}
}

func TestDiscoverServicesFromEnvironment_NoConfig(t *testing.T) {
	// Set up test environment
	originalEnv := map[string]string{
		"MODEL1_NAME":     os.Getenv("MODEL1_NAME"),
		"MODEL1_BASE_URL": os.Getenv("MODEL1_BASE_URL"),
		"MODEL1_API_KEY":  os.Getenv("MODEL1_API_KEY"),
		"MODEL2_NAME":     os.Getenv("MODEL2_NAME"),
		"MODEL2_BASE_URL": os.Getenv("MODEL2_BASE_URL"),
		"MODEL2_API_KEY":  os.Getenv("MODEL2_API_KEY"),
		"BASE_URL":        os.Getenv("BASE_URL"),
		"API_KEY":         os.Getenv("API_KEY"),
		"MODELS":          os.Getenv("MODELS"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Clear all environment variables
	os.Unsetenv("MODEL1_NAME")
	os.Unsetenv("MODEL1_BASE_URL")
	os.Unsetenv("MODEL1_API_KEY")
	os.Unsetenv("MODEL2_NAME")
	os.Unsetenv("MODEL2_BASE_URL")
	os.Unsetenv("MODEL2_API_KEY")
	os.Unsetenv("BASE_URL")
	os.Unsetenv("API_KEY")
	os.Unsetenv("MODELS")

	services, err := DiscoverServicesFromEnvironment()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(services) != 0 {
		t.Errorf("Expected 0 services for no configuration, got %d", len(services))
	}
}

func TestGetAPIKeyForEnvironmentModel(t *testing.T) {
	// Set up test environment
	originalEnv := map[string]string{
		"MODEL1_API_KEY": os.Getenv("MODEL1_API_KEY"),
		"MODEL2_API_KEY": os.Getenv("MODEL2_API_KEY"),
		"API_KEY":        os.Getenv("API_KEY"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Test MODEL1 API key
	os.Setenv("MODEL1_API_KEY", "sk-model1-key")
	apiKey, err := GetAPIKeyForEnvironmentModel("model1")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if apiKey != "sk-model1-key" {
		t.Errorf("Expected 'sk-model1-key', got '%s'", apiKey)
	}

	// Test MODEL2 API key
	os.Setenv("MODEL2_API_KEY", "sk-model2-key")
	apiKey, err = GetAPIKeyForEnvironmentModel("model2")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if apiKey != "sk-model2-key" {
		t.Errorf("Expected 'sk-model2-key', got '%s'", apiKey)
	}

	// Test generic API key
	os.Setenv("API_KEY", "sk-generic-key")
	apiKey, err = GetAPIKeyForEnvironmentModel("generic")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if apiKey != "sk-generic-key" {
		t.Errorf("Expected 'sk-generic-key', got '%s'", apiKey)
	}

	// Test unknown service ID
	_, err = GetAPIKeyForEnvironmentModel("unknown")
	if err == nil {
		t.Error("Expected error for unknown service ID")
	}
}

func TestIsEnvironmentConfigAvailable(t *testing.T) {
	// Set up test environment
	originalEnv := map[string]string{
		"MODEL1_NAME":     os.Getenv("MODEL1_NAME"),
		"MODEL1_BASE_URL": os.Getenv("MODEL1_BASE_URL"),
		"MODEL2_NAME":     os.Getenv("MODEL2_NAME"),
		"MODEL2_BASE_URL": os.Getenv("MODEL2_BASE_URL"),
		"BASE_URL":        os.Getenv("BASE_URL"),
		"API_KEY":         os.Getenv("API_KEY"),
		"MODELS":          os.Getenv("MODELS"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Clear all environment variables
	os.Unsetenv("MODEL1_NAME")
	os.Unsetenv("MODEL1_BASE_URL")
	os.Unsetenv("MODEL2_NAME")
	os.Unsetenv("MODEL2_BASE_URL")
	os.Unsetenv("BASE_URL")
	os.Unsetenv("API_KEY")
	os.Unsetenv("MODELS")

	// Test with no configuration
	if IsEnvironmentConfigAvailable() {
		t.Error("Expected no environment configuration to be available")
	}

	// Test with MODEL1 configuration
	os.Setenv("MODEL1_NAME", "gpt-4")
	os.Setenv("MODEL1_BASE_URL", "https://api.openai.com/v1")
	if !IsEnvironmentConfigAvailable() {
		t.Error("Expected environment configuration to be available with MODEL1")
	}

	// Clear and test with MODEL2 configuration
	os.Unsetenv("MODEL1_NAME")
	os.Unsetenv("MODEL1_BASE_URL")
	os.Setenv("MODEL2_NAME", "claude-3-opus")
	os.Setenv("MODEL2_BASE_URL", "https://api.anthropic.com/v1")
	if !IsEnvironmentConfigAvailable() {
		t.Error("Expected environment configuration to be available with MODEL2")
	}

	// Clear and test with generic configuration
	os.Unsetenv("MODEL2_NAME")
	os.Unsetenv("MODEL2_BASE_URL")
	os.Setenv("API_KEY", "sk-test-key")
	if !IsEnvironmentConfigAvailable() {
		t.Error("Expected environment configuration to be available with API_KEY")
	}
}

func TestValidateEnvironmentConfig(t *testing.T) {
	// Set up test environment
	originalEnv := map[string]string{
		"MODEL1_NAME":     os.Getenv("MODEL1_NAME"),
		"MODEL1_BASE_URL": os.Getenv("MODEL1_BASE_URL"),
		"MODEL2_NAME":     os.Getenv("MODEL2_NAME"),
		"MODEL2_BASE_URL": os.Getenv("MODEL2_BASE_URL"),
		"BASE_URL":        os.Getenv("BASE_URL"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Clear all environment variables
	os.Unsetenv("MODEL1_NAME")
	os.Unsetenv("MODEL1_BASE_URL")
	os.Unsetenv("MODEL2_NAME")
	os.Unsetenv("MODEL2_BASE_URL")
	os.Unsetenv("BASE_URL")

	// Test with no configuration
	errors := ValidateEnvironmentConfig()
	if len(errors) != 0 {
		t.Errorf("Expected no validation errors, got: %v", errors)
	}

	// Test with invalid MODEL1 configuration
	os.Setenv("MODEL1_NAME", "gpt-4")
	os.Setenv("MODEL1_BASE_URL", "invalid-url")
	errors = ValidateEnvironmentConfig()
	if len(errors) == 0 {
		t.Error("Expected validation errors for invalid MODEL1_BASE_URL")
	}

	// Test with missing MODEL1_BASE_URL
	os.Setenv("MODEL1_BASE_URL", "")
	errors = ValidateEnvironmentConfig()
	if len(errors) == 0 {
		t.Error("Expected validation errors for missing MODEL1_BASE_URL")
	}

	// Test with invalid BASE_URL
	os.Unsetenv("MODEL1_NAME")
	os.Setenv("BASE_URL", "invalid-url")
	errors = ValidateEnvironmentConfig()
	if len(errors) == 0 {
		t.Error("Expected validation errors for invalid BASE_URL")
	}
}

func TestGetUnifiedConfiguration(t *testing.T) {
	// Set up test environment
	originalEnv := map[string]string{
		"MODEL1_NAME":     os.Getenv("MODEL1_NAME"),
		"MODEL1_BASE_URL": os.Getenv("MODEL1_BASE_URL"),
		"MODEL1_API_KEY":  os.Getenv("MODEL1_API_KEY"),
		"VCAP_SERVICES":   os.Getenv("VCAP_SERVICES"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Clear VCAP_SERVICES and test with environment variables
	os.Unsetenv("VCAP_SERVICES")
	os.Setenv("MODEL1_NAME", "gpt-4")
	os.Setenv("MODEL1_BASE_URL", "https://api.openai.com/v1")
	os.Setenv("MODEL1_API_KEY", "sk-test-key")

	config, err := GetUnifiedConfiguration()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.Source != "environment" {
		t.Errorf("Expected source 'environment', got '%s'", config.Source)
	}

	if len(config.Services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(config.Services))
	}

	// Test with no configuration (should use defaults)
	os.Unsetenv("MODEL1_NAME")
	os.Unsetenv("MODEL1_BASE_URL")
	os.Unsetenv("MODEL1_API_KEY")

	config, err = GetUnifiedConfiguration()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.Source != "default" {
		t.Errorf("Expected source 'default', got '%s'", config.Source)
	}

	if len(config.Services) != 1 {
		t.Fatalf("Expected 1 default service, got %d", len(config.Services))
	}

	if config.Services[0].ID != "default" {
		t.Errorf("Expected default service ID 'default', got '%s'", config.Services[0].ID)
	}
}

// Benchmark tests
func BenchmarkDiscoverServicesFromEnvironment(b *testing.B) {
	// Set up test environment
	originalEnv := map[string]string{
		"MODEL1_NAME":     os.Getenv("MODEL1_NAME"),
		"MODEL1_BASE_URL": os.Getenv("MODEL1_BASE_URL"),
		"MODEL1_API_KEY":  os.Getenv("MODEL1_API_KEY"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	os.Setenv("MODEL1_NAME", "gpt-4")
	os.Setenv("MODEL1_BASE_URL", "https://api.openai.com/v1")
	os.Setenv("MODEL1_API_KEY", "sk-test-key")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DiscoverServicesFromEnvironment()
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}

func BenchmarkGetUnifiedConfiguration(b *testing.B) {
	// Set up test environment
	originalEnv := map[string]string{
		"MODEL1_NAME":     os.Getenv("MODEL1_NAME"),
		"MODEL1_BASE_URL": os.Getenv("MODEL1_BASE_URL"),
		"MODEL1_API_KEY":  os.Getenv("MODEL1_API_KEY"),
		"VCAP_SERVICES":   os.Getenv("VCAP_SERVICES"),
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	os.Unsetenv("VCAP_SERVICES")
	os.Setenv("MODEL1_NAME", "gpt-4")
	os.Setenv("MODEL1_BASE_URL", "https://api.openai.com/v1")
	os.Setenv("MODEL1_API_KEY", "sk-test-key")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetUnifiedConfiguration()
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}
