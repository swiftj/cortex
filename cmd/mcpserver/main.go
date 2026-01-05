// Package main provides the entry point for the Cortex MCP memory server.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/johnswift/cortex/internal/db"
	"github.com/johnswift/cortex/internal/llm"
	"github.com/johnswift/cortex/internal/mcp"
	"github.com/johnswift/cortex/internal/search"
	"github.com/johnswift/cortex/internal/sweeper"
	"github.com/johnswift/cortex/internal/transfer"
)

// Config holds all configuration for the Cortex server.
type Config struct {
	DatabaseURL     string
	TenantID        string
	WorkspaceID     string
	LMBackend       string
	LMModel         string
	EmbedModel      string
	EmbedModels     string // Comma-separated list for multi-model embeddings
	OpenAIKey       string
	GeminiKey       string
	SweeperEnabled  bool
	SweeperInterval time.Duration
	HealthPort      string
}

// CLI flags for export/import operations
var (
	exportFile = flag.String("export", "", "Export memories to JSONL file")
	importFile = flag.String("import", "", "Import memories from JSONL file")
	withEmbeddings = flag.Bool("with-embeddings", false, "Include embeddings in export")
	skipExisting = flag.Bool("skip-existing", false, "Skip existing records during import")
	regenerateEmbeddings = flag.Bool("regenerate-embeddings", false, "Regenerate embeddings during import")
	dryRun = flag.Bool("dry-run", false, "Validate import without writing to database")
)

func main() {
	// Configure logging to stderr (stdout reserved for MCP JSON-RPC)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	flag.Parse()

	// Check for CLI mode (export/import)
	if *exportFile != "" || *importFile != "" {
		if err := runCLI(); err != nil {
			log.Fatalf("cortex: %v", err)
		}
		return
	}

	// Run MCP server mode
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

	log.Printf("cortex: starting MCP memory server (tenant=%s, workspace=%s, backend=%s)", cfg.TenantID, cfg.WorkspaceID, cfg.LMBackend)

	// Initialize database connection
	database, err := db.NewWithWorkspace(ctx, cfg.DatabaseURL, cfg.TenantID, cfg.WorkspaceID)
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

	// Initialize multi-model embedder if EMBED_MODELS is configured
	var multiEmbedder *llm.MultiEmbedder
	if cfg.EmbedModels != "" {
		var apiKey string
		switch cfg.LMBackend {
		case "openai":
			apiKey = cfg.OpenAIKey
		case "gemini":
			apiKey = cfg.GeminiKey
		}
		multiEmbedder, err = llm.NewMultiEmbedder(cfg.LMBackend, apiKey, cfg.EmbedModels)
		if err != nil {
			return fmt.Errorf("init multi-embedder: %w", err)
		}
		log.Printf("cortex: multi-model embeddings enabled (%v)", multiEmbedder.Models())
	}

	// Initialize hybrid searcher
	searcher := search.NewHybridSearcher(database, provider)

	// Start health server if HEALTH_PORT is set
	var healthServer *mcp.HealthServer
	if cfg.HealthPort != "" {
		healthServer = mcp.NewHealthServer(cfg.HealthPort)
		if err := healthServer.Start(); err != nil {
			return fmt.Errorf("start health server: %w", err)
		}
		defer healthServer.Shutdown(ctx)
	}

	// Start TTL sweeper if enabled
	var sw *sweeper.Sweeper
	if cfg.SweeperEnabled {
		sw = sweeper.NewSweeper(database.Pool(), cfg.TenantID, cfg.WorkspaceID)
		sw.Start(ctx, cfg.SweeperInterval)
		log.Printf("cortex: TTL sweeper enabled (interval=%v)", cfg.SweeperInterval)
	}

	// Create MCP server
	server := mcp.NewServer("cortex", "1.0.0")

	// Register memory tools
	registerMemoryTools(server, database, provider, multiEmbedder, searcher)

	// Run the MCP server (blocks until context is cancelled)
	log.Println("cortex: MCP server ready, listening on stdio")
	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	// Stop sweeper gracefully
	if sw != nil {
		sw.Stop()
	}

	log.Println("cortex: shutting down gracefully")
	return nil
}

