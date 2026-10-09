// Package dump selects a consistent subset of a PostgreSQL database, seed rows
// with their children and every row they reference, and writes it as a psql
// data script.
package dump

import (
	"errors"
	"fmt"
	"io"
	"path"

	"github.com/LdDl/bungen/util"
	"gopkg.in/yaml.v3"
)

const (
	defaultChildDepth        = 1
	defaultChildrenPerParent = 20
)

// Config is the dump configuration file.
type Config struct {
	Seeds             []Seed   `yaml:"seeds"`
	ChildrenPerParent *int     `yaml:"children_per_parent"`
	Exclude           []string `yaml:"exclude"`
	Attach            []string `yaml:"attach"`
}

// Seed selects the rows a dump starts from. The seed table is aliased t in
// Where and OrderBy.
type Seed struct {
	Table             string   `yaml:"table"`
	Where             string   `yaml:"where"`
	OrderBy           string   `yaml:"order_by"`
	Limit             *int     `yaml:"limit"`
	Query             string   `yaml:"query"`
	ChildDepth        *int     `yaml:"child_depth"`
	ChildrenPerParent *int     `yaml:"children_per_parent"`
	AllChildren       []string `yaml:"all_children"`
}

// ParseConfig reads and validates a configuration.
func ParseConfig(r io.Reader) (*Config, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("dump config: empty")
		}
		return nil, fmt.Errorf("dump config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("dump config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Seeds) == 0 {
		return errors.New("no seeds")
	}
	if err := atLeast("children_per_parent", c.ChildrenPerParent, 1); err != nil {
		return err
	}
	if err := validPatterns("exclude", c.Exclude); err != nil {
		return err
	}
	if err := validPatterns("attach", c.Attach); err != nil {
		return err
	}
	for i, s := range c.Seeds {
		if err := s.validate(); err != nil {
			return fmt.Errorf("seed %d (%s): %w", i+1, s.Table, err)
		}
	}
	return nil
}

func (s Seed) validate() error {
	if s.Table == "" {
		return errors.New("table is required")
	}
	if s.Query != "" && (s.Where != "" || s.OrderBy != "" || s.Limit != nil) {
		return errors.New("query excludes where, order_by and limit")
	}
	if err := atLeast("limit", s.Limit, 1); err != nil {
		return err
	}
	if err := atLeast("child_depth", s.ChildDepth, 0); err != nil {
		return err
	}
	if err := atLeast("children_per_parent", s.ChildrenPerParent, 1); err != nil {
		return err
	}
	return validPatterns("all_children", s.AllChildren)
}

func validPatterns(name string, patterns []string) error {
	for _, p := range patterns {
		if _, err := path.Match(qualify(p), ""); err != nil {
			return fmt.Errorf("%s %q: %w", name, p, err)
		}
	}
	return nil
}

func atLeast(name string, v *int, low int) error {
	if v != nil && *v < low {
		return fmt.Errorf("%s must be at least %d, got %d", name, low, *v)
	}
	return nil
}

// qualify adds the public schema to a name without one.
func qualify(name string) string {
	schema, table := util.Split(name)
	return util.Join(schema, table)
}

// key is the seed table as schema.table.
func (s Seed) key() string {
	return qualify(s.Table)
}

func (s Seed) childDepth() int {
	if s.ChildDepth != nil {
		return *s.ChildDepth
	}
	return defaultChildDepth
}

func (c *Config) childrenPerParent(s Seed) int {
	switch {
	case s.ChildrenPerParent != nil:
		return *s.ChildrenPerParent
	case c.ChildrenPerParent != nil:
		return *c.ChildrenPerParent
	}
	return defaultChildrenPerParent
}

// childLimit is how many rows of the child table key a seed's child expansion
// takes per parent row; 0 means all of them (all_children).
func (c *Config) childLimit(s Seed, key string) int {
	if matchAny(s.AllChildren, key) {
		return 0
	}
	return c.childrenPerParent(s)
}

// excluded reports whether child expansion must skip the table key.
func (c *Config) excluded(key string) bool {
	return matchAny(c.Exclude, key)
}

// attached reports whether the rows of the table key referencing a selected
// row are selected too, wherever that row comes from.
func (c *Config) attached(key string) bool {
	return matchAny(c.Attach, key)
}

// matchAny reports whether key matches one of patterns, a bare pattern
// meaning a public table.
func matchAny(patterns []string, key string) bool {
	for _, p := range patterns {
		if ok, _ := path.Match(qualify(p), key); ok {
			return true
		}
	}
	return false
}
