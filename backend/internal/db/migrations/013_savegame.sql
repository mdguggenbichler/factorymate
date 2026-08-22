-- M22: Dedicated Server HTTPS API settings + savegame download audit/rate limit

ALTER TABLE app_settings ADD COLUMN game_api_host TEXT NOT NULL DEFAULT '';
ALTER TABLE app_settings ADD COLUMN game_api_port INTEGER NOT NULL DEFAULT 7777;
ALTER TABLE app_settings ADD COLUMN game_api_token TEXT;

CREATE TABLE savegame_download_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id),
    channel TEXT NOT NULL,
    save_name TEXT NOT NULL,
    bytes INTEGER NOT NULL,
    downloaded_at TEXT NOT NULL
);
CREATE INDEX idx_savegame_download_user_time ON savegame_download_log (user_id, downloaded_at);
