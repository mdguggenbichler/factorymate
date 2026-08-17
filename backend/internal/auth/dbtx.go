package auth

import (
	"context"
	"database/sql"
)

// DBTX is satisfied by *sql.DB and *sql.Tx for transactional auto-link.
type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
