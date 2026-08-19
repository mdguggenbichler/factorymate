-- M18: rebuild user_notification_prefs to per message_type_key; expand category DM defaults.

CREATE TABLE user_notification_prefs_new (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_type_key TEXT NOT NULL REFERENCES message_types(key),
    dm_enabled BOOLEAN NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (user_id, message_type_key)
);

INSERT INTO user_notification_prefs_new (user_id, message_type_key, dm_enabled, updated_at)
SELECT unp.user_id, mt.key, unp.dm_enabled, unp.updated_at
FROM user_notification_prefs unp
JOIN message_types mt ON mt.category = unp.category
WHERE mt.key NOT IN ('connection_details', 'connection_details_changed');

DROP TABLE user_notification_prefs;
ALTER TABLE user_notification_prefs_new RENAME TO user_notification_prefs;

UPDATE app_setting_kv
SET value = (
    SELECT COALESCE(
        (
            SELECT json_group_object(
                mt.key,
                json(CASE WHEN COALESCE(json_extract(kv.value, '$.' || mt.category), 0) THEN 'true' ELSE 'false' END)
            )
            FROM message_types mt
            WHERE mt.key NOT IN ('connection_details', 'connection_details_changed')
        ),
        '{}'
    )
    FROM app_setting_kv kv
    WHERE kv.key = 'notifications.dm_defaults_json'
)
WHERE key = 'notifications.dm_defaults_json';
