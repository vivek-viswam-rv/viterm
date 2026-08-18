package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/vivek-viswam-rv/viterm/internal/protocol"
)

func handleTabs(stdout, stderr io.Writer) int {
	socket, ok := requireSocket(stderr)
	if !ok {
		return 1
	}

	cmd := protocol.Command{
		Action: protocol.ActionTabsList,
		PaneID: os.Getenv(protocol.EnvPaneID),
	}
	reply, ok := sendOrFail(socket, cmd, stderr)
	if !ok {
		return 1
	}

	var tabs []protocol.TabListEntry
	if err := json.Unmarshal(reply, &tabs); err != nil {
		fmt.Fprintf(stderr, "Error: could not parse tab list: %v\n", err)
		return 1
	}

	renderTabs(tabs, stdout)
	return 0
}

// renderTabs prints the aligned TAB/PANE/TYPE/TITLE table, sorted by tab
// sequence number, marking the active tab's row with a trailing "*".
func renderTabs(tabs []protocol.TabListEntry, w io.Writer) {
	if len(tabs) == 0 {
		fmt.Fprintln(w, "No tabs.")
		return
	}

	sorted := make([]protocol.TabListEntry, len(tabs))
	copy(sorted, tabs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].TabSeq < sorted[j].TabSeq })

	fmt.Fprintln(w, "TAB  PANE  TYPE      TITLE")
	fmt.Fprintln(w, strings.Repeat("-", 50))
	for _, t := range sorted {
		active := ""
		if t.IsActive {
			active = " *"
		}
		fmt.Fprintf(w, "%-3d  %-4d  %-8s  %s%s\n", t.TabSeq, t.PaneSeq, t.Type, t.Title, active)
	}
}
