package main

import (
	"encoding/json"
	"fmt"

	"go.yaml.in/yaml/v4"
)

func (benchmark *BenchmarkResult) Json() (string, error) {
	prettyJSON, err := json.MarshalIndent(benchmark, "", "    ")
	if err != nil {
		return "", fmt.Errorf("error marshalling JSON: %w", err)
	}

	return string(prettyJSON), nil
}

func (benchmark *BenchmarkResult) Yaml() (string, error) {
	yamlData, err := yaml.Marshal(&benchmark)
	if err != nil {
		return "", fmt.Errorf("error marshalling yaml: %v", err)
	}

	return string(yamlData), nil
}
