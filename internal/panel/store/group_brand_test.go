package store

import (
	"context"
	"strings"
	"testing"
)

// Migration 0088 gives a group a brand of its own: its name and colour beside its appearance,
// and its own logo and background beside the panel's, named for the group. Going down takes
// the group's images and the columns away, since the releases before know neither.
func TestGroupBrandMigration(t *testing.T) {
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
	for _, name := range []string{"logo", "logo-g1", "background-g42", "logo-g" + strings.Repeat("9", 19)} {
		if err := put(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, name := range []string{"logo-g0", "logo-g", "logo-g01", "logo-gx", "icon-g1", "logo-g1-x", "Logo-g1", "logo-g" + strings.Repeat("9", 20)} {
		if err := put(name); err == nil {
			t.Errorf("%q: taken", name)
		}
	}
	var group int64
	if err := s.DB.QueryRowContext(ctx, `INSERT INTO user_groups (name, created_at) VALUES ('VIP', 1) RETURNING id`).Scan(&group); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO user_group_appearance (group_id, brand, brand_accent) VALUES ($1, 'VIP VPN', '#FF8800')`, group); err != nil {
		t.Fatal(err)
	}

	p, err := postgresProvider(ctx, s.DB, postgresFS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 87); err != nil {
		t.Fatal(err)
	}
	var names string
	if err := s.DB.QueryRowContext(ctx, "SELECT string_agg(name, ',' ORDER BY name) FROM sub_assets").Scan(&names); err != nil || names != "logo" {
		t.Fatalf("down: %q (%v), want the panel's logo alone", names, err)
	}
	if _, err := s.DB.ExecContext(ctx, `SELECT brand FROM user_group_appearance`); err == nil {
		t.Fatal("down: the brand is still there")
	}
	if _, err := p.UpTo(ctx, 88); err != nil {
		t.Fatal(err)
	}
	if err := put("logo-g1"); err != nil {
		t.Fatalf("up again: %v", err)
	}
}
