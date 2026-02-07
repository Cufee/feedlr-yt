-- Create "youtube_sync_accounts" table
CREATE TABLE `youtube_sync_accounts` (
  `id` text NOT NULL,
  `created_at` date NOT NULL,
  `updated_at` date NOT NULL,
  `user_id` text NOT NULL,
  `refresh_token_enc` blob NOT NULL,
  `enc_secret_hash` text NOT NULL,
  `playlist_id` text NULL,
  `sync_enabled` boolean NOT NULL DEFAULT true,
  `last_feed_video_published_at` date NULL,
  `last_synced_at` date NULL,
  `last_sync_attempt_at` date NULL,
  `last_error` text NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  CONSTRAINT `youtube_sync_accounts_user_id_fkey` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
);
-- Create index "idx_youtube_sync_accounts_user_id_unique" to table: "youtube_sync_accounts"
CREATE UNIQUE INDEX `idx_youtube_sync_accounts_user_id_unique` ON `youtube_sync_accounts` (`user_id`);
-- Create index "idx_youtube_sync_accounts_sync_enabled_last_synced_at" to table: "youtube_sync_accounts"
CREATE INDEX `idx_youtube_sync_accounts_sync_enabled_last_synced_at` ON `youtube_sync_accounts` (`sync_enabled`, `last_synced_at`);
