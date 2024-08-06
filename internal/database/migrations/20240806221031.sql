-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_sessions" table
CREATE TABLE `new_sessions` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `user_id` text NULL, `connection_id` text NULL, `expires_at` date NOT NULL, `last_used` date NOT NULL, `deleted` boolean NOT NULL DEFAULT false, `meta` blob NOT NULL DEFAULT '', PRIMARY KEY (`id`));
-- Copy rows from old table "sessions" to new temporary table "new_sessions"
INSERT INTO `new_sessions` (`id`, `created_at`, `updated_at`, `user_id`, `connection_id`, `expires_at`, `last_used`, `deleted`) SELECT `id`, `created_at`, `updated_at`, `user_id`, `connection_id`, `expires_at`, `last_used`, `deleted` FROM `sessions`;
-- Drop "sessions" table after copying rows
DROP TABLE `sessions`;
-- Rename temporary table "new_sessions" to "sessions"
ALTER TABLE `new_sessions` RENAME TO `sessions`;
-- Create index "idx_sessions_id_expires_at_deleted" to table: "sessions"
CREATE INDEX `idx_sessions_id_expires_at_deleted` ON `sessions` (`id`, `expires_at`, `deleted`);
-- Create index "idx_sessions_user_id_expires_at_deleted" to table: "sessions"
CREATE INDEX `idx_sessions_user_id_expires_at_deleted` ON `sessions` (`user_id`, `expires_at`, `deleted`);
-- Create index "idx_sessions_user_id_last_used_deleted" to table: "sessions"
CREATE INDEX `idx_sessions_user_id_last_used_deleted` ON `sessions` (`user_id`, `last_used`, `deleted`);
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
