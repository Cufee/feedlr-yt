-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_views" table
CREATE TABLE `new_views` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `user_id` text NOT NULL, `video_id` text NOT NULL, `progress` integer NOT NULL, `hidden` boolean NULL DEFAULT false, PRIMARY KEY (`id`), CONSTRAINT `views_user_id_fkey` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE, CONSTRAINT `views_video_id_fkey` FOREIGN KEY (`video_id`) REFERENCES `videos` (`id`));
-- Copy rows from old table "views" to new temporary table "new_views"
INSERT INTO `new_views` (`id`, `created_at`, `updated_at`, `user_id`, `video_id`, `progress`, `hidden`) SELECT `id`, `created_at`, `updated_at`, `user_id`, `video_id`, `progress`, `hidden` FROM `views`;
-- Drop "views" table after copying rows
DROP TABLE `views`;
-- Rename temporary table "new_views" to "views"
ALTER TABLE `new_views` RENAME TO `views`;
-- Create index "idx_views_user_id" to table: "views"
CREATE INDEX `idx_views_user_id` ON `views` (`user_id`);
-- Create index "idx_views_user_id_hidden" to table: "views"
CREATE INDEX `idx_views_user_id_hidden` ON `views` (`user_id`, `hidden`);
-- Create index "idx_views_video_id_user_id" to table: "views"
CREATE UNIQUE INDEX `idx_views_video_id_user_id` ON `views` (`video_id`, `user_id`);
-- Create index "idx_views_video_id_user_id_hidden" to table: "views"
CREATE INDEX `idx_views_video_id_user_id_hidden` ON `views` (`video_id`, `user_id`, `hidden`);
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
