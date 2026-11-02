package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain lets the scripts run the archcheck command from the test binary.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){"archcheck": main})
}

// TestScripts runs the command contract tests in testdata/script. Each
// script writes a module and runs archcheck on it, so the packages loader
// runs the go command offline with the build cache of the test.
func TestScripts(t *testing.T) {
	cache := os.Getenv("GOCACHE")
	if cache == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			t.Fatalf("find the user cache directory: %v", err)
		}
		cache = filepath.Join(dir, "go-build")
	}
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			env.Setenv("GOCACHE", cache)
			env.Setenv("GOPATH", filepath.Join(env.WorkDir, ".gopath"))
			env.Setenv("GOPROXY", "off")
			env.Setenv("GOTOOLCHAIN", "local")
			return nil
		},
	})
}
