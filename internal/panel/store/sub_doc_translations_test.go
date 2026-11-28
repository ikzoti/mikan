package store

import (
	"context"
	"testing"
)

// Migration 0087 gives the instructions of the subscription page their translations and, apart,
// the titles of those for the page's list: JSON objects by language, {} for none. Going down
// takes both columns away.
func TestSubDocTranslationsMigration(t *testing.T) {
	ctx := context.Background()
	s, err := OpenTest(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	columns := []string{"translations", "translated_titles"}
	put := func(column, value string) error {
		_, err := s.DB.ExecContext(ctx, `INSERT INTO sub_docs (title, body, `+column+`, published, created_at, updated_at) VALUES ('Как', 'Шаг', $1, 1, 1, 1)`, value)
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO sub_docs (title, created_at, updated_at) VALUES ('Без переводов', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := put("translations", `{"en": {"title": "How", "body": "Step"}, "ru": {"body": "Шаг"}}`); err != nil {
		t.Fatal(err)
	}
	const titles = `{"en": {"title": "How"}}`
	if err := put("translated_titles", titles); err != nil {
		t.Fatal(err)
	}
	for _, column := range columns {
		for _, bad := range []string{"", "not json", "[]", `"en"`, "null", `{"en": `} {
			if err := put(column, bad); err == nil {
				t.Errorf("%s %q: taken", column, bad)
			}
		}
	}
	var none, noTitles string
	if err := s.DB.QueryRowContext(ctx, `SELECT translations, translated_titles FROM sub_docs WHERE title = 'Без переводов'`).Scan(&none, &noTitles); err != nil || none != "{}" || noTitles != "{}" {
		t.Fatalf("a doc without translations: %q, %q (%v)", none, noTitles, err)
	}
	rows, err := s.Q.ListPublishedSubDocs(ctx)
	if err != nil || len(rows) != 2 || rows[0].TranslatedTitles != "{}" || rows[1].TranslatedTitles != titles {
		t.Fatalf("the page's list: %+v (%v)", rows, err)
	}

	p, err := postgresProvider(ctx, s.DB, postgresFS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 86); err != nil {
		t.Fatal(err)
	}
	for _, column := range columns {
		if _, err := s.DB.ExecContext(ctx, `SELECT `+column+` FROM sub_docs`); err == nil {
			t.Fatalf("down: %s is still there", column)
		}
	}
	if _, err := p.UpTo(ctx, 87); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sub_docs WHERE translations = '{}' AND translated_titles = '{}'`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("up again: %d docs without translations (%v)", n, err)
	}
}
