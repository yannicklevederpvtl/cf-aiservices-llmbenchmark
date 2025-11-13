package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/schollz/progressbar/v3"
)

// AskOpenAi sends a prompt to the OpenAI API, processes the response stream and returns stats on it.
func AskOpenAi(ctx context.Context, client *openai.Client, model string, prompt string, maxTokens int, bar *progressbar.ProgressBar) (float64, int, int, error) {
	start := time.Now()

	var (
		timeToFirstToken   float64
		firstTokenSeen     bool
		lastUsage          *openai.Usage
		accumulatedContent string // Accumulate all content to count tokens more accurately
		estimatedTokens    int    // Real-time token estimation
	)

	log.Printf("🔌 Creating chat completion stream for model: %s", model)
	
	// Debug: Check if this is a complex model ID and extract the actual model name
	actualModelName := model
	if strings.Contains(model, "|") {
		parts := strings.SplitN(model, "|", 2)
		if len(parts) == 2 {
			actualModelName = parts[1] // Use the actual model name, not the service ID
			log.Printf("🔍 DEBUG: Extracted model name '%s' from complex ID '%s'", actualModelName, model)
		}
	}
	stream, err := client.CreateChatCompletionStream(
		ctx,
		openai.ChatCompletionRequest{
			Model: actualModelName,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			// Add the deprecated `MaxTokens` for backward compatibility with some older API servers.
			MaxTokens:           maxTokens,
			MaxCompletionTokens: maxTokens,
			Temperature:         1,
			Stream:              true,
			StreamOptions: &openai.StreamOptions{
				IncludeUsage: true,
			},
		},
	)
	if err != nil {
		log.Printf("❌ OpenAI API request failed: %v", err)
		return 0, 0, 0, fmt.Errorf("OpenAI API request failed: %w", err)
	}
	log.Printf("✅ Chat completion stream created successfully")
	defer stream.Close()

	for {
		// Check for context cancellation before receiving
		select {
		case <-ctx.Done():
			log.Printf("🛑 Context cancelled during streaming for model: %s", model)
			return 0, 0, 0, ctx.Err()
		default:
		}
		
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, 0, 0, fmt.Errorf("stream error: %w", err)
		}

		if !firstTokenSeen && len(resp.Choices) > 0 {
			content := resp.Choices[0].Delta.Content
			if strings.TrimSpace(content) != "" {
				timeToFirstToken = time.Since(start).Seconds()
				firstTokenSeen = true
			}
		}

		// Process each chunk, accumulating to response content
		if len(resp.Choices) > 0 {
			content := resp.Choices[0].Delta.Content
			if content != "" {
				accumulatedContent += content

				// Estimate number of tokens in current chunk
				newTokens := estimateTokens(content)
				estimatedTokens += newTokens

				if bar != nil {
					bar.Add(newTokens)
				}
			}
		}

		if resp.Usage != nil {
			lastUsage = resp.Usage
		}
	}

	var promptTokens, completionTokens int
	if lastUsage != nil {
		promptTokens = lastUsage.PromptTokens
		completionTokens = lastUsage.CompletionTokens

		// Final adjustment: if we have actual completion tokens, adjust the progress bar
		if bar != nil && completionTokens > 0 {
			diff := completionTokens - estimatedTokens
			if diff != 0 { // Could be positive or negative
				bar.Add(diff)
			}
		}
	} else {
		// If no usage info, use our estimated tokens as completion tokens
		completionTokens = estimatedTokens
	}

	return timeToFirstToken, completionTokens, promptTokens, nil
}

func AskOpenAiRandomInput(ctx context.Context, client *openai.Client, model string, numWords int, maxTokens int, bar *progressbar.ProgressBar) (float64, int, int, error) {
	prompt := generateRandomPhrase(numWords)
	return AskOpenAi(ctx, client, model, prompt, maxTokens, bar)
}

func estimateTokens(content string) int {
	if content == "" {
		return 0
	}

	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return 0
	}

	words := strings.Fields(content)
	wordCount := len(words)

	// Different strategies based on content type
	if wordCount > 0 {
		// For text with clear word boundaries: ~1.3 tokens per word on average
		// This accounts for subword tokenization in modern models
		return max(1, int(float64(wordCount)*1.3))
	} else {
		// For content without clear word boundaries (like punctuation, single chars)
		// Use character-based estimation: ~3-4 characters per token
		charCount := len(content)
		return max(1, int(float64(charCount)/3.0))
	}
}

// GetFirstAvailableModel retrieves the first available model from the OpenAI API.
func GetFirstAvailableModel(client *openai.Client) (string, error) {
	modelList, err := client.ListModels(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to list models: %w", err)
	}

	if len(modelList.Models) == 0 {
		return "", fmt.Errorf("no models available")
	}

	return modelList.Models[0].ID, nil
}
