package auth

import (
	"context"
	"fmt"
	"log"
	"time"
)

func (s *Service) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return res.RowsAffected()
}

func (s *Service) StartCleanupJob(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := s.CleanupExpiredSessions(ctx)
				if err != nil {
					log.Printf("session cleanup: %v", err)
					continue
				}
				if n > 0 {
					log.Printf("session cleanup: removed %d expired sessions", n)
				}
			}
		}
	}()
}
