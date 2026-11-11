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
	"github.com/nepoet/nix-explain/internal/istty"
	"github.com/nepoet/nix-explain/internal/kb"
	"github.com/nepoet/nix-explain/internal/render"
	"github.com/nepoet/nix-explain/internal/state"
	"github.com/nepoet/nix-explain/internal/trace"
)

// maxCaptureBytes bounds how much of a wrapped command's output is kept for
// analysis. Nix prints the interesting part of a failure at the very end.
const maxCaptureBytes = 8 << 20

type options struct {
	json      bool
	color     string
	showAll   bool
	lang      string
	width     int
	last      bool
	clipboard bool
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
	fs.IntVar(&opts.width, "width", 0, "wrap text to this many columns (default 72)")
	fs.BoolVar(&opts.showAll, "show-all", false, "keep nixpkgs frames in the trace")
	fs.BoolVar(&opts.last, "last", false, "explain the last error captured for this terminal")
	fs.BoolVar(&opts.clipboard, "clipboard", false, "explain the text in the clipboard")
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
	flagArgs, rest := splitFlags(args)
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	strs := i18n.Get(i18n.Resolve(opts.lang))

	switch {
	case opts.clipboard:
		text, err := readClipboard()
		if err != nil {
			fmt.Fprintln(stderr, strs.ClipboardErr)
			return 2
		}
		return explain(text, stdout, rules, translations, opts)

	case opts.last && len(rest) == 0:
		text, err := state.Load()
		if err != nil {
			fmt.Fprintln(stderr, strs.NoLastError)
			return 2
		}
		return explain(text, stdout, rules, translations, opts)
	}

	if len(rest) > 0 {
		text, err := readInput(rest, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "nix-explain: %v\n", err)
			return 2
		}
		if strings.TrimSpace(text) == "" {
			fmt.Fprint(stderr, strs.Usage)
			return 2
		}
		return explain(text, stdout, rules, translations, opts)
	}

	// No arguments. Piped input keeps working exactly as before.
	if !isTerminal(stdin) {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "nix-explain: %v\n", err)
			return 2
		}
		text := string(raw)
		if strings.TrimSpace(text) == "" {
			fmt.Fprint(stderr, strs.Usage)
			return 2
		}
		return explain(text, stdout, rules, translations, opts)
	}

	// On a terminal: explain what the shim captured for this terminal, else
	// ask the user to paste it.
	if text, err := state.Load(); err == nil {
		return explain(text, stdout, rules, translations, opts)
	}

	fmt.Fprintln(stderr, strs.PastePrompt)
	raw, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "nix-explain: %v\n", err)
		return 2
	}
	text := string(raw)
	if strings.TrimSpace(text) == "" {
		fmt.Fprint(stderr, strs.Usage)
		return 2
	}
	return explain(text, stdout, rules, translations, opts)
}

// splitFlags moves known flags in front of positional arguments, so both
// `nix-explain err.log --color never` and `nix-explain --color never err.log`
// work. Unknown flags are treated as text: error output often contains them.
func splitFlags(args []string) (flagArgs, positional []string) {
	boolFlags := map[string]bool{
		"json": true, "show-all": true, "last": true, "clipboard": true,
	}
	valueFlags := map[string]bool{
		"color": true, "lang": true, "width": true,
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "--") {
			positional = append(positional, a)
			continue
		}
		name := strings.TrimPrefix(a, "--")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			if boolFlags[name[:eq]] || valueFlags[name[:eq]] {
				flagArgs = append(flagArgs, a)
				continue
			}
			positional = append(positional, a)
			continue
		}
		switch {
		case boolFlags[name]:
			flagArgs = append(flagArgs, a)
		case valueFlags[name] && i+1 < len(args):
			flagArgs = append(flagArgs, a, args[i+1])
			i++
		default:
			positional = append(positional, a)
		}
	}
	return flagArgs, positional
}

// readClipboard returns the text in the system clipboard, if a supported
// clipboard tool is installed.
func readClipboard() (string, error) {
	candidates := [][]string{
		{"wl-paste", "--no-newline"},
		{"xclip", "-selection", "clipboard", "-o"},
		{"xsel", "--clipboard", "--output"},
		{"pbpaste"},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		out, err := exec.Command(c[0], c[1:]...).Output()
		if err != nil {
			continue
		}
		if text := strings.TrimSpace(string(out)); text != "" {
			return text, nil
		}
	}
	return "", errors.New("no clipboard tool available")
}

// isTerminal reports whether r is an interactive terminal.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return istty.Is(f.Fd())
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
		Width:   opts.width,
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
	pre, extra := splitFlags(flagArgs)
	if len(extra) > 0 {
		fmt.Fprint(stderr, "nix-explain run: put the command after --\n\n")
		fmt.Fprint(stderr, i18n.Get(i18n.Resolve(opts.lang)).Usage)
		return 2
	}
	if err := fs.Parse(pre); err != nil {
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

	text := captured.String()
	if strings.TrimSpace(text) != "" {
		_ = state.Save(text)
	}

	// In run mode the explanation goes to stderr so that stdout stays a
	// faithful copy of the command's own output.
	explainTo := stderr
	if opts.json {
		explainTo = stdout
	}
	explainCaptured(text, explainTo, rules, translations, opts)
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
		Width:   opts.width,
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
	fs.IntVar(&opts.width, "width", 0, "wrap text to this many columns (default 72)")
	fs.Usage = func() {
		fmt.Fprint(stderr, i18n.Get(i18n.Resolve(opts.lang)).Usage)
	}
	flagArgs, queryArgs := splitFlags(args)
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	query := strings.Join(queryArgs, " ")
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
		Width:   opts.width,
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
