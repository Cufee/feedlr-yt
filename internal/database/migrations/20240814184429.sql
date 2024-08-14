-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_passkeys" table
CREATE TABLE `new_passkeys` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `label` text NOT NULL DEFAULT '', `data` blob NOT NULL, `user_id` text NOT NULL, PRIMARY KEY (`id`));
-- Copy rows from old table "passkeys" to new temporary table "new_passkeys"
INSERT INTO `new_passkeys` (`id`, `created_at`, `updated_at`, `data`, `user_id`) SELECT `id`, `created_at`, `updated_at`, `data`, `user_id` FROM `passkeys`;
-- Drop "passkeys" table after copying rows
DROP TABLE `passkeys`;
-- Rename temporary table "new_passkeys" to "passkeys"
ALTER TABLE `new_passkeys` RENAME TO `passkeys`;
-- Create index "idx_passkeys_user_id" to table: "passkeys"
CREATE INDEX `idx_passkeys_user_id` ON `passkeys` (`user_id`);
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
