package db

import (
	"context"
	"database/sql"

	"factorymate/internal/poller"
)

func dedupePlayerState(ctx context.Context, db *sql.DB) error {
	return poller.DedupePlayerStateByName(ctx, db)
}
