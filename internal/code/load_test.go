package code

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadFixture(t *testing.T) {
	if testing.Short() {
		t.Skip("loads packages with the go command")
	}
	g, err := Load("testdata/fixture")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := g.Module, "example.com/fixture"; got != want {
		t.Errorf("Module = %q, want %q", got, want)
	}
	var paths []string
	for _, p := range g.Packages {
		paths = append(paths, p.Path)
	}
	wantPaths := []string{"example.com/fixture/cmd/app", "example.com/fixture/internal/core", "example.com/fixture/internal/core/coretest"}
	if diff := cmp.Diff(wantPaths, paths); diff != "" {
		t.Errorf("package paths mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"internal/core/ignored.go"}, g.Unread); diff != "" {
		t.Errorf("Unread mismatch (-want +got):\n%s", diff)
	}
	core := g.Packages[1]
	if got, want := len(core.Files), 2; got != want {
		t.Fatalf("len(core.Files) = %d, want %d", got, want)
	}
	lib, test := core.Files[0], core.Files[1]
	if lib.Test || !test.Test {
		t.Errorf("Test flags = %t, %t, want false, true", lib.Test, test.Test)
	}
	if !slices.Contains(lib.Symbols, Symbol{Name: "os.Getenv", Line: 8}) {
		t.Errorf("core.go symbols = %v, want os.Getenv at line 8", lib.Symbols)
	}
	if !slices.Contains(test.Imports, Import{Path: "example.com/fixture/internal/core/coretest", Module: "example.com/fixture", Line: 7}) {
		t.Errorf("core_test.go imports = %v, want the coretest import", test.Imports)
	}
	if !slices.Contains(lib.Imports, Import{Path: "os", Line: 4}) {
		t.Errorf("core.go imports = %v, want os with no module", lib.Imports)
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load("testdata/missing"); err == nil {
		t.Errorf("Load() of a directory with no go.mod error = nil, want an error")
	}
}
