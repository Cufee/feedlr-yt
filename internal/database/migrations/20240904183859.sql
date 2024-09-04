-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_channels" table
CREATE TABLE `new_channels` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `title` text NOT NULL, `description` text NOT NULL, `thumbnail` text NOT NULL, `uploads_playlist_id` text NOT NULL DEFAULT '', PRIMARY KEY (`id`));
-- Copy rows from old table "channels" to new temporary table "new_channels"
INSERT INTO `new_channels` (`id`, `created_at`, `updated_at`, `title`, `description`, `thumbnail`) SELECT `id`, `created_at`, `updated_at`, `title`, `description`, `thumbnail` FROM `channels`;
-- Drop "channels" table after copying rows
DROP TABLE `channels`;
-- Rename temporary table "new_channels" to "channels"
ALTER TABLE `new_channels` RENAME TO `channels`;
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
