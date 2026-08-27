package cmd

import (
	"strings"
	"testing"

	"github.com/javi/eigenmemory/internal/config"
)

func TestReindexCmd(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "reindexproj", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error { return Reindex() })
	if err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	if !strings.Contains(out, "Reindexed") {
		t.Errorf("expected reindex message, got %q", out)
	}
}

func TestReindexCmd_ScopeResolutionFails(t *testing.T) {
	unresolvableScope(t)

	if err := Reindex(); err == nil {
		t.Fatal("expected error when neither project nor global scope can be resolved")
	}
}
