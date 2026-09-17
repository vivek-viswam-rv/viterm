// Package theme defines viterm's color themes. A theme covers the
// application chrome (session strip, tab bars, footer) only; pane
// contents keep the hosting terminal's colors and font.
package theme

// Chrome colors style viterm's own UI elements. Values are hex strings.
type Chrome struct {
	Accent          string
	Dim             string
	Foreground      string
	StripBackground string
}

// Theme is a named color scheme.
type Theme struct {
	Name   string
	Chrome Chrome
}

// DefaultName is the fallback theme for unknown names and the base for
// color overrides set without a theme. With neither a theme nor overrides
// configured, the chrome uses ANSI palette colors instead, so it adapts to
// the hosting terminal's scheme.
const DefaultName = "midnight"

var themes = []Theme{
	{
		Name: "midnight",
		Chrome: Chrome{
			Accent:          "#7aa2f7",
			Dim:             "#565f89",
			Foreground:      "#c0caf5",
			StripBackground: "#292e42",
		},
	},
	{
		Name: "aurora",
		Chrome: Chrome{
			Accent:          "#4fd6be",
			Dim:             "#527a78",
			Foreground:      "#d8e8e6",
			StripBackground: "#16302f",
		},
	},
	{
		Name: "ember",
		Chrome: Chrome{
			Accent:          "#ff9e64",
			Dim:             "#8a6a58",
			Foreground:      "#f2e5d5",
			StripBackground: "#3a2820",
		},
	},
	{
		Name: "paper",
		Chrome: Chrome{
			Accent:          "#2a6fbb",
			Dim:             "#8f8a80",
			Foreground:      "#3b3835",
			StripBackground: "#e8e2d4",
		},
	},
}

// Get returns the named theme, falling back to the default for unknown or
// empty names. Names are matched case-insensitively.
func Get(name string) Theme {
	for _, t := range themes {
		if equalFold(t.Name, name) {
			return t
		}
	}
	for _, t := range themes {
		if t.Name == DefaultName {
			return t
		}
	}
	return themes[0]
}

// Known reports whether name matches a built-in theme.
func Known(name string) bool {
	for _, t := range themes {
		if equalFold(t.Name, name) {
			return true
		}
	}
	return false
}

// Names lists the built-in themes.
func Names() []string {
	out := make([]string, len(themes))
	for i, t := range themes {
		out[i] = t.Name
	}
	return out
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
