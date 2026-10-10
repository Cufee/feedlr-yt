-- Create "podcast_transcript_contents" table
CREATE TABLE `podcast_transcript_contents` (
  `video_id` text NOT NULL,
  `source_key` text NOT NULL,
  `source` text NOT NULL,
  `source_url` text NOT NULL,
  `content_hash` text NOT NULL,
  `model` text NOT NULL,
  `duration_ms` integer NOT NULL,
  `cues_json` blob NOT NULL,
  `usage_json` blob NOT NULL,
  `updated_at_ms` integer NOT NULL,
  PRIMARY KEY (`video_id`, `source_key`),
  CONSTRAINT `podcast_transcript_contents_video_id_fkey` FOREIGN KEY (`video_id`) REFERENCES `videos` (`id`) ON DELETE CASCADE
);
-- Create "podcast_processing_jobs" table
CREATE TABLE `podcast_processing_jobs` (
  `id` text NOT NULL,
  `video_id` text NOT NULL,
  `source_key` text NOT NULL,
  `model` text NOT NULL,
  `prompt_version` text NOT NULL,
  `status` text NOT NULL,
  `phase` text NOT NULL,
  `error` text NOT NULL,
  `transcript_hash` text NOT NULL,
  `analysis_id` text NOT NULL,
  `duration_ms` integer NOT NULL,
  `token` text NOT NULL,
  `lease_until_ms` integer NOT NULL,
  `created_at_ms` integer NOT NULL,
  `updated_at_ms` integer NOT NULL,
  PRIMARY KEY (`id`),
  CONSTRAINT `podcast_processing_jobs_video_id_fkey` FOREIGN KEY (`video_id`) REFERENCES `videos` (`id`) ON DELETE CASCADE
);
-- Create index "idx_podcast_processing_jobs_input_unique" to table: "podcast_processing_jobs"
CREATE UNIQUE INDEX `idx_podcast_processing_jobs_input_unique` ON `podcast_processing_jobs` (`video_id`, `source_key`, `model`, `prompt_version`);
-- Create index "idx_podcast_processing_jobs_status_lease" to table: "podcast_processing_jobs"
CREATE INDEX `idx_podcast_processing_jobs_status_lease` ON `podcast_processing_jobs` (`status`, `lease_until_ms`);
-- Create "podcast_transcription_slots" table
CREATE TABLE `podcast_transcription_slots` (
  `token` text NOT NULL,
  `lease_until_ms` integer NOT NULL,
  PRIMARY KEY (`token`)
);
