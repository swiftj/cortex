package llm

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

const (
	// DefaultGeminiChatModel is the default chat model for Gemini.
	DefaultGeminiChatModel = "gemini-2.0-flash-lite"
	// DefaultGeminiEmbedModel is the default embedding model for Gemini.
	DefaultGeminiEmbedModel = "text-embedding-004"
	// GeminiEmbedDimensions is the dimensionality of text-embedding-004.
	GeminiEmbedDimensions = 768
)

// GeminiProvider implements Provider using Google Gemini APIs.
type GeminiProvider struct {
	client     *genai.Client
	chatModel  string
	embedModel string
	embedDims  int
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(apiKey, chatModel, embedModel string) (*GeminiProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini: API key is required")
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: create client: %w", err)
	}

	if chatModel == "" || chatModel == "auto" {
		chatModel = DefaultGeminiChatModel
	}
	if embedModel == "" || embedModel == "auto" {
		embedModel = DefaultGeminiEmbedModel
	}

	return &GeminiProvider{
		client:     client,
		chatModel:  chatModel,
		embedModel: embedModel,
		embedDims:  GeminiEmbedDimensions,
	}, nil
}

// Embed generates a vector embedding for the given text.
func (p *GeminiProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	content := genai.NewUserContentFromText(text)
	result, err := p.client.Models.EmbedContent(ctx, p.embedModel, []*genai.Content{content}, nil)
	if err != nil {
		return nil, fmt.Errorf("gemini embed: %w", err)
	}

	if len(result.Embeddings) == 0 || len(result.Embeddings[0].Values) == 0 {
		return nil, fmt.Errorf("gemini embed: no embedding returned")
	}

	return result.Embeddings[0].Values, nil
}

// Model returns the name of the chat model being used.
func (p *GeminiProvider) Model() string {
	return p.chatModel
}

// Dimensions returns the dimensionality of the embeddings.
func (p *GeminiProvider) Dimensions() int {
	return p.embedDims
}

// Complete generates a text completion for the given prompt.
func (p *GeminiProvider) Complete(ctx context.Context, prompt string) (string, error) {
	result, err := p.client.Models.GenerateContent(ctx, p.chatModel, genai.Text(prompt), nil)
	if err != nil {
		return "", fmt.Errorf("gemini complete: %w", err)
	}

	if result == nil || len(result.Candidates) == 0 {
		return "", fmt.Errorf("gemini complete: no completion returned")
	}

	// Extract text from the first candidate
	candidate := result.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", fmt.Errorf("gemini complete: empty response content")
	}

	// Concatenate all text parts
	var text string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			text += part.Text
		}
	}

	if text == "" {
		return "", fmt.Errorf("gemini complete: no text in response")
	}

	return text, nil
}

// EmbedModel returns the embedding model name.
func (p *GeminiProvider) EmbedModel() string {
	return p.embedModel
}

// Close releases resources held by the provider.
func (p *GeminiProvider) Close() error {
	// The genai client may not have a Close method, but we keep this for future compatibility
	return nil
}
