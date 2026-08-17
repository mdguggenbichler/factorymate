CREATE TABLE invites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'viewer')),
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    accepted_at TEXT,
    accepted_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    revoked_at TEXT
);

CREATE INDEX idx_invites_token ON invites(token);
CREATE INDEX idx_invites_pending ON invites(accepted_at, revoked_at, expires_at);

ALTER TABLE users ADD COLUMN player_id TEXT REFERENCES player_state(player_id);
CREATE INDEX idx_users_player_id ON users(player_id);
