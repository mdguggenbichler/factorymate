-- M16: per-user DM notification preferences (discord-bot-plan §12.2)

CREATE TABLE user_notification_prefs (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category TEXT NOT NULL
        CHECK (category IN ('server', 'player', 'power', 'progression', 'vehicle')),
    dm_enabled BOOLEAN NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (user_id, category)
);

INSERT INTO app_setting_kv (key, value) VALUES
    ('notifications.dm_defaults_json', '{"server":false,"player":false,"power":false,"progression":false,"vehicle":false}'),
    ('notifications.dm_player_personal_default', 'false');
