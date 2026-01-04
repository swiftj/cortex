// Package sweeper provides automatic cleanup of expired memories based on TTL.
package sweeper

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Sweeper manages automatic deletion of expired memories.
type Sweeper struct {
	pool        *pgxpool.Pool
	tenantID    string
	workspaceID string

	mu      sync.Mutex
	running bool
	done    chan struct{}
}

// NewSweeper creates a new Sweeper instance.
func NewSweeper(pool *pgxpool.Pool, tenantID, workspaceID string) *Sweeper {
	return &Sweeper{
		pool:        pool,
		tenantID:    tenantID,
		workspaceID: workspaceID,
	}
}

// Start begins the sweeper goroutine that periodically deletes expired memories.
func (s *Sweeper) Start(ctx context.Context, interval time.Duration) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		log.Printf("[sweeper] already running")
		return
	}
	s.running = true
	s.done = make(chan struct{})
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer func() {
			s.mu.Lock()
			s.running = false
			close(s.done)
			s.mu.Unlock()
		}()

		log.Printf("[sweeper] started with interval %v", interval)

		// Run initial sweep
		s.runSweep(ctx)

		for {
			select {
			case <-ctx.Done():
				log.Printf("[sweeper] context cancelled, stopping")
				return
			case <-ticker.C:
				s.runSweep(ctx)
			}
		}
	}()
}

// Stop gracefully stops the sweeper and waits for it to complete.
func (s *Sweeper) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	done := s.done
	s.mu.Unlock()

	<-done
	log.Printf("[sweeper] stopped")
}

func (s *Sweeper) runSweep(ctx context.Context) {
	deleted, err := s.DeleteExpired(ctx)
	if err != nil {
		log.Printf("[sweeper] error deleting expired memories: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("[sweeper] deleted %d expired memories", deleted)
	}
}

// DeleteExpired removes all memories that have exceeded their TTL.
// A memory expires when: created_at + (ttl_days * 1 day) < NOW()
func (s *Sweeper) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := s.pool.Exec(ctx, `
		DELETE FROM memories
		WHERE tenant_id = $1
		  AND workspace_id = $2
		  AND ttl_days IS NOT NULL
		  AND created_at + ttl_days * INTERVAL '1 day' < NOW()
	`, s.tenantID, s.workspaceID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
