-- Add description column to playlists
ALTER TABLE `playlists` ADD COLUMN `description` text NOT NULL DEFAULT '';
-- Add youtube_playlist_id column to playlists (for imported playlists)
ALTER TABLE `playlists` ADD COLUMN `youtube_playlist_id` text NULL;
-- Create index for youtube_playlist_id lookups
CREATE INDEX `idx_playlists_youtube_playlist_id` ON `playlists` (`youtube_playlist_id`);
-- Create index for ordered playlist item retrieval by position
CREATE INDEX `idx_playlist_items_playlist_id_position` ON `playlist_items` (`playlist_id`, `position`);
