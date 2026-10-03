package theme

import "testing"

func TestGetIsCaseInsensitive(t *testing.T) {
	if got := Get("Aurora").Name; got != "aurora" {
		t.Errorf("Get(\"Aurora\").Name = %q, want aurora", got)
	}
}

func TestGetUnknownFallsBackToDefault(t *testing.T) {
	for _, name := range []string{"nope", ""} {
		if got := Get(name).Name; got != DefaultName {
			t.Errorf("Get(%q).Name = %q, want %q", name, got, DefaultName)
		}
	}
}

func TestKnown(t *testing.T) {
	cases := map[string]bool{"ember": true, "EMBER": true, "": false, "nope": false}
	for name, want := range cases {
		if got := Known(name); got != want {
			t.Errorf("Known(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestNamesListsEveryTheme(t *testing.T) {
	names := Names()
	if len(names) != 4 {
		t.Errorf("len(Names()) = %d, want 4", len(names))
	}
	hasDefault := false
	for _, n := range names {
		if n == DefaultName {
			hasDefault = true
		}
		if !Known(n) {
			t.Errorf("Known(%q) = false, want true", n)
		}
	}
	if !hasDefault {
		t.Errorf("Names() = %v, missing %q", names, DefaultName)
	}
}
