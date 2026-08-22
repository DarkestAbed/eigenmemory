package wiki

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/types"
)

func TestSaveAndLoadPage(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	// Stub nowUTC for deterministic timestamps.
	fixed := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	oldNow := nowUTC
	nowUTC = func() time.Time { return fixed }
	defer func() { nowUTC = oldNow }()

	page := &types.Page{
		Frontmatter: types.Frontmatter{
			ID:     types.NewID(),
			Type:   types.PageTypeEntity,
			Status: types.PageStatusActive,
			Tags:   []string{"auth", "api"},
		},
		Slug: "auth-service",
		Body: "# Auth Service\n\nHandles login and token refresh.\n",
	}

	if err := SavePage(paths, page, types.PageTypeEntity); err != nil {
		t.Fatalf("SavePage: %v", err)
	}

	loaded, err := LoadPage(paths, types.PageTypeEntity, "auth-service")
	if err != nil {
		t.Fatalf("LoadPage: %v", err)
	}

	if loaded.Slug != page.Slug {
		t.Errorf("Slug = %q, want %q", loaded.Slug, page.Slug)
	}
	if loaded.Frontmatter.Type != page.Frontmatter.Type {
		t.Errorf("Type = %q, want %q", loaded.Frontmatter.Type, page.Frontmatter.Type)
	}
	if len(loaded.Frontmatter.Tags) != 2 {
		t.Errorf("Tags len = %d, want 2", len(loaded.Frontmatter.Tags))
	}
	if loaded.Frontmatter.Updated != fixed {
		t.Errorf("Updated = %v, want %v", loaded.Frontmatter.Updated, fixed)
	}
}

func TestExtractLinks(t *testing.T) {
	body := "See [auth service](entity/auth-service.md) and [concepts](concept/security.md#basics)."
	links := ExtractLinks(body)

	want := map[string]bool{
		"entity/auth-service": true,
		"concept/security":    true,
	}
	got := make(map[string]bool)
	for _, l := range links {
		got[l] = true
	}

	for k := range want {
		if !got[k] {
			t.Errorf("missing link %q", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("unexpected link %q", k)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Auth Service v2", "auth-service-v2"},
		{"  spaces everywhere  ", "spaces-everywhere"},
		{"under_scores!", "under-scores"},
	}
	for _, c := range cases {
		got := Slugify(c.in)
		if got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPageExists(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	if PageExists(paths, types.PageTypeEntity, "missing") {
		t.Error("PageExists = true for missing page")
	}

	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeEntity),
		Slug:        "present",
		Body:        "# Present\n",
	}
	if err := SavePage(paths, page, types.PageTypeEntity); err != nil {
		t.Fatal(err)
	}

	if !PageExists(paths, types.PageTypeEntity, "present") {
		t.Error("PageExists = false for existing page")
	}
}

func TestAtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nested", "file.txt")
	if err := writeFileAtomic(path, []byte("hello")); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("content = %q, want %q", data, "hello")
	}
}
