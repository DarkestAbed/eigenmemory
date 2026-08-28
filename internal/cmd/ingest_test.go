package cmd

import (
	"strings"
	"testing"

	"github.com/DarkestAbed/eigenmemory/internal/config"
)

func TestIngestCmd_InlineText(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "ingestproj", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return Ingest(IngestOptions{Source: "Some raw text to ingest.", Name: "note"})
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if !strings.Contains(out, "Ingested") {
		t.Errorf("expected ingest confirmation, got %q", out)
	}
}

func TestIngestCmd_ScopeResolutionFails(t *testing.T) {
	unresolvableScope(t)

	if err := Ingest(IngestOptions{Source: "text", Name: "note"}); err == nil {
		t.Fatal("expected error when neither project nor global scope can be resolved")
	}
}
