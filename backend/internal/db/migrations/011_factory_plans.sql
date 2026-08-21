CREATE TABLE factory_plans (
    id INTEGER PRIMARY KEY,
    owner_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'private'
        CHECK (visibility IN ('private', 'shared')),
    status TEXT NOT NULL DEFAULT 'planning'
        CHECK (status IN ('planning', 'in_progress', 'completed', 'archived')),
    target_item_class TEXT,
    target_rate REAL,
    solver_options_json TEXT NOT NULL DEFAULT '{}',
    graph_json TEXT NOT NULL,
    baseline_json TEXT,
    locked_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    lock_expires_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_factory_plans_owner ON factory_plans(owner_user_id);
CREATE INDEX idx_factory_plans_visibility ON factory_plans(visibility);
CREATE INDEX idx_factory_plans_status ON factory_plans(status);
