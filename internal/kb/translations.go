package kb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

// Translation replaces the human-readable fields of a rule. Rules keep their
// English text in rules/*.yaml; translations live in i18n/<lang>.yaml.
type Translation struct {
	ID      string `yaml:"id"`
	Title   string `yaml:"title"`
	Cause   string `yaml:"cause"`
	Fix     string `yaml:"fix"`
	Example string `yaml:"example"`
}

// LoadTranslations reads i18n/<lang>.yaml from fsys. A file may contain
// several documents separated by "---".
func LoadTranslations(fsys fs.FS, root, lang string) (map[string]Translation, error) {
	path := root + "/" + lang + ".yaml"
	data, err := fs.ReadFile(fsys, path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]Translation{}, nil
	}
	if err != nil {
		return nil, err
	}

	out := make(map[string]Translation)
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var t Translation
		err := dec.Decode(&t)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if t.ID == "" {
			continue
		}
		if t.Title == "" || strings.TrimSpace(t.Cause) == "" || strings.TrimSpace(t.Fix) == "" {
			return nil, fmt.Errorf("%s: translation %q is incomplete (title, cause and fix are required)", path, t.ID)
		}
		if _, dup := out[t.ID]; dup {
			return nil, fmt.Errorf("%s: duplicate translation for %q", path, t.ID)
		}
		out[t.ID] = t
	}
	return out, nil
}

// Translate returns a copy of rules with translations applied. Rules without
// a translation keep their English text, so a partial translation never
// breaks the tool.
func Translate(rules []Rule, translations map[string]Translation) []Rule {
	out := make([]Rule, len(rules))
	copy(out, rules)
	for i := range out {
		t, ok := translations[out[i].ID]
		if !ok {
			continue
		}
		if t.Title != "" {
			out[i].Title = t.Title
		}
		if t.Cause != "" {
			out[i].Cause = strings.TrimRight(t.Cause, "\n")
		}
		if t.Fix != "" {
			out[i].Fix = strings.TrimRight(t.Fix, "\n")
		}
		if t.Example != "" {
			out[i].Example = strings.TrimRight(t.Example, "\n")
		}
	}
	return out
}
