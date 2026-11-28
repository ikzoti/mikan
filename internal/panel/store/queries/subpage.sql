-- name: ListSubDocs :many
SELECT * FROM sub_docs ORDER BY position, id;

-- name: ListPublishedSubDocs :many
-- What the subscription page lists: the published pages without their text, and of their
-- translations the titles alone.
SELECT id, title, emoji, platform, translated_titles FROM sub_docs WHERE published = 1 ORDER BY position, id;

-- name: ListSubDocIDs :many
-- Which of ids are instructions: an app of the page links to those alone.
SELECT id FROM sub_docs WHERE id = ANY(sqlc.arg(ids)::bigint[]) ORDER BY id;

-- name: GetPublishedSubDoc :one
SELECT * FROM sub_docs WHERE id = $1 AND published = 1;

-- name: CountSubDocs :one
SELECT COUNT(*) FROM sub_docs;

-- name: CreateSubDoc :one
-- A new page goes last.
INSERT INTO sub_docs (title, emoji, body, platform, translations, translated_titles, position, published, created_at, updated_at)
VALUES (sqlc.arg(title), sqlc.arg(emoji), sqlc.arg(body), sqlc.arg(platform), sqlc.arg(translations), sqlc.arg(translated_titles), (SELECT COALESCE(MAX(d.position), 0) + 1 FROM sub_docs d), sqlc.arg(published), sqlc.arg(at), sqlc.arg(at))
RETURNING *;

-- name: UpdateSubDoc :one
-- No translations given keeps the ones it has, and their titles: a script that knows nothing
-- of them does not drop them.
UPDATE sub_docs SET title = sqlc.arg(title), emoji = sqlc.arg(emoji), body = sqlc.arg(body), platform = sqlc.arg(platform), published = sqlc.arg(published),
  translations = COALESCE(sqlc.narg(translations)::TEXT, translations), translated_titles = COALESCE(sqlc.narg(translated_titles)::TEXT, translated_titles),
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetSubDocPosition :execrows
UPDATE sub_docs SET position = $2 WHERE id = $1;

-- name: DeleteSubDoc :execrows
DELETE FROM sub_docs WHERE id = $1;

-- name: GetSubAsset :one
SELECT * FROM sub_assets WHERE name = $1;

-- name: ListSubAssetMeta :many
-- What the page needs to link the images: no bytes.
SELECT name, content_type, hash, OCTET_LENGTH(data)::BIGINT AS size, updated_at FROM sub_assets ORDER BY name;

-- name: PutSubAsset :exec
INSERT INTO sub_assets (name, content_type, data, hash, updated_at) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (name) DO UPDATE SET content_type = excluded.content_type, data = excluded.data, hash = excluded.hash, updated_at = excluded.updated_at;

-- name: DeleteSubAsset :execrows
DELETE FROM sub_assets WHERE name = $1;

-- name: DeleteSubAssets :execrows
-- The icons of apps taken out of the page, all in one go.
DELETE FROM sub_assets WHERE name = ANY(sqlc.arg(names)::text[]);
