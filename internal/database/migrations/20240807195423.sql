-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Drop "connections" table
DROP TABLE `connections`;
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