func loadConfig() (*Config, error) {
	// Parse sweeper interval
	sweeperIntervalStr := getEnv("SWEEPER_INTERVAL", "1h")
	sweeperInterval, err := time.ParseDuration(sweeperIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SWEEPER_INTERVAL: %w", err)
	}

	// Parse sweeper enabled (default: true)
	sweeperEnabled := true
	if v := getEnv("SWEEPER_ENABLED", "true"); v == "false" || v == "0" {
		sweeperEnabled = false
	}

	cfg := &Config{
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		TenantID:        getEnv("TENANT_ID", "local"),
		WorkspaceID:     getEnv("WORKSPACE_ID", "default"),
		LMBackend:       getEnv("LM_BACKEND", "openai"),
		LMModel:         getEnv("LM_MODEL", "auto"),
		EmbedModel:      getEnv("EMBED_MODEL", "auto"),
		EmbedModels:     getEnv("EMBED_MODELS", ""), // Comma-separated for multi-model
		OpenAIKey:       getEnv("OPENAI_API_KEY", ""),
		GeminiKey:       getEnv("GEMINI_API_KEY", ""),
		SweeperEnabled:  sweeperEnabled,
		SweeperInterval: sweeperInterval,
		HealthPort:      getEnv("HEALTH_PORT", ""),
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

func registerMemoryTools(server *mcp.Server, database *db.DB, provider llm.Provider, multiEmbedder *llm.MultiEmbedder, searcher *search.HybridSearcher) {
	// Register all memory tools with their handlers
	server.RegisterTool(mcp.MemoryAddTool(), createAddHandler(database, provider, multiEmbedder))
	server.RegisterTool(mcp.MemorySearchTool(), createSearchHandler(searcher))
	server.RegisterTool(mcp.MemoryUpdateTool(), createUpdateHandler(database, provider, multiEmbedder))
	server.RegisterTool(mcp.MemoryDeleteTool(), createDeleteHandler(database))
	server.RegisterTool(mcp.MemoryExportTool(), createExportHandler(database))
	server.RegisterTool(mcp.MemoryImportTool(), createImportHandler(database, provider))
}

func createAddHandler(database *db.DB, provider llm.Provider, multiEmbedder *llm.MultiEmbedder) mcp.Handler {
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

		// Generate and store embeddings
		if multiEmbedder != nil {
			// Multi-model: generate embeddings from all configured models
			embeddings, err := multiEmbedder.EmbedAll(ctx, args.Text)
			if err != nil {
				log.Printf("cortex: warning: failed to generate multi-model embeddings for memory %d: %v", id, err)
			} else {
				for model, embedding := range embeddings {
					if err := database.AddEmbedding(ctx, id, model, embedding); err != nil {
						log.Printf("cortex: warning: failed to store embedding (model=%s) for memory %d: %v", model, id, err)
					}
				}
			}
		} else {
			// Single-model: use the default provider
			embedding, err := provider.Embed(ctx, args.Text)
			if err != nil {
				log.Printf("cortex: warning: failed to generate embedding for memory %d: %v", id, err)
			} else {
				if err := database.AddEmbedding(ctx, id, provider.EmbedModel(), embedding); err != nil {
					log.Printf("cortex: warning: failed to store embedding for memory %d: %v", id, err)
				}
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

		// Get model filter if specified
		model := ""
		if args.Model != nil {
			model = *args.Model
		}

		results, err := searcher.Search(ctx, search.SearchParams{
			Query:  args.Query,
			Limit:  k,
			Hybrid: hybrid,
			Model:  model,
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

func createUpdateHandler(database *db.DB, provider llm.Provider, multiEmbedder *llm.MultiEmbedder) mcp.Handler {
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

		// If text was updated, regenerate embeddings
		if args.Patch.Text != nil {
			if multiEmbedder != nil {
				// Multi-model: regenerate all embeddings
				embeddings, err := multiEmbedder.EmbedAll(ctx, *args.Patch.Text)
				if err != nil {
					log.Printf("cortex: warning: failed to regenerate multi-model embeddings for memory %d: %v", args.ID, err)
				} else {
					for model, embedding := range embeddings {
						if err := database.AddEmbedding(ctx, args.ID, model, embedding); err != nil {
							log.Printf("cortex: warning: failed to update embedding (model=%s) for memory %d: %v", model, args.ID, err)
						}
					}
				}
			} else {
				embedding, err := provider.Embed(ctx, *args.Patch.Text)
				if err != nil {
					log.Printf("cortex: warning: failed to regenerate embedding for memory %d: %v", args.ID, err)
				} else {
					if err := database.AddEmbedding(ctx, args.ID, provider.EmbedModel(), embedding); err != nil {
						log.Printf("cortex: warning: failed to update embedding for memory %d: %v", args.ID, err)
					}
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

// runCLI handles CLI mode for export/import operations
func runCLI() error {
	cfg, err := loadConfigForCLI()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()

	// Connect to database
	database, err := db.NewWithWorkspace(ctx, cfg.DatabaseURL, cfg.TenantID, cfg.WorkspaceID)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer database.Close()

	// Run migrations
	if err := database.Migrate(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Handle export
	if *exportFile != "" {
		return runExport(ctx, database)
	}

	// Handle import
	if *importFile != "" {
		// Initialize LLM provider if regenerating embeddings
		var provider llm.Provider
		if *regenerateEmbeddings {
			provider, err = initLLMProvider(cfg)
			if err != nil {
				return fmt.Errorf("init LLM provider: %w", err)
			}
		}
		return runImport(ctx, database, provider)
	}

	return nil
}

func loadConfigForCLI() (*Config, error) {
	cfg := &Config{
		DatabaseURL: getEnv("DATABASE_URL", ""),
		TenantID:    getEnv("TENANT_ID", "local"),
		WorkspaceID: getEnv("WORKSPACE_ID", "default"),
		LMBackend:   getEnv("LM_BACKEND", "openai"),
		LMModel:     getEnv("LM_MODEL", "auto"),
		EmbedModel:  getEnv("EMBED_MODEL", "auto"),
		OpenAIKey:   getEnv("OPENAI_API_KEY", ""),
		GeminiKey:   getEnv("GEMINI_API_KEY", ""),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	// Only require API key if regenerating embeddings
	if *regenerateEmbeddings {
		switch cfg.LMBackend {
		case "openai":
			if cfg.OpenAIKey == "" {
				return nil, fmt.Errorf("OPENAI_API_KEY required when using --regenerate-embeddings")
			}
		case "gemini":
			if cfg.GeminiKey == "" {
				return nil, fmt.Errorf("GEMINI_API_KEY required when using --regenerate-embeddings")
			}
		}
	}

	return cfg, nil
}

func runExport(ctx context.Context, database *db.DB) error {
	log.Printf("cortex: exporting memories to %s", *exportFile)

	exporter := transfer.NewExporter(database.Pool())

	opts := transfer.ExportOptions{
		IncludeEmbeddings: *withEmbeddings,
		TenantID:          database.TenantID(),
		WorkspaceID:       database.WorkspaceID(),
	}

	// Create output file
	f, err := os.Create(*exportFile)
	if err != nil {
		return fmt.Errorf("create export file: %w", err)
	}
	defer f.Close()

	result, err := exporter.Export(ctx, f, opts)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	log.Printf("cortex: exported %d memories (%d errors)", result.Exported, result.Errors)
	return nil
}

func runImport(ctx context.Context, database *db.DB, provider llm.Provider) error {
	log.Printf("cortex: importing memories from %s", *importFile)

	// Create embedder wrapper if provider is available
	var embedder transfer.EmbeddingProvider
	if provider != nil {
		embedder = &providerWrapper{provider}
	}

	importer := transfer.NewImporter(database.Pool(), embedder)

	opts := transfer.ImportOptions{
		SkipExisting:         *skipExisting,
		RegenerateEmbeddings: *regenerateEmbeddings,
		OverrideTenantID:     database.TenantID(),
		OverrideWorkspaceID:  database.WorkspaceID(),
		DryRun:               *dryRun,
	}

	// Open input file
	f, err := os.Open(*importFile)
	if err != nil {
		return fmt.Errorf("open import file: %w", err)
	}
	defer f.Close()

	result, err := importer.Import(ctx, f, opts)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	if *dryRun {
		log.Printf("cortex: (dry run) would import %d memories (%d skipped, %d errors)",
			result.Imported, result.Skipped, result.Errors)
	} else {
		log.Printf("cortex: imported %d memories (%d skipped, %d errors)",
			result.Imported, result.Skipped, result.Errors)
	}
	return nil
}

// providerWrapper wraps llm.Provider to implement transfer.EmbeddingProvider
type providerWrapper struct {
	provider llm.Provider
}

func (p *providerWrapper) Embed(ctx context.Context, text string) ([]float32, error) {
	return p.provider.Embed(ctx, text)
}

func (p *providerWrapper) EmbedModel() string {
	return p.provider.EmbedModel()
}

func (p *providerWrapper) Dimensions() int {
	return p.provider.Dimensions()
}

// MCP tool handlers for export/import

func createExportHandler(database *db.DB) mcp.Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var args mcp.MemoryExportArgs
		if err := json.Unmarshal(params, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		exporter := transfer.NewExporter(database.Pool())

		opts := transfer.ExportOptions{
			IncludeEmbeddings: args.IncludeEmbeddings,
			TenantID:          database.TenantID(),
			WorkspaceID:       database.WorkspaceID(),
			Kind:              args.Kind,
			Limit:             args.Limit,
		}

		var buf bytes.Buffer
		result, err := exporter.Export(ctx, &buf, opts)
		if err != nil {
			return nil, fmt.Errorf("export: %w", err)
		}

		return mcp.MemoryExportResult{
			Data:     buf.String(),
			Exported: result.Exported,
			Errors:   result.Errors,
		}, nil
	}
}

func createImportHandler(database *db.DB, provider llm.Provider) mcp.Handler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var args mcp.MemoryImportArgs
		if err := json.Unmarshal(params, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}

		if args.Data == "" {
			return nil, fmt.Errorf("data is required")
		}

		// Create embedder wrapper
		var embedder transfer.EmbeddingProvider
		if provider != nil {
			embedder = &providerWrapper{provider}
		}

		importer := transfer.NewImporter(database.Pool(), embedder)

		opts := transfer.ImportOptions{
			SkipExisting:         args.SkipExisting,
			RegenerateEmbeddings: args.RegenerateEmbeddings,
			OverrideTenantID:     database.TenantID(),
			OverrideWorkspaceID:  database.WorkspaceID(),
			DryRun:               args.DryRun,
		}

		reader := strings.NewReader(args.Data)
		result, err := importer.Import(ctx, io.NopCloser(reader), opts)
		if err != nil {
			return nil, fmt.Errorf("import: %w", err)
		}

		return mcp.MemoryImportResult{
			Total:    result.Total,
			Imported: result.Imported,
			Skipped:  result.Skipped,
			Errors:   result.Errors,
		}, nil
	}
}
