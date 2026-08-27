package types

import (
	"time"
)

// PageType categorizes wiki pages.
type PageType string

const (
	PageTypeEntity    PageType = "entity"
	PageTypeConcept   PageType = "concept"
	PageTypeSummary   PageType = "summary"
	PageTypeProject   PageType = "project"
	PageTypeFeedback  PageType = "feedback"
	PageTypeReference PageType = "reference"
	PageTypeUser      PageType = "user"
)

// PageStatus tracks the lifecycle of a page.
type PageStatus string

const (
	PageStatusActive   PageStatus = "active"
	PageStatusStale    PageStatus = "stale"
	PageStatusMerged   PageStatus = "merged"
	PageStatusArchived PageStatus = "archived"
)

// Frontmatter is the YAML header shared by all wiki pages.
type Frontmatter struct {
	ID        string     `yaml:"id"`
	Type      PageType   `yaml:"type"`
	Status    PageStatus `yaml:"status"`
	Created   time.Time  `yaml:"created"`
	Updated   time.Time  `yaml:"updated"`
	Sources   []string   `yaml:"sources,omitempty"`
	Tags      []string   `yaml:"tags,omitempty"`
	Relations []Relation `yaml:"relations,omitempty"`
}

// Relation is a typed, directed link between two wiki pages.
type Relation struct {
	From       string `yaml:"from"`
	To         string `yaml:"to"`
	Type       string `yaml:"type"`
	Provenance string `yaml:"provenance,omitempty"`
}

// Page is a loaded wiki page with its frontmatter and body.
type Page struct {
	Frontmatter Frontmatter
	Body        string
	Path        string
	Slug        string
}

// ValidPageTypes returns the set of allowed page types.
func ValidPageTypes() []PageType {
	return []PageType{
		PageTypeEntity,
		PageTypeConcept,
		PageTypeSummary,
		PageTypeProject,
		PageTypeFeedback,
		PageTypeReference,
		PageTypeUser,
	}
}

// DefaultFrontmatter returns a fresh frontmatter for a new page.
func DefaultFrontmatter(pageType PageType) Frontmatter {
	now := time.Now().UTC()
	return Frontmatter{
		ID:      NewID(),
		Type:    pageType,
		Status:  PageStatusActive,
		Created: now,
		Updated: now,
	}
}
