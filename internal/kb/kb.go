// Package kb loads and matches the nix-explain knowledge base.
//
// The knowledge base is a set of YAML documents. Each document describes one
// error: how to recognise it, what causes it and how to fix it. Rules are
// embedded into the binary, so the tool works fully offline.
package kb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/nepoet/nix-explain/internal/normalize"
)

// Rule describes one known Nix error.
type Rule struct {
	ID       string   `yaml:"id" json:"id"`
	Title    string   `yaml:"title" json:"title"`
	Category string   `yaml:"category" json:"category"`
	Severity string   `yaml:"severity,omitempty" json:"severity,omitempty"`
	Match    []string `yaml:"match" json:"match"`
	MatchAll []string `yaml:"match_all,omitempty" json:"match_all,omitempty"`
	Priority int      `yaml:"priority,omitempty" json:"priority,omitempty"`
	Cause    string   `yaml:"cause" json:"cause"`
	Fix      string   `yaml:"fix" json:"fix"`
	Example  string   `yaml:"example,omitempty" json:"example,omitempty"`
	SeeAlso  []string `yaml:"see_also,omitempty" json:"see_also,omitempty"`
	Tags     []string `yaml:"tags,omitempty" json:"tags,omitempty"`

	// File records where the rule was loaded from. It is useful when
	// reporting a broken rule.
	File string `yaml:"-" json:"file,omitempty"`
}

// Hit is a rule together with its match score.
type Hit struct {
	Rule    Rule     `json:"rule"`
	Score   int      `json:"score"`
	Matched []string `json:"matched"`
}

// Load reads every YAML document below root in fsys. A file may contain
// several documents separated by "---".
func Load(fsys fs.FS, root string) ([]Rule, error) {
	var rules []Rule
	origins := map[string]string{}

	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		dec := yaml.NewDecoder(bytes.NewReader(data))
		for {
			var r Rule
			err := dec.Decode(&r)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if r.ID == "" && r.Title == "" && len(r.Match) == 0 {
				continue // empty document, e.g. a trailing "---"
			}
			r.File = path
			if err := r.validate(); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if prev, ok := origins[r.ID]; ok {
				return fmt.Errorf("%s: duplicate rule id %q (already defined in %s)", path, r.ID, prev)
			}
			origins[r.ID] = path
			rules = append(rules, r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules, nil
}

func (r Rule) validate() error {
	if r.ID == "" {
		return errors.New("rule without id")
	}
	if r.Title == "" {
		return fmt.Errorf("rule %q: missing title", r.ID)
	}
	if r.Category == "" {
		return fmt.Errorf("rule %q: missing category", r.ID)
	}
	if len(r.Match) == 0 && len(r.MatchAll) == 0 {
		return fmt.Errorf("rule %q: no match patterns", r.ID)
	}
	if strings.TrimSpace(r.Cause) == "" {
		return fmt.Errorf("rule %q: missing cause", r.ID)
	}
	if strings.TrimSpace(r.Fix) == "" {
		return fmt.Errorf("rule %q: missing fix", r.ID)
	}
	return nil
}

// Find returns every rule that matches text, best match first.
//
// A rule matches when all of its match_all patterns are present and at least
// one of its match patterns is present. Patterns are matched as
// case-insensitive substrings against whitespace-normalised text.
func Find(rules []Rule, text string) []Hit {
	hay := normalize.ForMatch(text)
	hits := make([]Hit, 0, 2)
	for _, r := range rules {
		matched := make([]string, 0, len(r.Match)+len(r.MatchAll))
		ok := true
		for _, p := range r.MatchAll {
			np := normalize.ForMatch(p)
			if !strings.Contains(hay, np) {
				ok = false
				break
			}
			matched = append(matched, np)
		}
		if !ok {
			continue
		}
		any := len(r.Match) == 0
		for _, p := range r.Match {
			np := normalize.ForMatch(p)
			if strings.Contains(hay, np) {
				any = true
				matched = append(matched, np)
			}
		}
		if !any {
			continue
		}
		score := r.Priority*1000 + len(matched)*10
		for _, m := range matched {
			score += len(m)
		}
		hits = append(hits, Hit{Rule: r, Score: score, Matched: matched})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Rule.ID < hits[j].Rule.ID
	})
	return hits
}

// Filter returns the rules whose id, title, category or tags contain query.
func Filter(rules []Rule, query string) []Rule {
	if query == "" {
		return rules
	}
	q := strings.ToLower(query)
	var out []Rule
	for _, r := range rules {
		fields := []string{r.ID, r.Title, r.Category}
		fields = append(fields, r.Tags...)
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f), q) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
