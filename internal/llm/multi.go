// Package llm provides LLM provider adapters for embeddings and text completions.
package llm

import (
	"context"
	"fmt"
	"strings"
)

// MultiEmbedder wraps multiple embedding providers for multi-model support.
type MultiEmbedder struct {
	providers map[string]EmbeddingProvider
	primary   string // primary model name for backward compatibility
}

// NewMultiEmbedder creates a multi-model embedder from a comma-separated list of models.
// Format: "model1,model2,model3" - first model is the primary.
// For OpenAI: "text-embedding-3-small,text-embedding-3-large"
// For Gemini: "gemini-embedding-exp-03-07"
func NewMultiEmbedder(backend, apiKey, modelList string) (*MultiEmbedder, error) {
	if modelList == "" || modelList == "auto" {
		// Default to single model based on backend
		switch backend {
		case "openai":
			modelList = DefaultOpenAIEmbedModel
		case "gemini":
			modelList = DefaultGeminiEmbedModel
		default:
			return nil, fmt.Errorf("unsupported backend: %s", backend)
		}
	}

	models := strings.Split(modelList, ",")
	providers := make(map[string]EmbeddingProvider, len(models))
	var primary string

	for i, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}

		var provider EmbeddingProvider
		var err error

		switch backend {
		case "openai":
			provider, err = NewOpenAIProvider(apiKey, "", model)
		case "gemini":
			provider, err = NewGeminiProvider(apiKey, "", model)
		default:
			return nil, fmt.Errorf("unsupported backend: %s", backend)
		}

		if err != nil {
			return nil, fmt.Errorf("create provider for model %s: %w", model, err)
		}

		providers[model] = provider
		if i == 0 {
			primary = model
		}
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no valid models specified")
	}

	return &MultiEmbedder{
		providers: providers,
		primary:   primary,
	}, nil
}

// Models returns the list of available embedding models.
func (m *MultiEmbedder) Models() []string {
	models := make([]string, 0, len(m.providers))
	// Ensure primary is first
	models = append(models, m.primary)
	for model := range m.providers {
		if model != m.primary {
			models = append(models, model)
		}
	}
	return models
}

// Primary returns the primary model name.
func (m *MultiEmbedder) Primary() string {
	return m.primary
}

// Embed generates an embedding using the primary model.
func (m *MultiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return m.EmbedWithModel(ctx, text, m.primary)
}

// EmbedWithModel generates an embedding using a specific model.
func (m *MultiEmbedder) EmbedWithModel(ctx context.Context, text, model string) ([]float32, error) {
	provider, ok := m.providers[model]
	if !ok {
		return nil, fmt.Errorf("unknown embedding model: %s", model)
	}
	return provider.Embed(ctx, text)
}

// EmbedAll generates embeddings from all configured models.
// Returns a map of model name to embedding.
func (m *MultiEmbedder) EmbedAll(ctx context.Context, text string) (map[string][]float32, error) {
	results := make(map[string][]float32, len(m.providers))
	for model, provider := range m.providers {
		embedding, err := provider.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed with %s: %w", model, err)
		}
		results[model] = embedding
	}
	return results, nil
}

// Dimensions returns the dimensionality of the primary model's embeddings.
func (m *MultiEmbedder) Dimensions() int {
	return m.providers[m.primary].Dimensions()
}

// DimensionsForModel returns the dimensionality for a specific model.
func (m *MultiEmbedder) DimensionsForModel(model string) (int, error) {
	provider, ok := m.providers[model]
	if !ok {
		return 0, fmt.Errorf("unknown embedding model: %s", model)
	}
	return provider.Dimensions(), nil
}

// Model returns the primary model name (implements EmbeddingProvider).
func (m *MultiEmbedder) Model() string {
	return m.primary
}
