// Package normalize prepares command output for analysis and for matching
// against the knowledge base.
package normalize

import (
	"regexp"
	"strings"
)

// ansi matches CSI escape sequences (colors, cursor movement and friends).
var ansi = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

// Text returns s with ANSI escape sequences removed and line endings
// normalised to "\n". Use this for anything that will be shown to the user.
func Text(s string) string {
	s = ansi.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// ForMatch reduces s to a form suitable for substring matching: lower case
// with every whitespace run collapsed to a single space. Rules are matched
// against this representation, so patterns spanning line breaks still work.
func ForMatch(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(Text(s)), " "))
}
