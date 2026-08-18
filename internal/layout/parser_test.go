package layout

import "testing"

func TestParse_SingleRunLeaf(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"non-empty command", "run: foo", "foo"},
		{"empty command", "run:", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := Parse(tt.text)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if n == nil {
				t.Fatal("Parse returned nil node")
			}
			if n.Kind != KindRun {
				t.Fatalf("Kind = %v, want KindRun", n.Kind)
			}
			if n.Command != tt.want {
				t.Fatalf("Command = %q, want %q", n.Command, tt.want)
			}
		})
	}
}

func TestParse_SingleVisitLeaf(t *testing.T) {
	n, err := Parse("visit: https://example.com")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n == nil {
		t.Fatal("Parse returned nil node")
	}
	if n.Kind != KindVisit {
		t.Fatalf("Kind = %v, want KindVisit", n.Kind)
	}
	if n.URL != "https://example.com" {
		t.Fatalf("URL = %q, want %q", n.URL, "https://example.com")
	}
}

func TestParse_SplitSizes(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		wantFirstSize *float64
		wantRatio     float64
	}{
		{
			name:      "size on first side only",
			text:      "split: columns\nsize: 30%\nrun: a\nrun: b\n",
			wantRatio: 0.3,
		},
		{
			name:      "size on second side only",
			text:      "split: columns\nrun: a\nsize: 70%\nrun: b\n",
			wantRatio: 0.3, // complement of second's 0.7
		},
		{
			name:      "size on both sides, first wins",
			text:      "split: columns\nsize: 20%\nrun: a\nsize: 90%\nrun: b\n",
			wantRatio: 0.2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := Parse(tt.text)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if n == nil {
				t.Fatal("Parse returned nil node")
			}
			if n.Kind != KindSplit {
				t.Fatalf("Kind = %v, want KindSplit", n.Kind)
			}
			if got := n.Ratio(); !closeTo(got, tt.wantRatio) {
				t.Fatalf("Ratio() = %v, want %v", got, tt.wantRatio)
			}
		})
	}
}

