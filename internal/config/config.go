package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DirName          = ".eigenmemory"
	ConfigFileName   = "config.json"
	SourcesDirName   = "sources"
	WikiDirName      = "wiki"
	DatabaseFileName = "eigenmemory.db"
	GlobalDirName    = ".eigenmemory"
	IndexFileName    = "index.md"
	LogFileName      = "log.md"
)

// Scope identifies whether we are operating on project or global memory.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeGlobal  Scope = "global"
)

// Config is the project-level runtime configuration.
type Config struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	CreatedAt string `json:"createdAt"`
	// ClaudeProjectDir is the sanitized directory name Claude Code uses under
	// ~/.claude/projects/ for this project's native memory — derived from the
	// project's absolute path at `eigenmemory init` time, NOT the same as
	// Name above (Claude Code keys its memory directories by working-directory
	// path, not by any human-chosen project name). Empty for global-scope
	// configs, and for project configs written before this field existed;
	// callers should fall back to config.SanitizeClaudeProjectDir on the
	// project root's parent directory in that case (see
	// adapters.ResolveClaudeProjectDir).
	ClaudeProjectDir string `json:"claudeProjectDir,omitempty"`
}

// Paths holds all filesystem paths for a given scope.
type Paths struct {
	Root       string
	ConfigFile string
	SourcesDir string
	WikiDir    string
	Database   string
	IndexFile  string
	LogFile    string
}

// Default returns a new default configuration.
func Default(name string) Config {
	return Config{
		Name:    name,
		Version: "0.1.0",
	}
}

// ScopeFromCWD detects the active scope by looking for a project eigenmemory
// directory in the current working directory or its parents. If none is found,
// it falls back to the global scope under the user's home directory.
func ScopeFromCWD() (Scope, *Paths, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("get working directory: %w", err)
	}

	root := findProjectRoot(cwd)
	if root != "" {
		return ScopeProject, PathsFor(root), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil, fmt.Errorf("get home directory: %w", err)
	}
	globalRoot := filepath.Join(home, GlobalDirName)
	return ScopeGlobal, PathsFor(globalRoot), nil
}

// ValidateProjectName rejects project names that could escape the intended
// directory when used as a path component (e.g. in Claude Code's
// ~/.claude/projects/<name>/memory/ projection). An empty name is allowed;
// callers that require a non-empty name check for that separately.
func ValidateProjectName(name string) error {
	if name == "" {
		return nil
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid project name %q", name)
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid project name %q: must not contain path separators or \"..\"", name)
	}
	return nil
}

// SanitizeClaudeProjectDir reproduces Claude Code's own scheme for naming a
// project's directory under ~/.claude/projects/: every path separator in the
// absolute project path is replaced with "-" (e.g. "/home/j/proj" becomes
// "-home-j-proj"). Verified against Claude Code's actual on-disk directory
// naming on Unix; unconfirmed on Windows, where the drive letter's ":" is
// left untouched.
func SanitizeClaudeProjectDir(absPath string) string {
	replacer := strings.NewReplacer("/", "-", `\`, "-")
	return replacer.Replace(absPath)
}

// LoadConfig reads the project config.json if it exists.
func LoadConfig(paths *Paths) (Config, error) {
	data, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	// A repo's config.json is not trusted input: it can be shipped by anyone
	// the user clones from. Reject a malicious/corrupt project name here
	// rather than letting it reach a path Join downstream (e.g. reconcile's
	// Claude Code memory projection).
	if err := ValidateProjectName(cfg.Name); err != nil {
		return Config{}, fmt.Errorf("%s: %w", paths.ConfigFile, err)
	}
	return cfg, nil
}

// SaveConfig writes the project config.json atomically.
func SaveConfig(paths *Paths, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return writeFileAtomic(paths.ConfigFile, data)
}

// PathsFor builds a Paths value from a root directory.
func PathsFor(root string) *Paths {
	return &Paths{
		Root:       root,
		ConfigFile: filepath.Join(root, ConfigFileName),
		SourcesDir: filepath.Join(root, SourcesDirName),
		WikiDir:    filepath.Join(root, WikiDirName),
		Database:   filepath.Join(root, DatabaseFileName),
		IndexFile:  filepath.Join(root, WikiDirName, IndexFileName),
		LogFile:    filepath.Join(root, WikiDirName, LogFileName),
	}
}

// findProjectRoot walks upward from dir looking for a .eigenmemory directory.
func findProjectRoot(dir string) string {
	for {
		candidate := filepath.Join(dir, DirName)
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// writeFileAtomic writes data to path using a temporary file and rename.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
