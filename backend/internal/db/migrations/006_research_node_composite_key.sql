-- FRM reuses node IDs across M.A.M. trees with different names/states per tree.
-- Track state per (tree_name, node_id), not node_id alone.

CREATE TABLE research_node_state_new (
    tree_name TEXT NOT NULL,
    node_id TEXT NOT NULL,
    name TEXT NOT NULL,
    category TEXT,
    state TEXT NOT NULL,
    tech_tier INTEGER,
    cost_json TEXT NOT NULL DEFAULT '[]',
    coord_x INTEGER,
    coord_y INTEGER,
    parents_json TEXT NOT NULL DEFAULT '[]',
    updated_at TEXT NOT NULL,
    PRIMARY KEY (tree_name, node_id)
);

INSERT INTO research_node_state_new (
    tree_name, node_id, name, category, state, tech_tier, cost_json,
    coord_x, coord_y, parents_json, updated_at
)
SELECT
    tree_name, node_id, name, category, state, tech_tier, cost_json,
    coord_x, coord_y, parents_json, updated_at
FROM research_node_state;

DROP TABLE research_node_state;

ALTER TABLE research_node_state_new RENAME TO research_node_state;
