package search

import (
	"math"
	"sort"
	"time"
)

// RankingOptions configures additional boosts for search results.
type RankingOptions struct {
	ImportanceWeight float32       // boost multiplier for importance (0 = disabled)
	RecencyWeight    float32       // boost multiplier for recency (0 = disabled)
	RecencyHalfLife  time.Duration // decay rate for recency boost
}

// DefaultRankingOptions returns sensible default ranking options.
func DefaultRankingOptions() RankingOptions {
	return RankingOptions{
		ImportanceWeight: 0.2,              // 20% boost for max importance
		RecencyWeight:    0.1,              // 10% boost for recent items
		RecencyHalfLife:  7 * 24 * time.Hour, // 1 week half-life
	}
}

// ApplyBoosts applies importance and recency boosts to search results.
// The createdTimes map should contain creation timestamps keyed by memory ID.
// Results are re-sorted by boosted score.
//
// Boost formula:
//   - importance_boost = 1 + opts.ImportanceWeight * importance
//   - age = now - created_at
//   - recency_boost = 1 + opts.RecencyWeight * exp(-age/halfLife)
//   - final_score = score * importance_boost * recency_boost
func ApplyBoosts(results []SearchResult, createdTimes map[int64]time.Time, opts RankingOptions) []SearchResult {
	if len(results) == 0 {
		return results
	}

	now := time.Now()

	// Apply boosts to each result
	boosted := make([]SearchResult, len(results))
	copy(boosted, results)

	for i := range boosted {
		boosted[i].Score = calculateBoostedScore(boosted[i], createdTimes, opts, now)
	}

	// Re-sort by boosted score
	sort.Slice(boosted, func(i, j int) bool {
		return boosted[i].Score > boosted[j].Score
	})

	return boosted
}

// calculateBoostedScore computes the final score with importance and recency boosts.
func calculateBoostedScore(result SearchResult, createdTimes map[int64]time.Time, opts RankingOptions, now time.Time) float32 {
	score := result.Score

	// Apply importance boost: 1 + weight * importance
	// importance is expected to be in range [0, 1]
	importanceBoost := float32(1.0)
	if opts.ImportanceWeight > 0 {
		importanceBoost = 1.0 + opts.ImportanceWeight*result.Importance
	}

	// Apply recency boost: 1 + weight * exp(-age/halfLife)
	recencyBoost := float32(1.0)
	if opts.RecencyWeight > 0 && opts.RecencyHalfLife > 0 {
		if createdAt, ok := createdTimes[result.ID]; ok {
			age := now.Sub(createdAt)
			if age < 0 {
				age = 0 // Handle future timestamps gracefully
			}
			// Exponential decay: exp(-age/halfLife)
			decayFactor := math.Exp(-float64(age) / float64(opts.RecencyHalfLife))
			recencyBoost = 1.0 + opts.RecencyWeight*float32(decayFactor)
		}
	}

	return score * importanceBoost * recencyBoost
}

// ApplyBoostsWithTime is a convenience function that uses a custom "now" time.
// Useful for testing with deterministic time values.
func ApplyBoostsWithTime(results []SearchResult, createdTimes map[int64]time.Time, opts RankingOptions, now time.Time) []SearchResult {
	if len(results) == 0 {
		return results
	}

	boosted := make([]SearchResult, len(results))
	copy(boosted, results)

	for i := range boosted {
		boosted[i].Score = calculateBoostedScore(boosted[i], createdTimes, opts, now)
	}

	sort.Slice(boosted, func(i, j int) bool {
		return boosted[i].Score > boosted[j].Score
	})

	return boosted
}
