// Package render prints explanations for humans.
package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/nepoet/nix-explain/internal/kb"
	"github.com/nepoet/nix-explain/internal/trace"
)

// Options controls the presentation of an explanation.
type Options struct {
	Color   bool
	ShowAll bool
}

type painter struct{ on bool }

func (p painter) wrap(code, s string) string {
	if !p.on || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (p painter) bold(s string) string   { return p.wrap("1", s) }
func (p painter) dim(s string) string    { return p.wrap("2", s) }
func (p painter) green(s string) string  { return p.wrap("32", s) }
func (p painter) yellow(s string) string { return p.wrap("33", s) }
func (p painter) cyan(s string) string   { return p.wrap("36", s) }

const ruleWidth = 72

// Explain writes a full explanation for hit.
func Explain(w io.Writer, hit kb.Hit, tr trace.Trace, opts Options) {
	p := painter{opts.Color}
	r := hit.Rule

	fmt.Fprintln(w, p.bold(r.Title))
	fmt.Fprintln(w, p.dim(strings.Repeat("─", ruleWidth)))
	fmt.Fprintln(w, p.dim(fmt.Sprintf("%s · %s", r.Category, r.ID)))
	fmt.Fprintln(w)

	section(w, p, "CAUSE", r.Cause)
	section(w, p, "FIX", r.Fix)

	if r.Example != "" {
		fmt.Fprintln(w, p.cyan("EXAMPLE"))
		for _, line := range strings.Split(strings.TrimRight(r.Example, "\n"), "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
		fmt.Fprintln(w)
	}

	if len(r.SeeAlso) > 0 {
		fmt.Fprintln(w, p.cyan("SEE ALSO"))
		for _, u := range r.SeeAlso {
			fmt.Fprintf(w, "  %s\n", p.dim(u))
		}
		fmt.Fprintln(w)
	}

	renderTrace(w, p, tr, opts)
}

func section(w io.Writer, p painter, title, body string) {
	fmt.Fprintln(w, p.cyan(title))
	for _, line := range wrap(strings.TrimRight(body, "\n"), ruleWidth) {
		fmt.Fprintf(w, "  %s\n", line)
	}
	fmt.Fprintln(w)
}

func renderTrace(w io.Writer, p painter, tr trace.Trace, opts Options) {
	if len(tr.Frames) == 0 {
		return
	}
	fmt.Fprintln(w, p.cyan("TRACE"))

	loc, hasLoc := tr.Location()
	if hasLoc {
		fmt.Fprintf(w, "  %s %s\n", p.green("→"), p.bold(fmt.Sprintf("%s:%d:%d", loc.File, loc.Line, loc.Col)))
		if loc.Description != "" {
			fmt.Fprintf(w, "    %s\n", p.dim(loc.Description))
		}
	}

	frames := tr.Frames
	if !opts.ShowAll {
		frames = tr.Visible()
	}
	for _, f := range frames {
		if hasLoc && f.File == loc.File && f.Line == loc.Line && f.Col == loc.Col {
			continue // already shown as the primary location
		}
		desc := f.Description
		if desc == "" {
			desc = "at"
		}
		line := fmt.Sprintf("  %s %s (%s:%d:%d)", p.dim("·"), desc, f.File, f.Line, f.Col)
		if f.Internal {
			line = p.dim(line)
		}
		fmt.Fprintln(w, line)
	}

	if hidden := tr.Hidden(); hidden > 0 && !opts.ShowAll {
		noun := "frames"
		if hidden == 1 {
			noun = "frame"
		}
		fmt.Fprintf(w, "  %s\n", p.dim(fmt.Sprintf("(%d %s from nixpkgs hidden — use --show-all to see them)", hidden, noun)))
	}
	fmt.Fprintln(w)
}

// NoMatch explains that nothing in the knowledge base matched.
func NoMatch(w io.Writer, text string, tr trace.Trace, opts Options) {
	p := painter{opts.Color}

	fmt.Fprintln(w, p.bold(p.yellow("No known explanation for this output.")))
	fmt.Fprintln(w, p.dim(strings.Repeat("─", ruleWidth)))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "The error was not recognised. A few things you can do:")
	fmt.Fprintf(w, "  - %s                 list every known error\n", p.cyan("nix-explain list"))
	fmt.Fprintf(w, "  - %s       show the full trace\n", p.cyan("nix-explain --show-all"))
	fmt.Fprintf(w, "  - %s  ask Nix for more detail\n", p.cyan("nix <command> --show-trace"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, p.dim("If this is a common error, please open an issue with the output:"))
	fmt.Fprintln(w, p.dim("  https://github.com/nepoet/nix-explain/issues"))
	fmt.Fprintln(w)

	renderTrace(w, p, tr, opts)
}

// alsoMaybe lists lower-ranked rules that also matched.
func AlsoMaybe(w io.Writer, hits []kb.Hit, opts Options) {
	if len(hits) == 0 {
		return
	}
	p := painter{opts.Color}
	if len(hits) > 3 {
		hits = hits[:3]
	}
	fmt.Fprintln(w, p.dim("Also matched:"))
	for _, h := range hits {
		fmt.Fprintf(w, "  %s\n", p.dim(fmt.Sprintf("%s  %s", h.Rule.ID, h.Rule.Title)))
	}
	fmt.Fprintln(w)
}

// List prints the knowledge base as a table grouped by category.
func List(w io.Writer, rules []kb.Rule, opts Options) {
	p := painter{opts.Color}
	category := ""
	for _, r := range rules {
		if r.Category != category {
			if category != "" {
				fmt.Fprintln(w)
			}
			category = r.Category
			fmt.Fprintln(w, p.cyan(strings.ToUpper(category)))
		}
		fmt.Fprintf(w, "  %s %s\n", p.green(fmt.Sprintf("%-34s", r.ID)), r.Title)
	}
}

// wrap word-wraps body to width. Rule files are written as soft-wrapped
// prose, so consecutive lines with the same indentation are joined into one
// paragraph before wrapping; lists and indented examples keep their breaks.
func wrap(body string, width int) []string {
	var out []string
	var buf string
	bufIndent := ""

	flush := func() {
		if buf == "" {
			return
		}
		out = append(out, wrapLine(bufIndent+buf, width)...)
		buf = ""
	}

	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			out = append(out, "")
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
		if buf == "" {
			buf, bufIndent = trimmed, indent
			continue
		}
		if indent != bufIndent || endsSentence(buf) || isListItem(buf) {
			flush()
			buf, bufIndent = trimmed, indent
			continue
		}
		buf += " " + trimmed
	}
	flush()

	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// endsSentence reports whether a soft-wrapped line ends a thought.
func endsSentence(s string) bool {
	if s == "" {
		return true
	}
	switch s[len(s)-1] {
	case '.', ':', '!', '?', ';':
		return true
	}
	return false
}

func isListItem(s string) bool {
	if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") {
		return true
	}
	for i := 0; i < len(s) && i < 3; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
		if i+1 < len(s) && s[i+1] == '.' {
			return true
		}
	}
	return false
}

// wrapLine wraps one logical line, preserving its indentation.
func wrapLine(text string, width int) []string {
	trimmed := strings.TrimSpace(text)
	indent := text[:len(text)-len(strings.TrimLeft(text, " "))]
	if len(text) <= width {
		return []string{indent + trimmed}
	}
	var out []string
	line := ""
	for _, word := range strings.Fields(trimmed) {
		switch {
		case line == "":
			line = word
		case len(indent)+len(line)+1+len(word) > width:
			out = append(out, indent+line)
			line = word
		default:
			line += " " + word
		}
	}
	if line != "" {
		out = append(out, indent+line)
	}
	return out
}
