package store

import (
	"context"
	"strings"
	"testing"
)

// Migration 0086 lets the subscription page keep an icon of each app beside its logo and
// background, named app-<id>, and nothing else. Going down takes the icons away with the
// rule, since the releases before take only the two names.
func TestSubAppIconsMigration(t *testing.T) {
	ctx := context.Background()
	s, err := OpenTest(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	put := func(name string) error {
		_, err := s.DB.ExecContext(ctx, `INSERT INTO sub_assets (name, content_type, data, hash, updated_at) VALUES ($1, 'image/png', '\x89504e47', 'h', 1)`, name)
		return err
	}
	for _, name := range []string{"logo", "background", "app-happ", "app-own-a1b2", "app-" + strings.Repeat("a", 24)} {
		if err := put(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, name := range []string{"app-", "app-Happ", "app-own_a", "app-" + strings.Repeat("a", 25), "application", "favicon", "app-happ/../logo", " app-happ"} {
		if err := put(name); err == nil {
			t.Errorf("%q: taken", name)
		}
	}
	p, err := postgresProvider(ctx, s.DB, postgresFS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 85); err != nil {
		t.Fatal(err)
	}
	var names string
	if err := s.DB.QueryRowContext(ctx, "SELECT string_agg(name, ',' ORDER BY name) FROM sub_assets").Scan(&names); err != nil || names != "background,logo" {
		t.Fatalf("down: %q (%v), want the logo and the background", names, err)
	}
	if err := put("app-happ"); err == nil {
		t.Fatal("an icon passed the old check")
	}
	if _, err := p.UpTo(ctx, 86); err != nil {
		t.Fatal(err)
	}
	if err := put("app-happ"); err != nil {
		t.Fatalf("up again: %v", err)
	}
}
