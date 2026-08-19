-- M17: Discord OAuth SSO — nullable password_hash for Discord-only users; oauth_states for CSRF.

-- Discord-origin users have no FactoryMate password; setup/invite users keep password_hash.
-- SQLite cannot ALTER COLUMN; recreate users with nullable password_hash.
CREATE TABLE users_new (
    id INTEGER PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    role TEXT NOT NULL CHECK (role IN ('admin', 'viewer')),
    player_id TEXT REFERENCES player_state(player_id),
    created_at TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('pending_approval', 'active')),
    external_platform TEXT
        CHECK (external_platform IS NULL OR external_platform IN ('discord', 'slack')),
    external_user_id TEXT,
    external_username TEXT,
    external_display_name TEXT,
    external_linked_at TEXT,
    pending_player_name TEXT,
    registration_source TEXT NOT NULL DEFAULT 'web_invite'
        CHECK (registration_source IN ('setup', 'web_invite', 'discord')),
    dm_player_personal BOOLEAN NOT NULL DEFAULT 0
);

INSERT INTO users_new (
    id, username, password_hash, role, player_id, created_at, status,
    external_platform, external_user_id, external_username, external_display_name,
    external_linked_at, pending_player_name, registration_source, dm_player_personal
)
SELECT
    id, username, password_hash, role, player_id, created_at, status,
    external_platform, external_user_id, external_username, external_display_name,
    external_linked_at, pending_player_name, registration_source, dm_player_personal
FROM users;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

CREATE UNIQUE INDEX idx_users_external_identity
    ON users(external_platform, external_user_id)
    WHERE external_user_id IS NOT NULL;
CREATE INDEX idx_users_player_id ON users(player_id);

-- Single-use OAuth state tokens (SHA-256 of random nonce), 10-minute TTL.
CREATE TABLE oauth_states (
    token_hash TEXT PRIMARY KEY,
    purpose TEXT NOT NULL CHECK (purpose IN ('login', 'register', 'link', 'register_complete')),
    external_user_id TEXT,
    external_username TEXT,
    external_display_name TEXT,
    force_approve INTEGER NOT NULL DEFAULT 0,
    fm_role TEXT,
    user_id INTEGER REFERENCES users(id),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at TEXT
);
CREATE INDEX idx_oauth_states_expires ON oauth_states(expires_at);
