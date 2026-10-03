-- +goose Up
-- add column "color_override" to table: "calendars"
ALTER TABLE `calendars` ADD COLUMN `color_override` text NULL;

-- +goose Down
-- reverse: add column "color_override" to table: "calendars"
ALTER TABLE `calendars` DROP COLUMN `color_override`;
