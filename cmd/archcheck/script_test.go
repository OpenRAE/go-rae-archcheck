package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
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
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			"exitcode": checkExitCode,
		},
		Setup: func(env *testscript.Env) error {
			env.Setenv("GOCACHE", cache)
			env.Setenv("GOPATH", filepath.Join(env.WorkDir, ".gopath"))
			env.Setenv("GOPROXY", "off")
			env.Setenv("GOTOOLCHAIN", "local")
			return nil
		},
	})
}

// checkExitCode preserves the command output and checks its exact exit status.
func checkExitCode(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) < 2 {
		ts.Fatalf("usage: exitcode status command [args...]")
	}
	want, err := strconv.Atoi(args[0])
	if err != nil || want < 0 || want > 255 {
		ts.Fatalf("invalid exit status %q", args[0])
	}
	got := 0
	if err := ts.Exec(args[1], args[2:]...); err != nil {
		var status interface{ ExitCode() int }
		if !errors.As(err, &status) {
			ts.Fatalf("run command: %v", err)
		}
		got = status.ExitCode()
	}
	if got != want {
		ts.Fatalf("exit status: got %d, want %d", got, want)
	}
}
