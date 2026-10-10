-- Add column "input_json" to table: "podcast_segment_analyses"
ALTER TABLE `podcast_segment_analyses` ADD COLUMN `input_json` blob NOT NULL DEFAULT '{}';
-- Add column "input_json" to table: "podcast_transcript_contents"
ALTER TABLE `podcast_transcript_contents` ADD COLUMN `input_json` blob NOT NULL DEFAULT '{}';
-- Create "podcast_source_validations" table
CREATE TABLE `podcast_source_validations` (
  `video_id` text NOT NULL,
  `metadata_key` text NOT NULL,
  `fingerprint` text NOT NULL,
  `input_json` blob NOT NULL,
  `validated_at` date NOT NULL,
  PRIMARY KEY (`video_id`, `metadata_key`),
  CONSTRAINT `podcast_source_validations_video_id_fkey` FOREIGN KEY (`video_id`) REFERENCES `videos` (`id`) ON DELETE CASCADE
);
