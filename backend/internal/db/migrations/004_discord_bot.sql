-- M15 Phase 1: Discord bot foundation (discord-bot-plan §12)

-- Generic external identity (portable to Slack etc.)
ALTER TABLE users ADD COLUMN external_platform TEXT
    CHECK (external_platform IS NULL OR external_platform IN ('discord', 'slack'));
ALTER TABLE users ADD COLUMN external_user_id TEXT;
ALTER TABLE users ADD COLUMN external_username TEXT;
ALTER TABLE users ADD COLUMN external_display_name TEXT;
ALTER TABLE users ADD COLUMN external_linked_at TEXT;
CREATE UNIQUE INDEX idx_users_external_identity
    ON users(external_platform, external_user_id)
    WHERE external_user_id IS NOT NULL;

ALTER TABLE users ADD COLUMN pending_player_name TEXT;
ALTER TABLE users ADD COLUMN registration_source TEXT NOT NULL DEFAULT 'web_invite'
    CHECK (registration_source IN ('setup', 'web_invite', 'discord'));
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('pending_approval', 'active'));
ALTER TABLE users ADD COLUMN dm_player_personal BOOLEAN NOT NULL DEFAULT 0;

CREATE TABLE registration_audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    external_user_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('submitted', 'approved', 'rejected')),
    acted_by_user_id INTEGER REFERENCES users(id),
    comment TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE bot_command_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    external_platform TEXT NOT NULL DEFAULT 'discord',
    external_user_id TEXT NOT NULL,
    command_name TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    detail TEXT,
    created_at TEXT NOT NULL
);

-- Extend notification_log for DM deliveries (SQLite table rebuild, §12.0 / §12.5)
CREATE TABLE notification_log_new (
    id INTEGER PRIMARY KEY,
    message_type_key TEXT NOT NULL,
    target_id INTEGER,
    rendered_preview TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    error TEXT,
    sent_at TEXT NOT NULL,
    delivery_mode TEXT NOT NULL DEFAULT 'channel'
        CHECK (delivery_mode IN ('channel', 'dm')),
    recipient_external_user_id TEXT
);

INSERT INTO notification_log_new (
    id, message_type_key, target_id, rendered_preview, success, error, sent_at,
    delivery_mode, recipient_external_user_id
)
SELECT
    id, message_type_key, target_id, rendered_preview, success, error, sent_at,
    'channel', NULL
FROM notification_log;

DROP TABLE notification_log;
ALTER TABLE notification_log_new RENAME TO notification_log;

-- Namespaced app settings (§12.4)
CREATE TABLE app_setting_kv (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO app_setting_kv (key, value) VALUES
    ('discord.bot_enabled', 'true'),
    ('discord.guild_id', ''),
    ('discord.role_mappings_json', '{}'),
    ('registration.auto_approve', 'true'),
    ('connection.details_json', '{}'),
    ('mods.smm_profile_name', 'FactoryMate Server');
