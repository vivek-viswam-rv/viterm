package term

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func startShell(t *testing.T) (*Pane, *atomic.Bool) {
	t.Helper()
	var exited atomic.Bool
	p, err := Start(Options{
		Shell: "/bin/sh",
		Dir:   t.TempDir(),
		Cols:  80,
		Rows:  24,
		Notify: func(ev Event) {
			if _, ok := ev.(ExitEvent); ok {
				exited.Store(true)
			}
		},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return p, &exited
}

func TestTwoPanesStayAlive(t *testing.T) {
	p1, exit1 := startShell(t)
	p2, exit2 := startShell(t)
	defer terminate(p1)
	defer terminate(p2)

	time.Sleep(2 * time.Second)
	if exit1.Load() {
		t.Error("first pane's shell exited unexpectedly")
	}
	if exit2.Load() {
		t.Error("second pane's shell exited unexpectedly")
	}
}

func TestCommandOutputReachesScreen(t *testing.T) {
	p, _ := startShell(t)
	defer terminate(p)

	time.Sleep(300 * time.Millisecond)
	p.TypeCommand("echo pane-check-marker")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(p.Render(), "pane-check-marker") {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("marker never appeared; screen:\n%s", p.Render())
}

func TestTerminateStopsProcess(t *testing.T) {
	p, exited := startShell(t)
	time.Sleep(300 * time.Millisecond)
	terminate(p)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if exited.Load() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("shell did not exit after Terminate")
}

func terminate(p *Pane) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = p.Terminate(ctx)
}