func TestParse_SizeClamping(t *testing.T) {
	tests := []struct {
		name string
		text string
		want float64
	}{
		{"below 0.05 clamps up", "split: columns\nsize: 1%\nrun: a\nrun: b\n", 0.05},
		{"above 0.95 clamps down", "split: columns\nsize: 99%\nrun: a\nrun: b\n", 0.95},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := Parse(tt.text)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if n == nil {
				t.Fatal("Parse returned nil node")
			}
			if got := n.Ratio(); !closeTo(got, tt.want) {
				t.Fatalf("Ratio() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParse_SplitDirectionCaseSensitivity(t *testing.T) {
	// Keys are case-insensitive ("RUN:", "Split:") but the "columns" value
	// comparison is exact, so "Columns" (capital C) does not match and the
	// split falls back to DirRows.
	n, err := Parse("Split: Columns\nRUN: a\nrun: b\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n == nil {
		t.Fatal("Parse returned nil node")
	}
	if n.Kind != KindSplit {
		t.Fatalf("Kind = %v, want KindSplit", n.Kind)
	}
	if n.Dir != DirRows {
		t.Fatalf("Dir = %v, want DirRows (case-sensitive value compare)", n.Dir)
	}
	if n.First == nil || n.First.Command != "a" {
		t.Fatalf("First = %+v, want run %q", n.First, "a")
	}
	if n.Second == nil || n.Second.Command != "b" {
		t.Fatalf("Second = %+v, want run %q", n.Second, "b")
	}

	// Lowercase "columns" does select DirColumns.
	n2, err := Parse("split: columns\nrun: a\nrun: b\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n2 == nil || n2.Dir != DirColumns {
		t.Fatalf("Dir = %v, want DirColumns", n2.Dir)
	}
}

func TestParse_TabsCollectsRunsAndStopsAtUnknownKey(t *testing.T) {
	// The interrupting unknown key is not consumed, but since this tabs node
	// is the sole top-level node, Parse only calls parseNode once and
	// everything after the interrupter -- including further run lines -- is
	// silently dropped.
	text := "tabs:\nrun: one\nrun: two\nbogus: nope\nrun: three\nrun: four\n"
	n, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n == nil {
		t.Fatal("Parse returned nil node")
	}
	if n.Kind != KindTabs {
		t.Fatalf("Kind = %v, want KindTabs", n.Kind)
	}
	if len(n.Children) != 2 {
		t.Fatalf("len(Children) = %d, want 2", len(n.Children))
	}
	if n.Children[0].Command != "one" || n.Children[1].Command != "two" {
		t.Fatalf("Children = %+v, want [one two]", n.Children)
	}
	leaves := n.Leaves()
	if len(leaves) != 2 {
		t.Fatalf("Leaves() len = %d, want 2 (later run lines dropped)", len(leaves))
	}
}

func TestParse_TabsMixedRunVisit(t *testing.T) {
	text := "tabs:\nrun: a\nvisit: http://x\nrun: b\n"
	n, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n == nil || n.Kind != KindTabs {
		t.Fatalf("n = %+v, want KindTabs", n)
	}
	if len(n.Children) != 3 {
		t.Fatalf("len(Children) = %d, want 3", len(n.Children))
	}
	if n.Children[0].Kind != KindRun || n.Children[1].Kind != KindVisit || n.Children[2].Kind != KindRun {
		t.Fatalf("Children kinds = %v, %v, %v", n.Children[0].Kind, n.Children[1].Kind, n.Children[2].Kind)
	}
}

func TestParse_UnknownKeysSkippedAtTopLevel(t *testing.T) {
	n, err := Parse("junk: whatever\nrun: hello\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n == nil {
		t.Fatal("Parse returned nil node")
	}
	if n.Kind != KindRun || n.Command != "hello" {
		t.Fatalf("n = %+v, want run %q", n, "hello")
	}
}

func TestParse_UnknownKeysSkippedBetweenSplitChildren(t *testing.T) {
	text := "split: columns\nrun: a\njunk: whatever\nrun: b\n"
	n, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n == nil {
		t.Fatal("Parse returned nil node")
	}
	if n.Kind != KindSplit {
		t.Fatalf("Kind = %v, want KindSplit", n.Kind)
	}
	if n.First == nil || n.First.Command != "a" {
		t.Fatalf("First = %+v, want run %q", n.First, "a")
	}
	if n.Second == nil || n.Second.Command != "b" {
		t.Fatalf("Second = %+v, want run %q", n.Second, "b")
	}
}

func TestParse_MalformedSizeConsumedNotRecorded(t *testing.T) {
	text := "split: columns\nsize: abc\nrun: a\nrun: b\n"
	n, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n == nil {
		t.Fatal("Parse returned nil node")
	}
	if n.FirstSize != nil {
		t.Fatalf("FirstSize = %v, want nil", *n.FirstSize)
	}
	// Parsing must have continued correctly past the consumed size line.
	if n.First == nil || n.First.Command != "a" {
		t.Fatalf("First = %+v, want run %q", n.First, "a")
	}
	if n.Second == nil || n.Second.Command != "b" {
		t.Fatalf("Second = %+v, want run %q", n.Second, "b")
	}
}

func TestDedent(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "common indent stripped",
			in:   "  foo\n  bar\n    baz\n",
			want: "foo\nbar\n  baz\n",
		},
		{
			name: "no common indent unchanged",
			in:   "foo\n  bar\n",
			want: "foo\n  bar\n",
		},
		{
			name: "blank lines ignored for prefix computation",
			in:   "  foo\n\n  bar\n",
			want: "foo\n\nbar\n",
		},
		{
			name: "blank line amid indented lines does not reduce prefix",
			in:   "    foo\n\n    bar\n    baz\n",
			want: "foo\n\nbar\nbaz\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Dedent(tt.in)
			if got != tt.want {
				t.Fatalf("Dedent(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDefaultLayoutText(t *testing.T) {
	n, err := Parse(DefaultLayoutText)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n == nil {
		t.Fatal("Parse returned nil node")
	}
	if n.Kind != KindSplit {
		t.Fatalf("Kind = %v, want KindSplit", n.Kind)
	}
	if n.Dir != DirColumns {
		t.Fatalf("Dir = %v, want DirColumns", n.Dir)
	}
	if got := n.Ratio(); got != 0.6 {
		t.Fatalf("Ratio() = %v, want 0.6", got)
	}
	if n.First == nil || n.First.Kind != KindRun || n.First.Command != "claude" {
		t.Fatalf("First = %+v, want run %q", n.First, "claude")
	}
	if n.Second == nil || n.Second.Kind != KindRun || n.Second.Command != "" {
		t.Fatalf("Second = %+v, want run with empty command", n.Second)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	for _, text := range []string{"", "   ", "\n\n  \t\n"} {
		n, err := Parse(text)
		if err != nil {
			t.Fatalf("Parse(%q): unexpected error %v", text, err)
		}
		if n != nil {
			t.Fatalf("Parse(%q) = %+v, want nil node", text, n)
		}
	}
}

func TestParse_SplitMissingSecondChild(t *testing.T) {
	n, err := Parse("split: columns\nrun: a\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n != nil {
		t.Fatalf("Parse = %+v, want nil (missing second child)", n)
	}
}

func TestNode_LeavesOrder(t *testing.T) {
	text := "split: columns\nrun: a\ntabs:\nrun: b\nvisit: http://c\n"
	n, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n == nil {
		t.Fatal("Parse returned nil node")
	}
	leaves := n.Leaves()
	if len(leaves) != 3 {
		t.Fatalf("len(Leaves()) = %d, want 3: %+v", len(leaves), leaves)
	}
	if leaves[0].Kind != KindRun || leaves[0].Command != "a" {
		t.Fatalf("leaves[0] = %+v, want run %q", leaves[0], "a")
	}
	if leaves[1].Kind != KindRun || leaves[1].Command != "b" {
		t.Fatalf("leaves[1] = %+v, want run %q", leaves[1], "b")
	}
	if leaves[2].Kind != KindVisit || leaves[2].URL != "http://c" {
		t.Fatalf("leaves[2] = %+v, want visit %q", leaves[2], "http://c")
	}
}

func TestRewriteRunCommands(t *testing.T) {
	text := "split: columns\nrun: a\ntabs:\nrun: b\nvisit: http://c\n"
	n, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n == nil {
		t.Fatal("Parse returned nil node")
	}
	RewriteRunCommands(n, func(s string) string { return "prefix-" + s })

	leaves := n.Leaves()
	for _, l := range leaves {
		switch l.Kind {
		case KindRun:
			if l.Command == "a" || l.Command == "b" {
				t.Fatalf("run leaf not rewritten: %+v", l)
			}
			if l.Command != "prefix-a" && l.Command != "prefix-b" {
				t.Fatalf("unexpected run command: %q", l.Command)
			}
		case KindVisit:
			if l.URL != "http://c" {
				t.Fatalf("visit leaf mutated: %+v", l)
			}
		}
	}
}

// closeTo compares floats with a small tolerance.
func closeTo(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}
