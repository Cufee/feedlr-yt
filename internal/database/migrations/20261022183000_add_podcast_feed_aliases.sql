CREATE TABLE `podcast_feed_aliases` (
  `feed_url` text NOT NULL,
  `channel_id` text NOT NULL,
  PRIMARY KEY (`feed_url`),
  CONSTRAINT `podcast_feed_aliases_channel_id_fkey` FOREIGN KEY (`channel_id`) REFERENCES `podcast_shows` (`channel_id`) ON DELETE CASCADE
);
CREATE INDEX `idx_podcast_feed_aliases_channel_id` ON `podcast_feed_aliases` (`channel_id`);
INSERT INTO `podcast_feed_aliases` (`feed_url`, `channel_id`) SELECT `feed_url`, `channel_id` FROM `podcast_shows`;
