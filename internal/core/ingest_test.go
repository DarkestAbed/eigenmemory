package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIngest_InlineText(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	result, err := store.Ingest(IngestOptions{Source: "Some raw text.", Name: "note"})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if !strings.Contains(result, "note") {
		t.Errorf("result = %q, missing source name", result)
	}

	count, err := store.Search.CountSources()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("CountSources = %d, want 1", count)
	}
}

func TestIngest_DefaultName(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	result, err := store.Ingest(IngestOptions{Source: "No name given."})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if !strings.Contains(result, "inline-source") {
		t.Errorf("result = %q, want default name inline-source", result)
	}
}

func TestIngest_FilePath(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	srcPath := filepath.Join(tmp, "doc.txt")
	if err := os.WriteFile(srcPath, []byte("Design doc contents."), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := store.Ingest(IngestOptions{Source: srcPath, IsPath: true})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if !strings.Contains(result, "doc.txt") {
		t.Errorf("result = %q, missing basename", result)
	}
}

func TestIngest_FilePathMissing(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if _, err := store.Ingest(IngestOptions{Source: filepath.Join(tmp, "missing.txt"), IsPath: true}); err == nil {
		t.Fatal("expected error for a missing source file")
	}
}

func TestIngest_Deduplicates(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if _, err := store.Ingest(IngestOptions{Source: "same content", Name: "dup"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(IngestOptions{Source: "same content", Name: "dup"}); err != nil {
		t.Fatal(err)
	}

	count, err := store.Search.CountSources()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("CountSources after duplicate ingest = %d, want 1", count)
	}

	// Ingesting the same source content again under the same derived slug
	// must update the existing summary page rather than erroring.
	result, err := store.Ingest(IngestOptions{Source: "same content", Name: "dup"})
	if err != nil {
		t.Fatalf("third Ingest: %v", err)
	}
	if !strings.Contains(result, "dup") {
		t.Errorf("result = %q, missing name", result)
	}
}

func TestIngest_UnnamedSlugFallsBackToSourceID(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	// A name made entirely of characters Slugify strips produces an empty
	// slug; Ingest must fall back to a source-id-derived slug instead of
	// erroring on an empty page slug.
	result, err := store.Ingest(IngestOptions{Source: "content", Name: "***"})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if strings.Contains(result, "summary/.md") {
		t.Errorf("result = %q, produced an empty slug", result)
	}
}
