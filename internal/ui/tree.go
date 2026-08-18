package ui

import "github.com/vivek-viswam-rv/viterm/internal/layout"

// TreeNode is the runtime split tree: either a leaf holding a pane or a
// two-way split.
type TreeNode struct {
	Pane *Pane // leaf when non-nil

	Dir    layout.Dir
	Ratio  float64
	First  *TreeNode
	Second *TreeNode
}

func leafNode(p *Pane) *TreeNode { return &TreeNode{Pane: p} }

// IsLeaf reports whether the node holds a pane.
func (n *TreeNode) IsLeaf() bool { return n.Pane != nil }

// Panes returns all panes in document order.
func (n *TreeNode) Panes() []*Pane {
	if n == nil {
		return nil
	}
	if n.IsLeaf() {
		return []*Pane{n.Pane}
	}
	return append(n.First.Panes(), n.Second.Panes()...)
}

// Rect is a screen-cell rectangle.
type Rect struct{ X, Y, W, H int }

// PaneRect pairs a pane with its computed rectangle.
type PaneRect struct {
	Pane *Pane
	Rect Rect
}

// Layout computes pane rectangles within bounds, reserving one divider cell
// between the sides of every split.
func (n *TreeNode) Layout(bounds Rect) []PaneRect {
	if n == nil {
		return nil
	}
	if n.IsLeaf() {
		return []PaneRect{{Pane: n.Pane, Rect: bounds}}
	}
	ratio := n.Ratio
	if ratio <= 0 {
		ratio = 0.5
	}
	if n.Dir == layout.DirColumns {
		avail := bounds.W - 1
		first := int(float64(avail) * ratio)
		first = clamp(first, 1, avail-1)
		a := Rect{X: bounds.X, Y: bounds.Y, W: first, H: bounds.H}
		b := Rect{X: bounds.X + first + 1, Y: bounds.Y, W: avail - first, H: bounds.H}
		return append(n.First.Layout(a), n.Second.Layout(b)...)
	}
	avail := bounds.H - 1
	first := int(float64(avail) * ratio)
	first = clamp(first, 1, avail-1)
	a := Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: first}
	b := Rect{X: bounds.X, Y: bounds.Y + first + 1, W: bounds.W, H: avail - first}
	return append(n.First.Layout(a), n.Second.Layout(b)...)
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// findParent returns the split whose first or second child is the leaf for
// pane, along with which side it is on.
func (n *TreeNode) findParent(paneID string) (parent *TreeNode, second bool) {
	if n == nil || n.IsLeaf() {
		return nil, false
	}
	if n.First.IsLeaf() && n.First.Pane.ID == paneID {
		return n, false
	}
	if n.Second.IsLeaf() && n.Second.Pane.ID == paneID {
		return n, true
	}
	if p, s := n.First.findParent(paneID); p != nil {
		return p, s
	}
	return n.Second.findParent(paneID)
}

// findLeaf returns the leaf node for a pane ID.
func (n *TreeNode) findLeaf(paneID string) *TreeNode {
	if n == nil {
		return nil
	}
	if n.IsLeaf() {
		if n.Pane.ID == paneID {
			return n
		}
		return nil
	}
	if l := n.First.findLeaf(paneID); l != nil {
		return l
	}
	return n.Second.findLeaf(paneID)
}

// SplitPane replaces the pane's leaf with a split holding the existing pane
// first and the new pane second.
func (n *TreeNode) SplitPane(paneID string, dir layout.Dir, newPane *Pane) bool {
	leaf := n.findLeaf(paneID)
	if leaf == nil {
		return false
	}
	existing := leaf.Pane
	leaf.Pane = nil
	leaf.Dir = dir
	leaf.Ratio = 0.5
	leaf.First = leafNode(existing)
	leaf.Second = leafNode(newPane)
	return true
}

// RemovePane collapses the pane's parent split, promoting the sibling. The
// root leaf cannot be removed; callers close the session instead.
func (n *TreeNode) RemovePane(paneID string) bool {
	parent, second := n.findParent(paneID)
	if parent == nil {
		return false
	}
	sibling := parent.First
	if !second {
		sibling = parent.Second
	}
	*parent = *sibling
	return true
}

// center returns a rectangle's midpoint.
func (r Rect) center() (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

// FocusNeighbor returns the pane nearest to the focused pane in the given
// direction (dx, dy), or nil when none exists.
func FocusNeighbor(rects []PaneRect, focusedID string, dx, dy int) *Pane {
	var from *PaneRect
	for i := range rects {
		if rects[i].Pane.ID == focusedID {
			from = &rects[i]
			break
		}
	}
	if from == nil {
		return nil
	}
	fx, fy := from.Rect.center()
	var best *Pane
	bestDist := 1 << 30
	for i := range rects {
		pr := &rects[i]
		if pr.Pane.ID == focusedID {
			continue
		}
		cx, cy := pr.Rect.center()
		if dx < 0 && cx >= fx || dx > 0 && cx <= fx ||
			dy < 0 && cy >= fy || dy > 0 && cy <= fy {
			continue
		}
		// Weight the off-axis distance so lateral neighbors win over
		// diagonal ones.
		var dist int
		if dx != 0 {
			dist = abs(cx-fx) + 3*abs(cy-fy)
		} else {
			dist = abs(cy-fy) + 3*abs(cx-fx)
		}
		if dist < bestDist {
			bestDist = dist
			best = pr.Pane
		}
	}
	return best
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
