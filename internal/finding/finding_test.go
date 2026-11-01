package finding

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestString(t *testing.T) {
	f := Finding{Location: "a.go:3", Rule: "r", Message: "broken", Fix: "repair"}
	if got, want := f.String(), "a.go:3: r: broken\n\tfix: repair"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestSort(t *testing.T) {
	b := Finding{Location: "b", Rule: "r", Message: "m", Fix: "f"}
	a1 := Finding{Location: "a", Rule: "r", Message: "m", Fix: "f"}
	a2 := Finding{Location: "a", Rule: "s", Message: "m", Fix: "f"}
	got := Sort([]Finding{b, a2, a1, b})
	want := []Finding{a1, a2, b}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Sort() mismatch (-want +got):\n%s", diff)
	}
}
