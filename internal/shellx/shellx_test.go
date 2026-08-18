package shellx

import (
	"errors"
	"strings"
	"testing"
)

func TestLoginShellRespectsEnv(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	if got := LoginShell(); got != "/bin/sh" {
		t.Errorf("LoginShell = %q, want /bin/sh", got)
	}
}

func TestRunSuccessTrimsOutput(t *testing.T) {
	out, err := Run(t.TempDir(), "sh", "-c", "echo '  hello  '")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "hello" {
		t.Errorf("out = %q, want %q", out, "hello")
	}
}

func TestRunFailureCarriesStderr(t *testing.T) {
	_, err := Run(t.TempDir(), "sh", "-c", "echo oops >&2; exit 3")
	if err == nil {
		t.Fatal("Run succeeded, want failure")
	}
	var xerr *ExitError
	if !errors.As(err, &xerr) {
		t.Fatalf("error type = %T, want *ExitError", err)
	}
	if !strings.Contains(xerr.Stderr, "oops") {
		t.Errorf("Stderr = %q, want to contain %q", xerr.Stderr, "oops")
	}
	if xerr.Output() != "oops" {
		t.Errorf("Output() = %q, want %q", xerr.Output(), "oops")
	}
	if !strings.Contains(err.Error(), "oops") {
		t.Errorf("Error() = %q lacks stderr detail", err.Error())
	}
}

func TestRunShellSeesExtraEnv(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	out, err := RunShell(t.TempDir(), []string{"VITERM_TEST_VALUE=42"}, "echo $VITERM_TEST_VALUE")
	if err != nil {
		t.Fatalf("RunShell: %v", err)
	}
	if out != "42" {
		t.Errorf("out = %q, want %q", out, "42")
	}
}

func TestOpenInBrowserRejectsUnsafeURLs(t *testing.T) {
	for _, u := range []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"not a url",
		"relative/path",
		"http://",
	} {
		if err := OpenInBrowser(u); err == nil {
			t.Errorf("OpenInBrowser(%q) succeeded, want error", u)
		}
	}
}
