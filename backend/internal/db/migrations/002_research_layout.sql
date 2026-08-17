ALTER TABLE research_node_state ADD COLUMN coord_x INTEGER;
ALTER TABLE research_node_state ADD COLUMN coord_y INTEGER;
ALTER TABLE research_node_state ADD COLUMN parents_json TEXT NOT NULL DEFAULT '[]';
