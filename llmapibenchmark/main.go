package main

import (
	"log"
	"os"

	"llmapibenchmark/cmd/server"
)

func main() {
	// Set default port if not provided
	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "8080")
	}

	// Run the server
	if err := server.Run(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
