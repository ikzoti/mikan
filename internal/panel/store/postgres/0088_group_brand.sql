-- +goose Up
-- A group's own brand on its own page (#278): its name and colour go over the panel's for the
-- group's clients, on the page and in the apps, and its logo and background are images of the
-- group, named for it (subpage.GroupAsset). NULL is the panel's.
ALTER TABLE user_group_appearance ADD COLUMN brand TEXT, ADD COLUMN brand_accent TEXT;
ALTER TABLE sub_assets DROP CONSTRAINT sub_assets_name_check;
ALTER TABLE sub_assets ADD CONSTRAINT sub_assets_name_check CHECK (name IN ('logo', 'background') OR name ~ '^app-[a-z0-9-]{1,24}$' OR name ~ '^(logo|background)-g[1-9][0-9]{0,18}$');

-- +goose Down
-- The releases before show every group the panel's brand.
DELETE FROM sub_assets WHERE name ~ '^(logo|background)-g';
ALTER TABLE sub_assets DROP CONSTRAINT sub_assets_name_check;
ALTER TABLE sub_assets ADD CONSTRAINT sub_assets_name_check CHECK (name IN ('logo', 'background') OR name ~ '^app-[a-z0-9-]{1,24}$');
ALTER TABLE user_group_appearance DROP COLUMN brand_accent, DROP COLUMN brand;
