// Package trace extracts evaluation traces from Nix error output.
//
// Nix prints traces in a format that mixes user files with thousands of
// frames from nixpkgs in /nix/store. The frames that matter to the user are
// the ones pointing at their own files, and those are usually buried.
package trace

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/nepoet/nix-explain/internal/normalize"
)

// Frame is a single "at file:line:col" entry from a Nix trace.
type Frame struct {
	Description string `json:"description,omitempty"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Col         int    `json:"col"`
	Internal    bool   `json:"internal"`
}

// Trace is an ordered list of frames, outermost first.
type Trace struct {
	Frames []Frame `json:"frames"`
}

// atLine matches "at /path/to/file.nix:12:5:" lines. The path is greedy
// because it may itself contain colons (for example «https://...»).
var atLine = regexp.MustCompile(`^\s*at\s+(.+):(\d+):(\d+):?\s*$`)

// Parse extracts the frames from raw Nix output.
func Parse(text string) Trace {
	var t Trace
	desc := ""
	for _, line := range strings.Split(normalize.Text(text), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "…") || strings.HasPrefix(trimmed, "...") {
			desc = trimmed
			continue
		}
		m := atLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNo, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		f := Frame{
			Description: desc,
			File:        m[1],
			Line:        lineNo,
			Col:         col,
		}
		f.Internal = isInternal(f.File)
		desc = ""
		if n := len(t.Frames); n > 0 {
			prev := t.Frames[n-1]
			if prev.File == f.File && prev.Line == f.Line && prev.Col == f.Col {
				continue
			}
		}
		t.Frames = append(t.Frames, f)
	}
	return t
}

// isInternal reports whether a frame belongs to nixpkgs, the Nix
// implementation or a remote source, rather than to the user's code.
func isInternal(file string) bool {
	switch {
	case strings.HasPrefix(file, "/nix/store/"):
		return true
	case strings.HasPrefix(file, "«nix-internal»"):
		return true
	case strings.HasPrefix(file, "«https://"), strings.HasPrefix(file, "«http://"):
		return true
	case strings.HasPrefix(file, "«flake»"):
		return true
	}
	return false
}

// Visible returns the frames that point at the user's own code.
func (t Trace) Visible() []Frame {
	var out []Frame
	for _, f := range t.Frames {
		if !f.Internal {
			out = append(out, f)
		}
	}
	return out
}

// Hidden returns the number of frames that were filtered out.
func (t Trace) Hidden() int {
	return len(t.Frames) - len(t.Visible())
}

// Location returns the innermost user frame that refers to a real file.
// Frames from «string» or other synthetic sources are skipped because the
// user cannot open them.
func (t Trace) Location() (Frame, bool) {
	frames := t.Visible()
	for i := len(frames) - 1; i >= 0; i-- {
		f := frames[i]
		if strings.HasPrefix(f.File, "«") {
			continue
		}
		return f, true
	}
	return Frame{}, false
}
