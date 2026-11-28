-- +goose Up
-- The apps of the subscription page get icons of their own (#278): an image of each app beside
-- the logo and the background, named app-<the app's id>, a built-in app's or one of the
-- admin's own (subpage.AppIconAsset).
ALTER TABLE sub_assets DROP CONSTRAINT sub_assets_name_check;
ALTER TABLE sub_assets ADD CONSTRAINT sub_assets_name_check CHECK (name IN ('logo', 'background') OR name ~ '^app-[a-z0-9-]{1,24}$');

-- +goose Down
-- The apps' icons go: the releases before draw every app with its letter.
DELETE FROM sub_assets WHERE name NOT IN ('logo', 'background');
ALTER TABLE sub_assets DROP CONSTRAINT sub_assets_name_check;
ALTER TABLE sub_assets ADD CONSTRAINT sub_assets_name_check CHECK (name IN ('logo', 'background'));
