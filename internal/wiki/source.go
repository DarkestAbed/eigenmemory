package wiki

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/javi/eigenmemory/internal/config"
)

// Source stores an immutable raw document.
type Source struct {
	ID      string
	SHA256  string
	Path    string
	Name    string
	StoredAt time.Time
}

// IngestSource copies raw source data into .eigenmemory/sources/ keyed by SHA-256.
// It returns the source ID (same as SHA-256) and reports whether it is new.
func IngestSource(paths *config.Paths, name string, data []byte) (*Source, bool, error) {
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	id := sha

	if err := os.MkdirAll(paths.SourcesDir, 0o755); err != nil {
		return nil, false, fmt.Errorf("create sources dir: %w", err)
	}

	sourcePath := filepath.Join(paths.SourcesDir, sha)
	_, err := os.Stat(sourcePath)
	if err == nil {
		// Already exists.
		return &Source{
			ID:       id,
			SHA256:   sha,
			Path:     sourcePath,
			Name:     name,
			StoredAt: time.Now().UTC(),
		}, false, nil
	} else if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("stat source: %w", err)
	}

	if err := writeFileAtomic(sourcePath, data); err != nil {
		return nil, false, fmt.Errorf("write source: %w", err)
	}

	return &Source{
		ID:       id,
		SHA256:   sha,
		Path:     sourcePath,
		Name:     name,
		StoredAt: time.Now().UTC(),
	}, true, nil
}

// LoadSource reads a source from disk by its SHA-256 id.
func LoadSource(paths *config.Paths, id string) ([]byte, error) {
	path := filepath.Join(paths.SourcesDir, id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read source %s: %w", id, err)
	}
	return data, nil
}
