package pgstore

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// DefaultCleanupInterval is the interval Cleanup uses when given none.
const DefaultCleanupInterval = 5 * time.Minute

// DeleteExpired deletes the rows of expired sessions and returns how many it
// deleted.
func (s *Store) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM http_sessions WHERE expires_on < now()")
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to count deleted sessions: %w", err)
	}

	return n, nil
}

// Cleanup calls DeleteExpired every interval (DefaultCleanupInterval if
// interval is not positive) until ctx is done, logging failures to log. It
// blocks; run it in its own goroutine.
func (s *Store) Cleanup(ctx context.Context, interval time.Duration, log *zap.Logger) {
	if interval <= 0 {
		interval = DefaultCleanupInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.DeleteExpired(ctx); err != nil && ctx.Err() == nil {
				log.Error("pgstore: failed to delete expired sessions", zap.Error(err))
			}
		}
	}
}
