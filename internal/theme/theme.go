// Package theme defines viterm's color themes. A theme covers both the
// application chrome (session strip, tab bars, footer) and, for the windowed
// app host, the terminal surface itself.
package theme

// Chrome colors style viterm's own UI elements. Values are hex strings.
type Chrome struct {
	Accent          string
	Dim             string
	Foreground      string
	StripBackground string
}

// Terminal colors style the terminal surface of the windowed app host.
type Terminal struct {
	Background string
	Foreground string
	Cursor     string
	Selection  string
	ANSI       [16]string
}

// Theme is a named color scheme.
type Theme struct {
	Name     string
	Chrome   Chrome
	Terminal Terminal
}

// DefaultName is the theme used when none is configured. In a plain
// terminal the chrome additionally falls back to ANSI palette colors so it
// adapts to the hosting terminal's scheme.
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
		Terminal: Terminal{
			Background: "#1a1b26",
			Foreground: "#c0caf5",
			Cursor:     "#c0caf5",
			Selection:  "#33467c",
			ANSI: [16]string{
				"#15161e", "#f7768e", "#9ece6a", "#e0af68",
				"#7aa2f7", "#bb9af7", "#7dcfff", "#a9b1d6",
				"#414868", "#ff899d", "#9fe044", "#faba4a",
				"#8db0ff", "#c7a9ff", "#a4daff", "#c0caf5",
			},
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
		Terminal: Terminal{
			Background: "#0e1a19",
			Foreground: "#d8e8e6",
			Cursor:     "#4fd6be",
			Selection:  "#1f403d",
			ANSI: [16]string{
				"#0a1414", "#e67e80", "#a7c080", "#dbbc7f",
				"#7fbbb3", "#d699b6", "#83c092", "#c5d0cd",
				"#425c5a", "#f28b8d", "#b8d493", "#eccc8e",
				"#93cec6", "#e2aac4", "#96d3a5", "#e6f0ee",
			},
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
		Terminal: Terminal{
			Background: "#1f1410",
			Foreground: "#f2e5d5",
			Cursor:     "#ff9e64",
			Selection:  "#4a3327",
			ANSI: [16]string{
				"#181008", "#e06c60", "#a3b86c", "#e5c07b",
				"#d19a66", "#c678dd", "#56b6c2", "#d8c8b8",
				"#5c4a3d", "#ef8378", "#b5ca80", "#f0d197",
				"#e0ab7c", "#d78fe8", "#6cc7d3", "#f7ede1",
			},
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
		Terminal: Terminal{
			Background: "#f7f3e9",
			Foreground: "#3b3835",
			Cursor:     "#2a6fbb",
			Selection:  "#d5e3f5",
			ANSI: [16]string{
				"#3b3835", "#c14a4a", "#5a7d3c", "#a8760f",
				"#2a6fbb", "#8f4d9e", "#25808a", "#e8e2d4",
				"#78736a", "#d9534f", "#6f9a4a", "#c79121",
				"#4a8ad4", "#a86bb8", "#3499a4", "#fbf8f1",
			},
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
