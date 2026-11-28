-- +goose Up
-- The instructions of the subscription page get translations (#278): their title and text in
-- the page's languages, as JSON by language ({"en": {"title": …, "body": …}},
-- subpage.MarshalDocI18n); the page shows its visitor's, the instruction's own where there is
-- none. The titles of the translations are kept apart too (subpage.MarshalDocTitles): the page's
-- list reads them on every view, and reading them out of the translations would make the
-- database parse every translated text along with them. The checks keep both JSON objects.
ALTER TABLE sub_docs
  ADD COLUMN translations TEXT NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(translations::JSONB) = 'object'),
  ADD COLUMN translated_titles TEXT NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(translated_titles::JSONB) = 'object');

-- +goose Down
-- The releases before know the instructions in their own words alone.
ALTER TABLE sub_docs DROP COLUMN translated_titles, DROP COLUMN translations;
