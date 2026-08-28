package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DarkestAbed/eigenmemory/internal/types"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
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

// TestIngest_StripsMdSlugSuffix is a regression test for the slug-extension
// mismatch: ingesting "target.md" used to produce slug "target-md" (Slugify
// mapped "." to a segment), so [[target]] links could never resolve. The slug
// must now be "target", matching the link side (wiki.LinkToSlug strips ".md").
func TestIngest_StripsMdSlugSuffix(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	srcPath := filepath.Join(tmp, "target.md")
	if err := os.WriteFile(srcPath, []byte("# Target\n\nBody."), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := store.Ingest(IngestOptions{Source: srcPath, IsPath: true})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if !strings.Contains(result, "summary/target.md") {
		t.Errorf("result = %q, want slug target", result)
	}
	page, err := store.LoadPage(types.PageTypeSummary, "target")
	if err != nil {
		t.Fatalf("LoadPage target: %v", err)
	}
	if page.Slug != "target" {
		t.Errorf("slug = %q, want target", page.Slug)
	}
	if _, err := store.LoadPage(types.PageTypeSummary, "target-md"); err == nil {
		t.Error("legacy target-md page should not exist after the fix")
	}

	// Mixed-case and multi-dot names strip only the trailing .md.
	memPath := filepath.Join(tmp, "MEMORY.md")
	if err := os.WriteFile(memPath, []byte("# Memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(IngestOptions{Source: memPath, IsPath: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadPage(types.PageTypeSummary, "memory"); err != nil {
		t.Errorf("LoadPage memory: %v (want slug memory from MEMORY.md)", err)
	}
}

// TestIngest_SourceLinksAreEdges verifies that [[wikilinks]] authored in an
// ingested source's prose become links_to graph edges, extracted from the full
// raw source (not the ~2KB truncated summary body). The summary body itself
// must contribute no edges: its leak-proof fence wraps the digest as verbatim.
func TestIngest_SourceLinksAreEdges(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	targetPath := filepath.Join(tmp, "target.md")
	if err := os.WriteFile(targetPath, []byte("# Target\n\nThe target doc."), 0o644); err != nil {
		t.Fatal(err)
	}
	carrierPath := filepath.Join(tmp, "carrier.md")
	if err := os.WriteFile(carrierPath, []byte("# Carrier\n\nSee [[target]] for context.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Ingest(IngestOptions{Source: targetPath, IsPath: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(IngestOptions{Source: carrierPath, IsPath: true}); err != nil {
		t.Fatal(err)
	}

	rels, err := store.Search.ListRelations()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, r := range rels {
		if r.From == "carrier" && r.To == "target" && r.Type == "links_to" {
			found = true
		}
	}
	if !found {
		t.Errorf("carrier -> target links_to edge missing; relations = %+v", rels)
	}

	// The leak-proof summary body must not itself leak the source's link.
	carrierPage, err := store.LoadPage(types.PageTypeSummary, "carrier")
	if err != nil {
		t.Fatalf("LoadPage carrier: %v", err)
	}
	if leaks := wiki.ExtractLinks(carrierPage.Body); len(leaks) != 0 {
		t.Errorf("summary body leaked links %v; want none (leak-proof fence)", leaks)
	}
}

// TestIngest_SourceLinkExtractionIgnoresCodeBlocks verifies extraction is
// deterministic regardless of fence parity (the old body-wrapper bug) and that
// a [[wikilink]] inside the source's own fenced code block is NOT an edge.
func TestIngest_SourceLinkExtractionIgnoresCodeBlocks(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	targetPath := filepath.Join(tmp, "target.md")
	if err := os.WriteFile(targetPath, []byte("# Target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(IngestOptions{Source: targetPath, IsPath: true}); err != nil {
		t.Fatal(err)
	}

	// [[target]] in prose is an edge; [[ghost]] inside a fenced code block is a
	// literal and must not be extracted.
	carrier := []byte("# Carrier\n\nPreamble.\n\n```\n[[ghost]]\n```\n\nSee [[target]].\n")
	carrierPath := filepath.Join(tmp, "carrier.md")
	if err := os.WriteFile(carrierPath, carrier, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ingest(IngestOptions{Source: carrierPath, IsPath: true}); err != nil {
		t.Fatal(err)
	}

	rels, err := store.Search.ListRelations()
	if err != nil {
		t.Fatal(err)
	}
	var hasTarget, hasGhost bool
	for _, r := range rels {
		if r.From == "carrier" && r.To == "target" {
			hasTarget = true
		}
		if r.From == "carrier" && r.To == "ghost" {
			hasGhost = true
		}
	}
	if !hasTarget {
		t.Errorf("prose [[target]] not extracted as edge; rels = %+v", rels)
	}
	if hasGhost {
		t.Errorf("code-block [[ghost]] must not be an edge; rels = %+v", rels)
	}
}
