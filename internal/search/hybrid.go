// Package search provides hybrid search combining vector and lexical retrieval.
package search

import (
	"context"
	"sort"

	"github.com/johnswift/cortex/internal/db"
)

// EmbeddingProvider generates embeddings for text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// HybridSearcher combines vector and lexical search with score fusion.
type HybridSearcher struct {
	db    *db.DB
	embed EmbeddingProvider
	alpha float32 // vector weight, default 0.7
}

// NewHybridSearcher creates a new hybrid searcher.
// The alpha parameter controls the blend between vector and lexical search:
// - alpha = 1.0: pure vector search
// - alpha = 0.0: pure lexical search
// - alpha = 0.7 (default): 70% vector, 30% lexical
func NewHybridSearcher(database *db.DB, embedder EmbeddingProvider) *HybridSearcher {
	return &HybridSearcher{
		db:    database,
		embed: embedder,
		alpha: 0.7,
	}
}

// WithAlpha returns a new HybridSearcher with the specified alpha value.
func (h *HybridSearcher) WithAlpha(alpha float32) *HybridSearcher {
	return &HybridSearcher{
		db:    h.db,
		embed: h.embed,
		alpha: alpha,
	}
}

// SearchParams configures the search operation.
type SearchParams struct {
	Query  string  // The search query text
	Limit  int     // Maximum number of results to return
	Hybrid bool    // false = vector only, true = hybrid fusion
	Alpha  float32 // 0 = lexical only, 1 = vector only (overrides default if > 0)
	Model  string  // Optional: filter by embedding model (empty = any model)
}

// SearchResult is a memory with fused score.
type SearchResult struct {
	ID         int64    `json:"id"`
	Text       string   `json:"text"`
	Kind       string   `json:"kind"`
	Source     *string  `json:"source,omitempty"`
	Tags       []string `json:"tags"`
	Importance float32  `json:"importance"`
	Score      float32  `json:"score"`
}

// Search performs hybrid search with score fusion.
// When params.Hybrid is false, only vector search is performed.
// When params.Hybrid is true, both vector and lexical results are fused.
func (h *HybridSearcher) Search(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}

	// Generate embedding for the query
	embedding, err := h.embed.Embed(ctx, params.Query)
	if err != nil {
		return nil, err
	}

	// Determine effective alpha
	alpha := h.alpha
	if params.Alpha > 0 {
		alpha = params.Alpha
	}

	// Vector-only search
	if !params.Hybrid {
		return h.vectorOnlySearch(ctx, embedding, params.Limit, params.Model)
	}

	// Hybrid search with score fusion
	return h.hybridSearch(ctx, params.Query, embedding, params.Limit, alpha, params.Model)
}

// vectorOnlySearch performs pure vector similarity search.
func (h *HybridSearcher) vectorOnlySearch(ctx context.Context, embedding []float32, limit int, model string) ([]SearchResult, error) {
	results, err := h.db.VectorSearch(ctx, db.VectorSearchParams{
		Embedding: embedding,
		Limit:     limit,
		Model:     model,
	})
	if err != nil {
		return nil, err
	}

	return memoriesToResults(results), nil
}

// hybridSearch performs combined vector and lexical search with score fusion.
func (h *HybridSearcher) hybridSearch(ctx context.Context, query string, embedding []float32, limit int, alpha float32, model string) ([]SearchResult, error) {
	// Fetch more results than needed to improve fusion quality
	fetchLimit := limit * 3
	if fetchLimit < 20 {
		fetchLimit = 20
	}

	// Run vector and lexical searches
	vectorResults, err := h.db.VectorSearch(ctx, db.VectorSearchParams{
		Embedding: embedding,
		Limit:     fetchLimit,
		Model:     model,
	})
	if err != nil {
		return nil, err
	}

	lexicalResults, err := h.db.LexicalSearch(ctx, db.LexicalSearchParams{
		Query: query,
		Limit: fetchLimit,
	})
	if err != nil {
		return nil, err
	}

	// Handle empty results
	if len(vectorResults) == 0 && len(lexicalResults) == 0 {
		return []SearchResult{}, nil
	}

	// If one set is empty, return the other
	if len(vectorResults) == 0 {
		results := memoriesToResults(lexicalResults)
		return truncateResults(results, limit), nil
	}
	if len(lexicalResults) == 0 {
		results := memoriesToResults(vectorResults)
		return truncateResults(results, limit), nil
	}

	// Normalize scores to 0-1 range
	normalizeScores(vectorResults)
	normalizeScores(lexicalResults)

	// Build lookup maps for efficient merging
	vectorScores := make(map[int64]float32, len(vectorResults))
	for _, r := range vectorResults {
		vectorScores[r.ID] = r.Score
	}

	lexicalScores := make(map[int64]float32, len(lexicalResults))
	for _, r := range lexicalResults {
		lexicalScores[r.ID] = r.Score
	}

	// Merge results by ID with score fusion
	merged := make(map[int64]*fusedResult)

	// Add vector results
	for _, r := range vectorResults {
		lexScore := lexicalScores[r.ID] // 0 if not present
		fusedScore := alpha*r.Score + (1-alpha)*lexScore
		merged[r.ID] = &fusedResult{
			memory: r,
			score:  fusedScore,
		}
	}

	// Add lexical results not already present
	for _, r := range lexicalResults {
		if _, exists := merged[r.ID]; !exists {
			vecScore := vectorScores[r.ID] // 0 if not present
			fusedScore := alpha*vecScore + (1-alpha)*r.Score
			merged[r.ID] = &fusedResult{
				memory: r,
				score:  fusedScore,
			}
		}
	}

	// Convert to slice and sort by fused score
	results := make([]SearchResult, 0, len(merged))
	for _, fr := range merged {
		results = append(results, SearchResult{
			ID:         fr.memory.ID,
			Text:       fr.memory.Text,
			Kind:       fr.memory.Kind,
			Source:     fr.memory.Source,
			Tags:       fr.memory.Tags,
			Importance: fr.memory.Importance,
			Score:      fr.score,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return truncateResults(results, limit), nil
}

// fusedResult holds a memory with its fused score during merging.
type fusedResult struct {
	memory db.MemoryWithScore
	score  float32
}

// normalizeScores normalizes scores to 0-1 by dividing by max.
func normalizeScores(results []db.MemoryWithScore) {
	if len(results) == 0 {
		return
	}

	// Find max score
	maxScore := results[0].Score
	for _, r := range results[1:] {
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}

	// Avoid division by zero
	if maxScore <= 0 {
		return
	}

	// Normalize
	for i := range results {
		results[i].Score = results[i].Score / maxScore
	}
}

// memoriesToResults converts MemoryWithScore slice to SearchResult slice.
func memoriesToResults(memories []db.MemoryWithScore) []SearchResult {
	results := make([]SearchResult, len(memories))
	for i, m := range memories {
		results[i] = SearchResult{
			ID:         m.ID,
			Text:       m.Text,
			Kind:       m.Kind,
			Source:     m.Source,
			Tags:       m.Tags,
			Importance: m.Importance,
			Score:      m.Score,
		}
	}
	return results
}

// truncateResults returns at most limit results.
func truncateResults(results []SearchResult, limit int) []SearchResult {
	if len(results) <= limit {
		return results
	}
	return results[:limit]
}
