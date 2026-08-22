-- FRM safe reconnect: recovery phase tracking + grace period setting (§4.2).

ALTER TABLE server_state ADD COLUMN recovery_phase TEXT NOT NULL DEFAULT 'healthy';

ALTER TABLE app_settings ADD COLUMN frm_recovery_grace_seconds INTEGER NOT NULL DEFAULT 60;
