// Package cli implements the nix-explain command line interface.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/nepoet/nix-explain/internal/i18n"
	"github.com/nepoet/nix-explain/internal/kb"
	"github.com/nepoet/nix-explain/internal/render"
	"github.com/nepoet/nix-explain/internal/trace"
)

// maxCaptureBytes bounds how much of a wrapped command's output is kept for
// analysis. Nix prints the interesting part of a failure at the very end.
const maxCaptureBytes = 8 << 20

type options struct {
	json    bool
	color   string
	showAll bool
	lang    string
}

// Run dispatches to a subcommand and returns the process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, rules []kb.Rule, translations map[string]kb.Translation, version string) int {
	if len(args) > 0 {
		switch args[0] {
		case "run":
			return runCommand(args[1:], stdin, stdout, stderr, rules, translations)
		case "list":
			return listCommand(args[1:], stdout, stderr, rules, translations)
		case "version", "--version", "-v":
			fmt.Fprintf(stdout, "nix-explain %s\n", version)
			return 0
		case "help", "--help", "-h":
			fmt.Fprint(stdout, i18n.Get(i18n.Resolve("")).Usage)
			return 0
		}
	}
	return explainCommand(args, stdin, stdout, stderr, rules, translations)
}

func newFlagSet(name string, stderr io.Writer, opts *options) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.json, "json", false, "print the matched rule as JSON")
	fs.StringVar(&opts.color, "color", "auto", "color output: auto, always or never")
	fs.StringVar(&opts.lang, "lang", "auto", "output language: en, ru, auto")
	fs.BoolVar(&opts.showAll, "show-all", false, "keep nixpkgs frames in the trace")
	fs.Usage = func() {
		fmt.Fprint(stderr, i18n.Get(i18n.Resolve(opts.lang)).Usage)
	}
	return fs
}

// localize resolves the language and returns translated rules plus the
// matching interface strings.
func localize(rules []kb.Rule, translations map[string]kb.Translation, lang string) ([]kb.Rule, i18n.Strings) {
	resolved := i18n.Resolve(lang)
	if resolved == "ru" && len(translations) > 0 {
		return kb.Translate(rules, translations), i18n.Get(resolved)
	}
	return rules, i18n.Get(resolved)
}

func explainCommand(args []string, stdin io.Reader, stdout, stderr io.Writer, rules []kb.Rule, translations map[string]kb.Translation) int {
	var opts options
	fs := newFlagSet("nix-explain", stderr, &opts)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	rest := fs.Args()
	if len(rest) == 0 && isTerminal(stdin) {
		// Nothing to explain and no piped input: do not wait forever.
		fmt.Fprint(stderr, i18n.Get(i18n.Resolve(opts.lang)).Usage)
		return 2
	}

	text, err := readInput(rest, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "nix-explain: %v\n", err)
		return 2
	}
	if strings.TrimSpace(text) == "" {
		fmt.Fprint(stderr, i18n.Get(i18n.Resolve(opts.lang)).Usage)
		return 2
	}
	return explain(text, stdout, rules, translations, opts)
}

// isTerminal reports whether r is an interactive terminal. It is used to avoid
// blocking on stdin when the command is invoked without arguments.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func readInput(args []string, stdin io.Reader) (string, error) {
	switch {
	case len(args) == 0:
		b, err := io.ReadAll(stdin)
		return string(b), err
	case len(args) == 1:
		if args[0] == "-" {
			b, err := io.ReadAll(stdin)
			return string(b), err
		}
		if isFile(args[0]) {
			b, err := os.ReadFile(args[0])
			return string(b), err
		}
		return args[0], nil
	default:
		return strings.Join(args, " "), nil
	}
}

func isFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// explain prints the best explanation and returns 0, or a "no match" report
// and returns 2.
func explain(text string, stdout io.Writer, rules []kb.Rule, translations map[string]kb.Translation, opts options) int {
	rules, strs := localize(rules, translations, opts.lang)
	tr := trace.Parse(text)
	hits := kb.Find(rules, text)
	renderOpts := render.Options{
		Color:   colorEnabled(opts.color, stdout),
		ShowAll: opts.showAll,
		Strings: strs,
	}

	if opts.json {
		writeJSON(stdout, hits, tr, opts.showAll, strs.Lang)
		if len(hits) == 0 {
			return 2
		}
		return 0
	}

	if len(hits) == 0 {
		render.NoMatch(stdout, text, tr, renderOpts)
		return 2
	}

	render.Explain(stdout, hits[0], tr, renderOpts)
	if len(hits) > 1 {
		render.AlsoMaybe(stdout, hits[1:], renderOpts)
	}
	return 0
}

