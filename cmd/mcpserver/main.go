// Package main provides the entry point for the Cortex MCP memory server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/johnswift/cortex/internal/db"
	"github.com/johnswift/cortex/internal/llm"
	"github.com/johnswift/cortex/internal/mcp"
	"github.com/johnswift/cortex/internal/search"
)

// Config holds all configuration for the Cortex server.
type Config struct {
	DatabaseURL string
	TenantID    string
	LMBackend   string
	LMModel     string
	EmbedModel  string
	OpenAIKey   string
	GeminiKey   string
}

func main() {
	// Configure logging to stderr (stdout reserved for MCP JSON-RPC)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if err := run(); err != nil {
		log.Fatalf("cortex: %v", err)
	}
}

func run() error {
	// Load configuration from environment
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Create context with signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("cortex: starting MCP memory server (tenant=%s, backend=%s)", cfg.TenantID, cfg.LMBackend)

	// Initialize database connection
	database, err := db.New(ctx, cfg.DatabaseURL, cfg.TenantID)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer database.Close()

	// Run migrations
	log.Println("cortex: running database migrations")
	if err := database.Migrate(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Initialize LLM provider
	provider, err := initLLMProvider(cfg)
	if err != nil {
		return fmt.Errorf("init LLM provider: %w", err)
	}

	// Initialize hybrid searcher
	searcher := search.NewHybridSearcher(database, provider)

	// Create MCP server
	server := mcp.NewServer("cortex", "1.0.0")

	// Register memory tools
	registerMemoryTools(server, database, provider, searcher)

	// Run the MCP server (blocks until context is cancelled)
	log.Println("cortex: MCP server ready, listening on stdio")
	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	log.Println("cortex: shutting down gracefully")
	return nil
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		DatabaseURL: getEnv("DATABASE_URL", ""),
		TenantID:    getEnv("TENANT_ID", "local"),
		LMBackend:   getEnv("LM_BACKEND", "openai"),
		LMModel:     getEnv("LM_MODEL", "auto"),
		EmbedModel:  getEnv("EMBED_MODEL", "auto"),
		OpenAIKey:   getEnv("OPENAI_API_KEY", ""),
		GeminiKey:   getEnv("GEMINI_API_KEY", ""),
	}

	// Validate required configuration
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	// Validate LLM backend and API key
	switch cfg.LMBackend {
	case "openai":
		if cfg.OpenAIKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required when LM_BACKEND=openai")
		}
	case "gemini":
		if cfg.GeminiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY environment variable is required when LM_BACKEND=gemini")
		}
	default:
		return nil, fmt.Errorf("invalid LM_BACKEND: %q (must be 'openai' or 'gemini')", cfg.LMBackend)
	}

	return cfg, nil
}

func initLLMProvider(cfg *Config) (llm.Provider, error) {
	var apiKey string
	switch cfg.LMBackend {
	case "openai":
		apiKey = cfg.OpenAIKey
	case "gemini":
		apiKey = cfg.GeminiKey
	}
	return llm.NewProvider(cfg.LMBackend, apiKey, cfg.LMModel, cfg.EmbedModel)
}

func registerMemoryTools(server *mcp.Server, database *db.DB, provider llm.Provider, searcher *search.HybridSearcher) {
	// Register all memory tools with their handlers
	server.RegisterTool(mcp.MemoryAddTool(), createAddHandler(database, provider))
	server.RegisterTool(mcp.MemorySearchTool(), createSearchHandler(searcher))
	server.RegisterTool(mcp.MemoryUpdateTool(), createUpdateHandler(database, provider))
	server.RegisterTool(mcp.MemoryDeleteTool(), createDeleteHandler(database))
}

