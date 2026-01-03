package llm

import (
	"context"
	"fmt"
	"strings"
)

const normalizePrompt = `You are a memory normalization assistant. Your task is to rewrite the following text as a concise, factual statement suitable for storage in a memory system.

Guidelines:
- Remove filler words, hedging, and unnecessary context
- Convert questions into declarative statements when possible
- Preserve all factual information and specific details
- Use third person or passive voice for consistency
- Keep the result to 1-2 sentences maximum
- Do not add information that was not in the original text
- If the text is already concise and factual, return it unchanged

Original text:
%s

Normalized memory (respond with only the normalized text, no explanation):`

// Normalizer uses an LLM to normalize memory text for consistent storage.
type Normalizer struct {
	chat ChatProvider
}

// NewNormalizer creates a new Normalizer with the given chat provider.
func NewNormalizer(chat ChatProvider) *Normalizer {
	return &Normalizer{chat: chat}
}

// Normalize rewrites memory text for consistency and conciseness.
// It uses the LLM to convert the input text into a normalized form
// suitable for storage and retrieval.
func (n *Normalizer) Normalize(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", nil
	}

	// Skip normalization for very short texts (likely already concise)
	text = strings.TrimSpace(text)
	if len(text) < 20 {
		return text, nil
	}

	prompt := fmt.Sprintf(normalizePrompt, text)

	result, err := n.chat.Complete(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("normalize memory: %w", err)
	}

	// Clean up the result
	result = strings.TrimSpace(result)

	// If the LLM returned an empty result, return the original
	if result == "" {
		return text, nil
	}

	return result, nil
}

// NormalizeIfNeeded normalizes text only if it exceeds a certain length
// or contains indicators of verbose/informal language.
func (n *Normalizer) NormalizeIfNeeded(ctx context.Context, text string) (string, bool, error) {
	text = strings.TrimSpace(text)

	// Skip empty or very short texts
	if len(text) < 30 {
		return text, false, nil
	}

	// Check for indicators that normalization might help
	needsNormalization := false

	// Check for question marks (convert to statements)
	if strings.Contains(text, "?") {
		needsNormalization = true
	}

	// Check for hedging words
	hedgingWords := []string{"maybe", "perhaps", "i think", "i guess", "kind of", "sort of", "probably"}
	textLower := strings.ToLower(text)
	for _, word := range hedgingWords {
		if strings.Contains(textLower, word) {
			needsNormalization = true
			break
		}
	}

	// Check for excessive length
	if len(text) > 200 {
		needsNormalization = true
	}

	if !needsNormalization {
		return text, false, nil
	}

	normalized, err := n.Normalize(ctx, text)
	if err != nil {
		return text, false, err
	}

	return normalized, true, nil
}
