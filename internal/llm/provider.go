// Package llm provides LLM provider adapters for embeddings and text completions.
package llm

import (
	"context"
	"fmt"
	"strings"
)

// EmbeddingProvider generates vector embeddings for text.
type EmbeddingProvider interface {
	// Embed generates a vector embedding for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// Model returns the name of the embedding model being used.
	Model() string
	// Dimensions returns the dimensionality of the embeddings.
	Dimensions() int
}

// ChatProvider generates text completions.
type ChatProvider interface {
	// Complete generates a text completion for the given prompt.
	Complete(ctx context.Context, prompt string) (string, error)
	// Model returns the name of the chat model being used.
	Model() string
}

// Provider combines embedding and chat capabilities.
type Provider interface {
	EmbeddingProvider
	ChatProvider
	// EmbedModel returns the name of the embedding model being used.
	EmbedModel() string
}

// multiProvider wraps separate embedding and chat providers into a unified Provider.
type multiProvider struct {
	embed EmbeddingProvider
	chat  ChatProvider
}

func (m *multiProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return m.embed.Embed(ctx, text)
}

func (m *multiProvider) Model() string {
	return m.chat.Model()
}

func (m *multiProvider) Dimensions() int {
	return m.embed.Dimensions()
}

func (m *multiProvider) Complete(ctx context.Context, prompt string) (string, error) {
	return m.chat.Complete(ctx, prompt)
}

// EmbedModel returns the embedding model name.
func (m *multiProvider) EmbedModel() string {
	return m.embed.Model()
}

// NewProvider creates a provider based on backend type.
// backend: "openai" or "gemini"
// apiKey: the API key for the chosen backend
// chatModel: model for chat (empty for default)
// embedModel: model for embeddings (empty for default)
func NewProvider(backend, apiKey, chatModel, embedModel string) (Provider, error) {
	backend = strings.ToLower(strings.TrimSpace(backend))

	switch backend {
	case "openai":
		return NewOpenAIProvider(apiKey, chatModel, embedModel)
	case "gemini":
		return NewGeminiProvider(apiKey, chatModel, embedModel)
	default:
		return nil, fmt.Errorf("unsupported LLM backend: %q (supported: openai, gemini)", backend)
	}
}
