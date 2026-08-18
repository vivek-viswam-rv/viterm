package layout

import (
	"strconv"
	"strings"
)

// line is one meaningful input line. Indentation is recorded for
// completeness but the grammar is order-driven and never consults it.
type line struct {
	indent int
	key    string
	value  string
}

// Parse builds a layout tree. Empty or whitespace-only input yields a nil
// node and nil error. Trailing lines after a complete top-level node are
// ignored.
func Parse(text string) (*Node, error) {
	lines := splitLines(text)
	p := &parser{lines: lines}
	return p.parseNode(), nil
}

func splitLines(text string) []line {
	var out []line
	for _, raw := range strings.Split(text, "\n") {
		raw = strings.TrimSuffix(raw, "\r")
		stripped := strings.TrimLeft(raw, " \t")
		if stripped == "" {
			continue
		}
		indent := len(raw) - len(stripped)
		key, value, found := strings.Cut(stripped, ":")
		if !found {
			key = stripped
			value = ""
		}
		out = append(out, line{
			indent: indent,
			key:    strings.ToLower(strings.TrimSpace(key)),
			value:  strings.TrimSpace(value),
		})
	}
	return out
}

type parser struct {
	lines []line
	pos   int
}

func (p *parser) done() bool    { return p.pos >= len(p.lines) }
func (p *parser) current() line { return p.lines[p.pos] }
func (p *parser) advance()      { p.pos++ }

// parseNode parses the next node, skipping any unknown keys. A split that
// cannot complete both sides collapses to nil.
func (p *parser) parseNode() *Node {
	for !p.done() {
		ln := p.current()
		switch ln.key {
		case "run":
			p.advance()
			return &Node{Kind: KindRun, Command: ln.value}
		case "visit":
			p.advance()
			return &Node{Kind: KindVisit, URL: ln.value}
		case "split":
			return p.parseSplit(ln)
		case "tabs":
			return p.parseTabs()
		default:
			// Unknown keys are skipped; scanning continues.
			p.advance()
		}
	}
	return nil
}

// parseSplit parses a split's two sides. The direction value comparison is
// exact: only the literal "columns" selects columns.
func (p *parser) parseSplit(ln line) *Node {
	p.advance()
	dir := DirRows
	if ln.value == "columns" {
		dir = DirColumns
	}

	p.skipLabel()
	firstSize := p.parseSize()
	first := p.parseNode()
	if first == nil {
		return nil
	}

	p.skipLabel()
	secondSize := p.parseSize()
	second := p.parseNode()
	if second == nil {
		return nil
	}

	return &Node{
		Kind:       KindSplit,
		Dir:        dir,
		First:      first,
		Second:     second,
		FirstSize:  firstSize,
		SecondSize: secondSize,
	}
}

// skipLabel consumes a single side-label line when present. Any of the four
// labels is accepted in either position.
func (p *parser) skipLabel() {
	if p.done() {
		return
	}
	switch p.current().key {
	case "left", "right", "top", "bottom":
		p.advance()
	}
}

// parseSize consumes a size line when present. The line is consumed even
// when its value does not parse, in which case no size is recorded.
func (p *parser) parseSize() *float64 {
	if p.done() || p.current().key != "size" {
		return nil
	}
	value := p.current().value
	p.advance()
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	f /= 100
	return &f
}

// parseTabs collects consecutive run/visit lines. The first line with any
// other key ends the tab list; unlike general node parsing, unknown keys are
// not skipped here.
func (p *parser) parseTabs() *Node {
	p.advance()
	var children []*Node
	for !p.done() {
		ln := p.current()
		switch ln.key {
		case "run":
			children = append(children, &Node{Kind: KindRun, Command: ln.value})
			p.advance()
		case "visit":
			children = append(children, &Node{Kind: KindVisit, URL: ln.value})
			p.advance()
		default:
			if len(children) == 0 {
				return nil
			}
			return &Node{Kind: KindTabs, Children: children}
		}
	}
	if len(children) == 0 {
		return nil
	}
	return &Node{Kind: KindTabs, Children: children}
}
