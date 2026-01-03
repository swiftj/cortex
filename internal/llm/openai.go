package llm

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	// DefaultOpenAIChatModel is the default chat model for OpenAI.
	DefaultOpenAIChatModel = "gpt-4o-mini"
	// DefaultOpenAIEmbedModel is the default embedding model for OpenAI.
	DefaultOpenAIEmbedModel = "text-embedding-3-small"
	// OpenAIEmbedDimensions is the dimensionality of text-embedding-3-small.
	OpenAIEmbedDimensions = 1536
)

// OpenAIProvider implements Provider using OpenAI APIs.
type OpenAIProvider struct {
	client     *openai.Client
	chatModel  string
	embedModel string
	embedDims  int
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(apiKey, chatModel, embedModel string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("openai: API key is required")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	if chatModel == "" || chatModel == "auto" {
		chatModel = DefaultOpenAIChatModel
	}
	if embedModel == "" || embedModel == "auto" {
		embedModel = DefaultOpenAIEmbedModel
	}

	// Determine embedding dimensions based on model
	embedDims := OpenAIEmbedDimensions
	if embedModel == "text-embedding-3-large" {
		embedDims = 3072
	}

	return &OpenAIProvider{
		client:     client,
		chatModel:  chatModel,
		embedModel: embedModel,
		embedDims:  embedDims,
	}, nil
}

// Embed generates a vector embedding for the given text.
func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := p.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.F(p.embedModel),
		Input: openai.F(openai.EmbeddingNewParamsInputUnion(openai.EmbeddingNewParamsInputArrayOfStrings{text})),
	})
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("openai embed: no embedding returned")
	}

	// Convert []float64 to []float32
	embedding := make([]float32, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// Model returns the name of the chat model being used.
func (p *OpenAIProvider) Model() string {
	return p.chatModel
}

// Dimensions returns the dimensionality of the embeddings.
func (p *OpenAIProvider) Dimensions() int {
	return p.embedDims
}

// Complete generates a text completion for the given prompt.
func (p *OpenAIProvider) Complete(ctx context.Context, prompt string) (string, error) {
	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.F(p.chatModel),
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		}),
	})
	if err != nil {
		return "", fmt.Errorf("openai complete: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai complete: no completion returned")
	}

	return resp.Choices[0].Message.Content, nil
}

// EmbedModel returns the embedding model name.
func (p *OpenAIProvider) EmbedModel() string {
	return p.embedModel
}
