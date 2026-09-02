package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLAUDE_SubstitutesProjectName is a basic placeholder-substitution check.
func TestCLAUDE_SubstitutesProjectName(t *testing.T) {
	got := CLAUDE("some-project")
	if strings.Contains(got, projectNamePlaceholder) {
		t.Errorf("CLAUDE(%q) still contains the placeholder token", "some-project")
	}
	if !strings.Contains(got, "some-project") {
		t.Errorf("CLAUDE(%q) does not contain the project name", "some-project")
	}
}

// TestAGENTS_SubstitutesProjectName mirrors TestCLAUDE_SubstitutesProjectName for AGENTS.md.
func TestAGENTS_SubstitutesProjectName(t *testing.T) {
	got := AGENTS("some-project")
	if strings.Contains(got, projectNamePlaceholder) {
		t.Errorf("AGENTS(%q) still contains the placeholder token", "some-project")
	}
	if !strings.Contains(got, "some-project") {
		t.Errorf("AGENTS(%q) does not contain the project name", "some-project")
	}
}

// TestRootFiles_MatchTemplate guards against the exact drift the templates
// were reworked to prevent: this repo's own checked-in CLAUDE.md/AGENTS.md
// dogfood the same content `eigenmemory init` ships to every other project.
// If they diverge, either the root files were hand-edited without updating
// the template, or vice versa — both need reconciling before merge.
func TestRootFiles_MatchTemplate(t *testing.T) {
	repoRoot := findRepoRoot(t)

	cases := []struct {
		file string
		want string
	}{
		{"CLAUDE.md", CLAUDE("eigenmemory")},
		{"AGENTS.md", AGENTS("eigenmemory")},
	}

	for _, c := range cases {
		data, err := os.ReadFile(filepath.Join(repoRoot, c.file))
		if err != nil {
			t.Fatalf("read root %s: %v", c.file, err)
		}
		if string(data) != c.want {
			t.Errorf("root %s has drifted from templates.%s(\"eigenmemory\"); "+
				"regenerate it from internal/templates/data/%s.tmpl (or update the template "+
				"if the root file's change was the intended one)", c.file, strings.TrimSuffix(c.file, ".md"), c.file)
		}
	}
}

// findRepoRoot walks up from the current package directory to find the
// module root (identified by go.mod), so the test works regardless of the
// working directory `go test` is invoked from.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}
