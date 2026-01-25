-- Add video_filter column to subscriptions table
ALTER TABLE subscriptions ADD COLUMN video_filter TEXT NOT NULL DEFAULT 'all';
