-- Separate export destinations from the source IDs used by imported playlists.
CREATE TABLE `youtube_sync_targets` (
  `account_id` text NOT NULL,
  `source_id` text NOT NULL,
  `playlist_id` text NOT NULL DEFAULT '',
  `title` text NOT NULL DEFAULT '',
  `description` text NOT NULL DEFAULT '',
  `last_attempt_at` date NOT NULL,
  PRIMARY KEY (`account_id`, `source_id`),
  CONSTRAINT `youtube_sync_targets_account_id_fkey` FOREIGN KEY (`account_id`) REFERENCES `youtube_sync_accounts` (`id`) ON DELETE CASCADE
);
