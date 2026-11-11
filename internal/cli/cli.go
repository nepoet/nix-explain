// Package cli implements the nix-explain command line interface.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/nepoet/nix-explain/internal/kb"
	"github.com/nepoet/nix-explain/internal/render"
	"github.com/nepoet/nix-explain/internal/trace"
)

const usage = `nix-explain — explain Nix error messages, offline.

Usage:
  nix-explain [flags] [text|file]      explain an error from a file or argument
  nix-explain < error.log              explain an error from stdin
  nix-explain run [flags] -- CMD...    run a command and explain if it fails
  nix-explain list [query]             list known errors
  nix-explain version                  print the version

Flags (explain):
  --json          print the matched rule as JSON
  --color MODE    auto, always or never (default auto)
  --show-all      keep nixpkgs frames in the trace

Flags (run):
  --json          print the matched rule as JSON
  --color MODE    auto, always or never (default auto)
  --show-all      keep nixpkgs frames in the trace

Exit codes (explain): 0 matched, 2 not recognised.
Exit codes (run): the exit code of CMD.

Examples:
  nix build .#foo 2>&1 | nix-explain
  nix-explain run -- nixos-rebuild switch
  nix-explain "error: infinite recursion encountered"
`

type options struct {
	json    bool
	color   string
	showAll bool
}

// Run dispatches to a subcommand and returns the process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, rules []kb.Rule, version string) int {
	if len(args) > 0 {
		switch args[0] {
		case "run":
			return runCommand(args[1:], stdin, stdout, stderr, rules)
		case "list":
			return listCommand(args[1:], stdout, stderr, rules)
		case "version", "--version", "-v":
			fmt.Fprintf(stdout, "nix-explain %s\n", version)
			return 0
		case "help", "--help", "-h":
			fmt.Fprint(stdout, usage)
			return 0
		}
	}
	return explainCommand(args, stdin, stdout, stderr, rules)
}

func newFlagSet(name string, stderr io.Writer, opts *options) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.json, "json", false, "print the matched rule as JSON")
	fs.StringVar(&opts.color, "color", "auto", "color output: auto, always or never")
	fs.BoolVar(&opts.showAll, "show-all", false, "keep nixpkgs frames in the trace")
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	return fs
}

func explainCommand(args []string, stdin io.Reader, stdout, stderr io.Writer, rules []kb.Rule) int {
	var opts options
	fs := newFlagSet("nix-explain", stderr, &opts)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	text, err := readInput(fs.Args(), stdin)
	if err != nil {
		fmt.Fprintf(stderr, "nix-explain: %v\n", err)
		return 2
	}
	if strings.TrimSpace(text) == "" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	return explain(text, stdout, stderr, rules, opts)
}

func readInput(args []string, stdin io.Reader) (string, error) {
	switch {
	case len(args) == 0:
		b, err := io.ReadAll(stdin)
		return string(b), err
	case len(args) == 1:
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
func explain(text string, stdout, stderr io.Writer, rules []kb.Rule, opts options) int {
	tr := trace.Parse(text)
	hits := kb.Find(rules, text)
	renderOpts := render.Options{
		Color:   colorEnabled(opts.color, stdout),
		ShowAll: opts.showAll,
	}

	if opts.json {
		writeJSON(stdout, hits, tr, opts.showAll)
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

func runCommand(args []string, stdin io.Reader, stdout, stderr io.Writer, rules []kb.Rule) int {
	var opts options
	flagArgs, cmdArgs := splitOnDashDash(args)

	fs := newFlagSet("nix-explain run", stderr, &opts)
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if len(cmdArgs) == 0 {
		fmt.Fprint(stderr, "nix-explain run: missing command after --\n\n")
		fmt.Fprint(stderr, usage)
		return 2
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	var captured bytes.Buffer
	cmd.Stdin = stdin
	cmd.Stdout = io.MultiWriter(stdout, &captured)
	cmd.Stderr = io.MultiWriter(stderr, &captured)

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
	explainCaptured(captured.String(), explainTo, stderr, rules, opts)
	return code
}

func explainCaptured(text string, stdout, stderr io.Writer, rules []kb.Rule, opts options) {
	tr := trace.Parse(text)
	hits := kb.Find(rules, text)
	if opts.json {
		writeJSON(stdout, hits, tr, opts.showAll)
		return
	}
	renderOpts := render.Options{
		Color:   colorEnabled(opts.color, stdout),
		ShowAll: opts.showAll,
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

func listCommand(args []string, stdout, stderr io.Writer, rules []kb.Rule) int {
	var opts options
	fs := flag.NewFlagSet("nix-explain list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.json, "json", false, "print the knowledge base as JSON")
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return 2
	}

	query := strings.Join(fs.Args(), " ")
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
	render.List(stdout, selected, render.Options{Color: colorEnabled(opts.color, stdout)})
	return 0
}

type jsonOutput struct {
	Matched      bool          `json:"matched"`
	Rule         *kb.Rule      `json:"rule,omitempty"`
	Score        int           `json:"score,omitempty"`
	Location     *trace.Frame  `json:"location,omitempty"`
	Frames       []trace.Frame `json:"frames"`
	HiddenFrames int           `json:"hidden_frames"`
	Alternatives []string      `json:"alternatives,omitempty"`
}

func writeJSON(w io.Writer, hits []kb.Hit, tr trace.Trace, showAll bool) {
	out := jsonOutput{HiddenFrames: tr.Hidden()}

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
