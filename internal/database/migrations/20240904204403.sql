-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_videos" table
CREATE TABLE `new_videos` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `title` text NOT NULL, `description` text NOT NULL, `duration` integer NOT NULL, `published_at` date NOT NULL, `private` boolean NOT NULL, `type` text NOT NULL, `channel_id` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `videos_channel_id_fkey` FOREIGN KEY (`channel_id`) REFERENCES `channels` (`id`) ON DELETE CASCADE);
-- Copy rows from old table "videos" to new temporary table "new_videos"
INSERT INTO `new_videos` (`id`, `created_at`, `updated_at`, `title`, `description`, `duration`, `published_at`, `private`, `type`, `channel_id`) SELECT `id`, `created_at`, `updated_at`, `title`, `description`, `duration`, `published_at`, `private`, `type`, `channel_id` FROM `videos`;
-- Drop "videos" table after copying rows
DROP TABLE `videos`;
-- Rename temporary table "new_videos" to "videos"
ALTER TABLE `new_videos` RENAME TO `videos`;
-- Create index "idx_videos_published_at" to table: "videos"
CREATE INDEX `idx_videos_published_at` ON `videos` (`published_at`);
-- Create index "idx_videos_channel_id" to table: "videos"
CREATE INDEX `idx_videos_channel_id` ON `videos` (`channel_id`);
-- Create index "idx_videos_published_at_channel_id" to table: "videos"
CREATE INDEX `idx_videos_published_at_channel_id` ON `videos` (`published_at`, `channel_id`);
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
