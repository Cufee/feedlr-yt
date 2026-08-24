-- Create "podcast_episode_transcripts" table
CREATE TABLE `podcast_episode_transcripts` (
  `video_id` text NOT NULL,
  `url` text NOT NULL,
  `mime_type` text NOT NULL,
  `language` text NULL,
  `rel` text NULL,
  `updated_at` date NOT NULL,
  PRIMARY KEY (`video_id`),
  CONSTRAINT `podcast_episode_transcripts_video_id_fkey` FOREIGN KEY (`video_id`) REFERENCES `videos` (`id`) ON DELETE CASCADE
);
-- Create "podcast_segment_analyses" table
CREATE TABLE `podcast_segment_analyses` (
  `id` text NOT NULL,
  `video_id` text NOT NULL,
  `transcript_hash` text NOT NULL,
  `transcript_url` text NOT NULL,
  `model` text NOT NULL,
  `prompt_version` text NOT NULL,
  `status` text NOT NULL,
  `error` text NULL,
  `started_at` date NULL,
  `completed_at` date NULL,
  `created_at` date NOT NULL,
  `updated_at` date NOT NULL,
  PRIMARY KEY (`id`),
  CONSTRAINT `podcast_segment_analyses_video_id_fkey` FOREIGN KEY (`video_id`) REFERENCES `videos` (`id`) ON DELETE CASCADE
);
-- Create index "idx_podcast_segment_analyses_input_unique" to table: "podcast_segment_analyses"
CREATE UNIQUE INDEX `idx_podcast_segment_analyses_input_unique` ON `podcast_segment_analyses` (`video_id`, `transcript_hash`, `model`, `prompt_version`);
-- Create "podcast_episode_segments" table
CREATE TABLE `podcast_episode_segments` (
  `id` text NOT NULL,
  `video_id` text NOT NULL,
  `analysis_id` text NULL,
  `source` text NOT NULL,
  `position` integer NOT NULL,
  `category` text NOT NULL,
  `start_ms` integer NOT NULL,
  `end_ms` integer NOT NULL,
  `start_cue` integer NOT NULL,
  `end_cue` integer NOT NULL,
  `start_text` text NOT NULL,
  `end_text` text NOT NULL,
  `reason` text NOT NULL,
  `brand` text NULL,
  PRIMARY KEY (`id`),
  CONSTRAINT `podcast_episode_segments_video_id_fkey` FOREIGN KEY (`video_id`) REFERENCES `videos` (`id`) ON DELETE CASCADE,
  CONSTRAINT `podcast_episode_segments_analysis_id_fkey` FOREIGN KEY (`analysis_id`) REFERENCES `podcast_segment_analyses` (`id`) ON DELETE CASCADE
);
-- Create index "idx_podcast_episode_segments_analysis_position" to table: "podcast_episode_segments"
CREATE UNIQUE INDEX `idx_podcast_episode_segments_analysis_position` ON `podcast_episode_segments` (`analysis_id`, `position`);
