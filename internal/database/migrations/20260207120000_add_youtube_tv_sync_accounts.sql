-- Create "youtube_tv_sync_accounts" table
CREATE TABLE `youtube_tv_sync_accounts` (
  `id` text NOT NULL,
  `created_at` date NOT NULL,
  `updated_at` date NOT NULL,
  `user_id` text NOT NULL,
  `screen_id` text NOT NULL,
  `screen_name` text NOT NULL DEFAULT '',
  `lounge_token_enc` blob NOT NULL,
  `enc_secret_hash` text NOT NULL,
  `sync_enabled` boolean NOT NULL DEFAULT true,
  `connection_state` text NOT NULL DEFAULT 'disconnected',
  `state_reason` text NOT NULL DEFAULT '',
  `last_connected_at` date NULL,
  `last_event_at` date NULL,
  `last_disconnect_at` date NULL,
  `last_user_activity_at` date NULL,
  `last_video_id` text NULL,
  `last_error` text NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  CONSTRAINT `youtube_tv_sync_accounts_user_id_fkey` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
);
-- Create index "idx_youtube_tv_sync_accounts_user_id_unique" to table: "youtube_tv_sync_accounts"
CREATE UNIQUE INDEX `idx_youtube_tv_sync_accounts_user_id_unique` ON `youtube_tv_sync_accounts` (`user_id`);
-- Create index "idx_youtube_tv_sync_accounts_sync_enabled_connection_state" to table: "youtube_tv_sync_accounts"
CREATE INDEX `idx_youtube_tv_sync_accounts_sync_enabled_connection_state` ON `youtube_tv_sync_accounts` (`sync_enabled`, `connection_state`);
-- Create index "idx_youtube_tv_sync_accounts_last_event_at" to table: "youtube_tv_sync_accounts"
CREATE INDEX `idx_youtube_tv_sync_accounts_last_event_at` ON `youtube_tv_sync_accounts` (`last_event_at`);

