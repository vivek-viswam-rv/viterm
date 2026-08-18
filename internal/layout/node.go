// Package layout parses viterm's pane layout language: a small line-based
// format describing how a session's window splits into panes and what each
// pane runs. Structure is determined by key order; indentation is accepted
// for readability but carries no meaning.
//
// Keys (case-insensitive):
//
//	run: CMD        a terminal pane running CMD (empty CMD is a plain shell)
//	visit: URL      a link pane that opens URL in the browser
//	split: columns  a two-way split; any other value splits into rows
//	left|right|top|bottom:   optional labels introducing a split's sides
//	size: N%        optional size for the side that follows
//	tabs:           a pane whose tabs are the run/visit lines that follow
package layout

// Kind discriminates node types.
type Kind int

const (
	// KindRun is a terminal leaf.
	KindRun Kind = iota
	// KindVisit is a link leaf.
	KindVisit
	// KindSplit is a two-way split.
	KindSplit
	// KindTabs is a pane holding multiple leaves as tabs.
	KindTabs
)

// Dir is a split's orientation.
type Dir int

const (
	// DirColumns places children side by side.
	DirColumns Dir = iota
	// DirRows stacks children.
	DirRows
)

// Node is one element of a parsed layout tree.
type Node struct {
	Kind    Kind
	Command string // KindRun; empty means a plain shell
	URL     string // KindVisit
	Dir     Dir    // KindSplit

	First      *Node // KindSplit
	Second     *Node
	FirstSize  *float64 // fraction of the split, nil when unspecified
	SecondSize *float64

	Children []*Node // KindTabs; KindRun and KindVisit leaves only
}

// Ratio resolves the first side's share of a split: the first side's
// explicit size, else the complement of the second side's, else an even
// split, clamped to [0.05, 0.95].
func (n *Node) Ratio() float64 {
	r := 0.5
	switch {
	case n.FirstSize != nil:
		r = *n.FirstSize
	case n.SecondSize != nil:
		r = 1 - *n.SecondSize
	}
	if r < 0.05 {
		r = 0.05
	}
	if r > 0.95 {
		r = 0.95
	}
	return r
}

// Walk visits every node in preorder.
func Walk(n *Node, fn func(*Node)) {
	if n == nil {
		return
	}
	fn(n)
	Walk(n.First, fn)
	Walk(n.Second, fn)
	for _, c := range n.Children {
		Walk(c, fn)
	}
}

// Leaves returns the run and visit leaves in document order. Tabs nodes
// contribute their children.
func (n *Node) Leaves() []*Node {
	var out []*Node
	Walk(n, func(m *Node) {
		if m.Kind == KindRun || m.Kind == KindVisit {
			out = append(out, m)
		}
	})
	return out
}

// RewriteRunCommands applies fn to every run leaf's command in place.
func RewriteRunCommands(n *Node, fn func(string) string) {
	Walk(n, func(m *Node) {
		if m.Kind == KindRun {
			m.Command = fn(m.Command)
		}
	})
}

// DefaultLayoutText is the layout used when a repo has none configured.
const DefaultLayoutText = `split: columns
left:
  size: 60%
  run: claude
right:
  run:
`
