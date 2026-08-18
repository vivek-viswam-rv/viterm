//go:build !darwin

// The window host is currently macOS-only; other platforms run viterm
// directly in a terminal.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "viterm-app is only available on macOS; run viterm in a terminal instead")
	os.Exit(1)
}
