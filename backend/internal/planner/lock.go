package planner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const LockTTL = 5 * time.Minute

var (
	ErrLockHeld      = errors.New("edit lock held by another user")
	ErrLockRequired  = errors.New("edit lock required")
	ErrPlanArchived  = errors.New("plan is archived")
	ErrNoBaseline    = errors.New("no solver baseline")
	ErrPlanNotFound  = errors.New("plan not found")
	ErrUpdatedAtMismatch = errors.New("plan was modified")
)

// LockState is the lock summary returned by list/detail APIs.
type LockState struct {
	Held      bool   `json:"held"`
	UserID    int64  `json:"userId,omitempty"`
	Username  string `json:"username,omitempty"`
	ExpiresAt string `json:"expiresAt,omitempty"`
	Mine      bool   `json:"mine"`
}

// ClearExpiredLock clears an expired lock on a plan if present.
func ClearExpiredLock(ctx context.Context, db *sql.DB, planID int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `
		UPDATE factory_plans
		SET locked_by_user_id = NULL, lock_expires_at = NULL
		WHERE id = ? AND lock_expires_at IS NOT NULL AND lock_expires_at < ?`,
		planID, now,
	)
	return err
}

// LoadLockState returns the current lock summary for a plan.
func LoadLockState(ctx context.Context, db *sql.DB, planID, viewerUserID int64) (LockState, error) {
	if err := ClearExpiredLock(ctx, db, planID); err != nil {
		return LockState{}, err
	}

	var lockedBy sql.NullInt64
	var expiresAt sql.NullString
	var username sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT fp.locked_by_user_id, fp.lock_expires_at, u.username
		FROM factory_plans fp
		LEFT JOIN users u ON u.id = fp.locked_by_user_id
		WHERE fp.id = ?`,
		planID,
	).Scan(&lockedBy, &expiresAt, &username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LockState{}, ErrPlanNotFound
		}
		return LockState{}, err
	}

	if !lockedBy.Valid {
		return LockState{Held: false, Mine: false}, nil
	}

	state := LockState{
		Held:   true,
		UserID: lockedBy.Int64,
		Mine:   lockedBy.Int64 == viewerUserID,
	}
	if expiresAt.Valid {
		state.ExpiresAt = expiresAt.String
	}
	if username.Valid {
		state.Username = username.String
	}
	return state, nil
}

// AcquireLock grants the edit lock to userID when the plan is editable and the lock is free or expired.
func AcquireLock(ctx context.Context, db *sql.DB, planID, userID int64) error {
	if err := ClearExpiredLock(ctx, db, planID); err != nil {
		return err
	}

	var status string
	var lockedBy sql.NullInt64
	var expiresAt sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT status, locked_by_user_id, lock_expires_at
		FROM factory_plans WHERE id = ?`,
		planID,
	).Scan(&status, &lockedBy, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPlanNotFound
		}
		return err
	}
	if status == "archived" {
		return ErrPlanArchived
	}

	now := time.Now().UTC()
	if lockedBy.Valid && lockedBy.Int64 != userID {
		if expiresAt.Valid {
			if t, parseErr := time.Parse(time.RFC3339, expiresAt.String); parseErr == nil && t.After(now) {
				return ErrLockHeld
			}
		} else {
			return ErrLockHeld
		}
	}

	newExpiry := now.Add(LockTTL).Format(time.RFC3339)
	updatedAt := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		UPDATE factory_plans
		SET locked_by_user_id = ?, lock_expires_at = ?, updated_at = ?
		WHERE id = ?`,
		userID, newExpiry, updatedAt, planID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPlanNotFound
	}
	return nil
}

// HeartbeatLock extends the lock expiry for the current holder.
func HeartbeatLock(ctx context.Context, db *sql.DB, planID, userID int64) error {
	if err := ClearExpiredLock(ctx, db, planID); err != nil {
		return err
	}

	var status string
	var lockedBy sql.NullInt64
	err := db.QueryRowContext(ctx, `
		SELECT status, locked_by_user_id FROM factory_plans WHERE id = ?`,
		planID,
	).Scan(&status, &lockedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPlanNotFound
		}
		return err
	}
	if status == "archived" {
		return ErrPlanArchived
	}
	if !lockedBy.Valid || lockedBy.Int64 != userID {
		return ErrLockRequired
	}

	newExpiry := time.Now().UTC().Add(LockTTL).Format(time.RFC3339)
	_, err = db.ExecContext(ctx, `
		UPDATE factory_plans SET lock_expires_at = ? WHERE id = ? AND locked_by_user_id = ?`,
		newExpiry, planID, userID,
	)
	return err
}

// ReleaseLock clears the lock when held by userID.
func ReleaseLock(ctx context.Context, db *sql.DB, planID, userID int64) error {
	res, err := db.ExecContext(ctx, `
		UPDATE factory_plans
		SET locked_by_user_id = NULL, lock_expires_at = NULL
		WHERE id = ? AND locked_by_user_id = ?`,
		planID, userID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		if err := ClearExpiredLock(ctx, db, planID); err != nil {
			return err
		}
		return ErrLockRequired
	}
	return nil
}

// ForceReleaseLock clears the lock when actor is the plan owner or an admin.
func ForceReleaseLock(ctx context.Context, db *sql.DB, planID, actorUserID int64, actorIsAdmin bool) error {
	var ownerID int64
	err := db.QueryRowContext(ctx, `SELECT owner_user_id FROM factory_plans WHERE id = ?`, planID).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPlanNotFound
		}
		return err
	}
	if !actorIsAdmin && ownerID != actorUserID {
		return fmt.Errorf("forbidden")
	}

	_, err = db.ExecContext(ctx, `
		UPDATE factory_plans
		SET locked_by_user_id = NULL, lock_expires_at = NULL
		WHERE id = ?`,
		planID,
	)
	return err
}

// RequireLockHolder verifies userID holds a live lock on a non-archived plan.
func RequireLockHolder(ctx context.Context, db *sql.DB, planID, userID int64) error {
	if err := ClearExpiredLock(ctx, db, planID); err != nil {
		return err
	}

	var status string
	var lockedBy sql.NullInt64
	err := db.QueryRowContext(ctx, `
		SELECT status, locked_by_user_id FROM factory_plans WHERE id = ?`,
		planID,
	).Scan(&status, &lockedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPlanNotFound
		}
		return err
	}
	if status == "archived" {
		return ErrPlanArchived
	}
	if !lockedBy.Valid || lockedBy.Int64 != userID {
		return ErrLockRequired
	}
	return nil
}

// ClearPlanLock removes any lock (e.g. when archiving).
func ClearPlanLock(ctx context.Context, tx *sql.Tx, planID int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE factory_plans
		SET locked_by_user_id = NULL, lock_expires_at = NULL
		WHERE id = ?`,
		planID,
	)
	return err
}
