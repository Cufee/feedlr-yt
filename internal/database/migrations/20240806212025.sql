-- Create "auth_nonces" table
CREATE TABLE `auth_nonces` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `used` boolean NOT NULL, `expires_at` date NOT NULL, `value` text NOT NULL, PRIMARY KEY (`id`));
-- Create index "idx_auth_nonces_expires_at_used" to table: "auth_nonces"
CREATE INDEX `idx_auth_nonces_expires_at_used` ON `auth_nonces` (`id`, `expires_at`, `used`);
-- Create "channels" table
CREATE TABLE `channels` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `title` text NOT NULL, `description` text NOT NULL, `thumbnail` text NOT NULL, PRIMARY KEY (`id`));
-- Create "videos" table
CREATE TABLE `videos` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `title` text NOT NULL, `description` text NOT NULL, `duration` integer NOT NULL, `published_at` date NOT NULL, `private` boolean NOT NULL, `type` text NOT NULL, `channel_id` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `videos_channel_id_fkey` FOREIGN KEY (`channel_id`) REFERENCES `channels` (`id`) ON DELETE CASCADE);
-- Create index "idx_videos_published_at" to table: "videos"
CREATE INDEX `idx_videos_published_at` ON `videos` (`published_at`);
-- Create index "idx_videos_channel_id" to table: "videos"
CREATE INDEX `idx_videos_channel_id` ON `videos` (`channel_id`);
-- Create index "idx_videos_published_at_channel_id" to table: "videos"
CREATE INDEX `idx_videos_published_at_channel_id` ON `videos` (`published_at`, `channel_id`);
-- Create "views" table
CREATE TABLE `views` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `user_id` text NOT NULL, `video_id` text NOT NULL, `progress` integer NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `views_user_id_fkey` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE, CONSTRAINT `views_video_id_fkey` FOREIGN KEY (`video_id`) REFERENCES `videos` (`id`));
-- Create index "idx_viewsuser_id" to table: "views"
CREATE INDEX `idx_viewsuser_id` ON `views` (`user_id`);
-- Create index "idx_views_video_id_user_id" to table: "views"
CREATE INDEX `idx_views_video_id_user_id` ON `views` (`video_id`, `user_id`);
-- Create "users" table
CREATE TABLE `users` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, PRIMARY KEY (`id`));
-- Create "sessions" table
CREATE TABLE `sessions` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `user_id` text NULL, `connection_id` text NULL, `expires_at` date NOT NULL, `last_used` date NOT NULL, `deleted` boolean NOT NULL DEFAULT false, PRIMARY KEY (`id`));
-- Create index "idx_sessions_id_expires_at_deleted" to table: "sessions"
CREATE INDEX `idx_sessions_id_expires_at_deleted` ON `sessions` (`id`, `expires_at`, `deleted`);
-- Create index "idx_sessions_user_id_expires_at_deleted" to table: "sessions"
CREATE INDEX `idx_sessions_user_id_expires_at_deleted` ON `sessions` (`user_id`, `expires_at`, `deleted`);
-- Create index "idx_sessions_user_id_last_used_deleted" to table: "sessions"
CREATE INDEX `idx_sessions_user_id_last_used_deleted` ON `sessions` (`user_id`, `last_used`, `deleted`);
-- Create "connections" table
CREATE TABLE `connections` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `type` text NOT NULL, `user_id` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `connections_user_id_fkey` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE);
-- Create index "idx_connections_user_id" to table: "connections"
CREATE INDEX `idx_connections_user_id` ON `connections` (`user_id`);
-- Create index "idx_connections_user_id_type" to table: "connections"
CREATE INDEX `idx_connections_user_id_type` ON `connections` (`user_id`, `type`);
-- Create "settings" table
CREATE TABLE `settings` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `data` blob NOT NULL, `user_id` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `settings_user_id_fkey` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE);
-- Create index "idx_settings_user_id" to table: "settings"
CREATE INDEX `idx_settings_user_id` ON `settings` (`user_id`);
-- Create index "idx_settings_id_user_id_unique" to table: "settings"
CREATE UNIQUE INDEX `idx_settings_id_user_id_unique` ON `settings` (`id`, `user_id`);
-- Create "subscriptions" table
CREATE TABLE `subscriptions` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `favorite` boolean NOT NULL, `channel_id` text NOT NULL, `user_id` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `subscriptions_channel_id_fkey` FOREIGN KEY (`channel_id`) REFERENCES `channels` (`id`) ON DELETE CASCADE, CONSTRAINT `subscriptions_user_id_fkey` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE);
-- Create index "idx_subscriptions_user_id" to table: "subscriptions"
CREATE INDEX `idx_subscriptions_user_id` ON `subscriptions` (`user_id`);
-- Create index "idx_subscriptions_user_id_favorite" to table: "subscriptions"
CREATE INDEX `idx_subscriptions_user_id_favorite` ON `subscriptions` (`user_id`, `favorite`);
-- Create index "idx_subscriptions_user_id_channel_id_unique" to table: "subscriptions"
CREATE UNIQUE INDEX `idx_subscriptions_user_id_channel_id_unique` ON `subscriptions` (`user_id`, `channel_id`);
-- Create index "idx_subscriptions_channel_id" to table: "subscriptions"
CREATE INDEX `idx_subscriptions_channel_id` ON `subscriptions` (`channel_id`);