func createAddHandler(database *db.DB, provider llm.Provider) mcp.Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var args mcp.MemoryAddArgs
		if err := json.Unmarshal(params, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		if args.Text == "" {
			return nil, fmt.Errorf("text is required")
		}

		// Set defaults
		kind := args.Kind
		if kind == "" {
			kind = "note"
		}

		importance := float32(0.5)
		if args.Importance != nil {
			importance = *args.Importance
		}

		// Add memory to database
		id, err := database.AddMemory(ctx, db.AddMemoryParams{
			Kind:       kind,
			Text:       args.Text,
			Source:     args.Source,
			Tags:       args.Tags,
			Importance: importance,
			TTLDays:    args.TTLDays,
			Meta:       nil,
		})
		if err != nil {
			return nil, fmt.Errorf("add memory: %w", err)
		}

		// Generate and store embedding
		embedding, err := provider.Embed(ctx, args.Text)
		if err != nil {
			// Log but don't fail - memory is stored, embedding can be added later
			log.Printf("cortex: warning: failed to generate embedding for memory %d: %v", id, err)
		} else {
			if err := database.AddEmbedding(ctx, id, provider.EmbedModel(), embedding); err != nil {
				log.Printf("cortex: warning: failed to store embedding for memory %d: %v", id, err)
			}
		}

		return mcp.MemoryAddResult{ID: id}, nil
	}
}

func createSearchHandler(searcher *search.HybridSearcher) mcp.Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var args mcp.MemorySearchArgs
		if err := json.Unmarshal(params, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		if args.Query == "" {
			return nil, fmt.Errorf("query is required")
		}

		// Set defaults
		k := 10
		if args.K != nil {
			k = *args.K
		}

		hybrid := true
		if args.Hybrid != nil {
			hybrid = *args.Hybrid
		}

		results, err := searcher.Search(ctx, search.SearchParams{
			Query:  args.Query,
			Limit:  k,
			Hybrid: hybrid,
		})
		if err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}

		// Convert to response format
		response := make([]mcp.MemorySearchResult, len(results))
		for i, r := range results {
			response[i] = mcp.MemorySearchResult{
				ID:         r.ID,
				Text:       r.Text,
				Score:      r.Score,
				Source:     r.Source,
				Tags:       r.Tags,
				Importance: r.Importance,
			}
		}

		return response, nil
	}
}

func createUpdateHandler(database *db.DB, provider llm.Provider) mcp.Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var args mcp.MemoryUpdateArgs
		if err := json.Unmarshal(params, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		if args.ID == 0 {
			return nil, fmt.Errorf("id is required")
		}

		updateParams := db.UpdateMemoryParams{
			Text:       args.Patch.Text,
			Kind:       args.Patch.Kind,
			Importance: args.Patch.Importance,
			Tags:       args.Patch.Tags,
			TTLDays:    args.Patch.TTLDays,
			Source:     args.Patch.Source,
		}

		if err := database.UpdateMemory(ctx, args.ID, updateParams); err != nil {
			return nil, fmt.Errorf("update memory: %w", err)
		}

		// If text was updated, regenerate embedding
		if args.Patch.Text != nil {
			embedding, err := provider.Embed(ctx, *args.Patch.Text)
			if err != nil {
				log.Printf("cortex: warning: failed to regenerate embedding for memory %d: %v", args.ID, err)
			} else {
				if err := database.AddEmbedding(ctx, args.ID, provider.EmbedModel(), embedding); err != nil {
					log.Printf("cortex: warning: failed to update embedding for memory %d: %v", args.ID, err)
				}
			}
		}

		return mcp.MemoryUpdateResult{OK: true}, nil
	}
}

func createDeleteHandler(database *db.DB) mcp.Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var args mcp.MemoryDeleteArgs
		if err := json.Unmarshal(params, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		if args.ID == 0 {
			return nil, fmt.Errorf("id is required")
		}

		if err := database.DeleteMemory(ctx, args.ID); err != nil {
			return nil, fmt.Errorf("delete memory: %w", err)
		}

		return mcp.MemoryDeleteResult{OK: true}, nil
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
