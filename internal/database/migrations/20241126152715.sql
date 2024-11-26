-- Create "app_configuration" table
CREATE TABLE `app_configuration` (`id` text NOT NULL, `created_at` date NOT NULL, `updated_at` date NOT NULL, `version` integer NOT NULL, `data` blob NOT NULL, PRIMARY KEY (`id`));
