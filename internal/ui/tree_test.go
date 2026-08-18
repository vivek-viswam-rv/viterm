package ui

import (
	"testing"

	"github.com/vivek-viswam-rv/viterm/internal/layout"
)

func newTestPane(id string) *Pane { return &Pane{ID: id} }

func TestSplitAndRemovePane(t *testing.T) {
	a := newTestPane("a")
	root := leafNode(a)

	b := newTestPane("b")
	if !root.SplitPane("a", layout.DirColumns, b) {
		t.Fatal("SplitPane failed")
	}
	if root.IsLeaf() {
		t.Fatal("root still a leaf after split")
	}
	panes := root.Panes()
	if len(panes) != 2 || panes[0].ID != "a" || panes[1].ID != "b" {
		t.Fatalf("Panes = %v", ids(panes))
	}

	c := newTestPane("c")
	if !root.SplitPane("b", layout.DirRows, c) {
		t.Fatal("nested SplitPane failed")
	}
	if got := ids(root.Panes()); len(got) != 3 {
		t.Fatalf("Panes = %v", got)
	}

	if !root.RemovePane("b") {
		t.Fatal("RemovePane failed")
	}
	got := ids(root.Panes())
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("after remove, Panes = %v", got)
	}

	if !root.RemovePane("c") {
		t.Fatal("RemovePane failed")
	}
	if !root.IsLeaf() || root.Pane.ID != "a" {
		t.Fatalf("root should collapse to leaf a, got %v", ids(root.Panes()))
	}

	if root.RemovePane("a") {
		t.Fatal("removing the last pane should fail")
	}
}

func ids(panes []*Pane) []string {
	out := make([]string, len(panes))
	for i, p := range panes {
		out[i] = p.ID
	}
	return out
}

func TestLayoutRectangles(t *testing.T) {
	a, b := newTestPane("a"), newTestPane("b")
	root := &TreeNode{Dir: layout.DirColumns, Ratio: 0.5, First: leafNode(a), Second: leafNode(b)}

	rects := root.Layout(Rect{X: 0, Y: 1, W: 81, H: 20})
	if len(rects) != 2 {
		t.Fatalf("len(rects) = %d", len(rects))
	}
	ra, rb := rects[0].Rect, rects[1].Rect
	if ra.W+rb.W+1 != 81 {
		t.Errorf("widths %d + %d + divider != 81", ra.W, rb.W)
	}
	if rb.X != ra.X+ra.W+1 {
		t.Errorf("second pane not adjacent past divider: %+v %+v", ra, rb)
	}
	if ra.H != 20 || rb.H != 20 {
		t.Errorf("heights = %d, %d, want 20", ra.H, rb.H)
	}
}

func TestFocusNeighbor(t *testing.T) {
	a, b, c := newTestPane("a"), newTestPane("b"), newTestPane("c")
	rects := []PaneRect{
		{Pane: a, Rect: Rect{X: 0, Y: 0, W: 40, H: 20}},
		{Pane: b, Rect: Rect{X: 41, Y: 0, W: 40, H: 9}},
		{Pane: c, Rect: Rect{X: 41, Y: 10, W: 40, H: 10}},
	}

	if got := FocusNeighbor(rects, "a", 1, 0); got == nil || (got.ID != "b" && got.ID != "c") {
		t.Errorf("right of a = %v", got)
	}
	if got := FocusNeighbor(rects, "b", 0, 1); got == nil || got.ID != "c" {
		t.Errorf("below b = %v, want c", got)
	}
	if got := FocusNeighbor(rects, "c", 0, -1); got == nil || got.ID != "b" {
		t.Errorf("above c = %v, want b", got)
	}
	if got := FocusNeighbor(rects, "b", -1, 0); got == nil || got.ID != "a" {
		t.Errorf("left of b = %v, want a", got)
	}
	if got := FocusNeighbor(rects, "a", -1, 0); got != nil {
		t.Errorf("left of a = %v, want nil", got)
	}
}

func TestPaneTabOperations(t *testing.T) {
	p := &Pane{ID: "p"}
	p.Tabs = []*Tab{{ID: "t1", Seq: 1}, {ID: "t2", Seq: 2}, {ID: "t3", Seq: 3}}
	p.Active = 0

	p.CycleTab(1)
	if p.Active != 1 {
		t.Errorf("Active = %d, want 1", p.Active)
	}
	p.CycleTab(-2)
	if p.Active != 2 {
		t.Errorf("Active = %d, want 2 (wrap)", p.Active)
	}
	p.SelectTab(99)
	if p.Active != 2 {
		t.Errorf("SelectTab clamped = %d, want 2", p.Active)
	}

	if i, tab := p.FindTab("2"); i != 1 || tab == nil || tab.ID != "t2" {
		t.Errorf("FindTab by seq = %d, %v", i, tab)
	}
	if i, tab := p.FindTab("t3"); i != 2 || tab == nil {
		t.Errorf("FindTab by id = %d, %v", i, tab)
	}

	p.RemoveTab(2)
	if len(p.Tabs) != 2 || p.Active != 1 {
		t.Errorf("after remove: len=%d active=%d", len(p.Tabs), p.Active)
	}
}
