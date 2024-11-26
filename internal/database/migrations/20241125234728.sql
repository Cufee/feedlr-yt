-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_users" table
CREATE TABLE `new_users` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `permissions` text NOT NULL DEFAULT '', `username` text NOT NULL, PRIMARY KEY (`id`));
-- Copy rows from old table "users" to new temporary table "new_users"
INSERT INTO `new_users` (`id`, `created_at`, `updated_at`, `username`) SELECT `id`, `created_at`, `updated_at`, `username` FROM `users`;
-- Drop "users" table after copying rows
DROP TABLE `users`;
-- Rename temporary table "new_users" to "users"
ALTER TABLE `new_users` RENAME TO `users`;
-- Create index "idx_users_username" to table: "users"
CREATE UNIQUE INDEX `idx_users_username` ON `users` (`username`);
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
