-- Create "playlists" table
CREATE TABLE `playlists` (
  `id` text NOT NULL,
  `created_at` date NOT NULL,
  `updated_at` date NOT NULL,
  `user_id` text NOT NULL,
  `slug` text NOT NULL,
  `name` text NOT NULL,
  `system` boolean NOT NULL DEFAULT false,
  `ttl_days` integer NULL,
  `max_size` integer NULL,
  PRIMARY KEY (`id`),
  CONSTRAINT `playlists_user_id_fkey` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
);
-- Create index "idx_playlists_user_id" to table: "playlists"
CREATE INDEX `idx_playlists_user_id` ON `playlists` (`user_id`);
-- Create index "idx_playlists_user_id_slug_unique" to table: "playlists"
CREATE UNIQUE INDEX `idx_playlists_user_id_slug_unique` ON `playlists` (`user_id`, `slug`);
-- Create "playlist_items" table
CREATE TABLE `playlist_items` (
  `id` text NOT NULL,
  `created_at` date NOT NULL,
  `updated_at` date NOT NULL,
  `playlist_id` text NOT NULL,
  `video_id` text NOT NULL,
  `position` integer NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  CONSTRAINT `playlist_items_playlist_id_fkey` FOREIGN KEY (`playlist_id`) REFERENCES `playlists` (`id`) ON DELETE CASCADE,
  CONSTRAINT `playlist_items_video_id_fkey` FOREIGN KEY (`video_id`) REFERENCES `videos` (`id`)
);
-- Create index "idx_playlist_items_playlist_id" to table: "playlist_items"
CREATE INDEX `idx_playlist_items_playlist_id` ON `playlist_items` (`playlist_id`);
-- Create index "idx_playlist_items_playlist_id_video_id_unique" to table: "playlist_items"
CREATE UNIQUE INDEX `idx_playlist_items_playlist_id_video_id_unique` ON `playlist_items` (`playlist_id`, `video_id`);
-- Create index "idx_playlist_items_playlist_id_created_at" to table: "playlist_items"
CREATE INDEX `idx_playlist_items_playlist_id_created_at` ON `playlist_items` (`playlist_id`, `created_at`);