func runCommand(args []string, stdin io.Reader, stdout, stderr io.Writer, rules []kb.Rule, translations map[string]kb.Translation) int {
	var opts options
	flagArgs, cmdArgs := splitOnDashDash(args)

	fs := newFlagSet("nix-explain run", stderr, &opts)
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if len(cmdArgs) == 0 {
		fmt.Fprint(stderr, "nix-explain run: missing command after --\n\n")
		fmt.Fprint(stderr, i18n.Get(i18n.Resolve(opts.lang)).Usage)
		return 2
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	captured := newTailBuffer(maxCaptureBytes)
	cmd.Stdin = stdin
	cmd.Stdout = io.MultiWriter(stdout, captured)
	cmd.Stderr = io.MultiWriter(stderr, captured)

	err := cmd.Run()
	if err == nil {
		return 0
	}

	code := 1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if c := exitErr.ExitCode(); c > 0 {
			code = c
		}
	}

	// In run mode the explanation goes to stderr so that stdout stays a
	// faithful copy of the command's own output.
	explainTo := stderr
	if opts.json {
		explainTo = stdout
	}
	explainCaptured(captured.String(), explainTo, rules, translations, opts)
	return code
}

func explainCaptured(text string, stdout io.Writer, rules []kb.Rule, translations map[string]kb.Translation, opts options) {
	rules, strs := localize(rules, translations, opts.lang)
	tr := trace.Parse(text)
	hits := kb.Find(rules, text)
	if opts.json {
		writeJSON(stdout, hits, tr, opts.showAll, strs.Lang)
		return
	}
	renderOpts := render.Options{
		Color:   colorEnabled(opts.color, stdout),
		ShowAll: opts.showAll,
		Strings: strs,
	}
	if len(hits) == 0 {
		render.NoMatch(stdout, text, tr, renderOpts)
		return
	}
	render.Explain(stdout, hits[0], tr, renderOpts)
	if len(hits) > 1 {
		render.AlsoMaybe(stdout, hits[1:], renderOpts)
	}
}

func splitOnDashDash(args []string) (flags, command []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func listCommand(args []string, stdout, stderr io.Writer, rules []kb.Rule, translations map[string]kb.Translation) int {
	var opts options
	fs := flag.NewFlagSet("nix-explain list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.json, "json", false, "print the knowledge base as JSON")
	fs.StringVar(&opts.color, "color", "auto", "color output: auto, always or never")
	fs.StringVar(&opts.lang, "lang", "auto", "output language: en, ru, auto")
	fs.Usage = func() {
		fmt.Fprint(stderr, i18n.Get(i18n.Resolve(opts.lang)).Usage)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	query := strings.Join(fs.Args(), " ")
	rules, strs := localize(rules, translations, opts.lang)
	selected := kb.Filter(rules, query)

	if opts.json {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(selected); err != nil {
			fmt.Fprintf(stderr, "nix-explain: %v\n", err)
			return 2
		}
		return 0
	}
	if len(selected) == 0 {
		fmt.Fprintf(stdout, "No rules match %q.\n", query)
		return 2
	}
	render.List(stdout, selected, render.Options{
		Color:   colorEnabled(opts.color, stdout),
		Strings: strs,
	})
	return 0
}

type jsonOutput struct {
	Matched      bool          `json:"matched"`
	Lang         string        `json:"lang"`
	Rule         *kb.Rule      `json:"rule,omitempty"`
	Score        int           `json:"score,omitempty"`
	Location     *trace.Frame  `json:"location,omitempty"`
	Frames       []trace.Frame `json:"frames"`
	HiddenFrames int           `json:"hidden_frames"`
	Alternatives []string      `json:"alternatives,omitempty"`
}

func writeJSON(w io.Writer, hits []kb.Hit, tr trace.Trace, showAll bool, lang string) {
	out := jsonOutput{Lang: lang}
	if !showAll {
		out.HiddenFrames = tr.Hidden()
	}

	frames := tr.Visible()
	if showAll {
		frames = tr.Frames
	}
	if frames == nil {
		frames = []trace.Frame{}
	}
	out.Frames = frames

	if loc, ok := tr.Location(); ok {
		out.Location = &loc
	}
	if len(hits) > 0 {
		out.Matched = true
		out.Rule = &hits[0].Rule
		out.Score = hits[0].Score
		for _, h := range hits[1:] {
			out.Alternatives = append(out.Alternatives, h.Rule.ID)
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func colorEnabled(mode string, w io.Writer) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
