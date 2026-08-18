package layout

import "strings"

// Dedent strips the longest common leading whitespace prefix of all
// non-blank lines. The prefix comparison is byte-wise, so lines mixing tabs
// and spaces only share a prefix up to the first differing character. Blank
// lines are left untouched and do not affect the computed prefix.
func Dedent(text string) string {
	lines := strings.Split(text, "\n")
	prefix := ""
	first := true
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		indent := ln[:len(ln)-len(strings.TrimLeft(ln, " \t"))]
		if first {
			prefix = indent
			first = false
			continue
		}
		prefix = commonPrefix(prefix, indent)
		if prefix == "" {
			return text
		}
	}
	if prefix == "" {
		return text
	}
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		lines[i] = strings.TrimPrefix(ln, prefix)
	}
	return strings.Join(lines, "\n")
}

func commonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}
