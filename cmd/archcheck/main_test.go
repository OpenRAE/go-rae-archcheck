package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
)

// fixtureModel is a valid model with no rules and no code to check.
const fixtureModel = `version: 1
module: example.com/m
system:
  id: m
  name: M
  description: A system.
people: []
external_systems: []
relationships: []
containers:
  - id: app
    name: App
    technology: Go
    description: The application.
layers:
  - id: core
    directory: internal/core
    description: The core.
    uses: []
    external: []
components: []
test_modules: []
rules: []
`

const fixtureRecord = "---\ndesign: 1\nissue: 1\ntitle: Start\n---\n"

func runCommand(t *testing.T, args []string, fsys fstest.MapFS) (status int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	status = run(context.Background(), args, fsys, ".", &out, &errOut)
	return status, out.String(), errOut.String()
}

func TestRunNoFindings(t *testing.T) {
	fsys := fstest.MapFS{
		"architecture/model.yaml":              {Data: []byte(fixtureModel)},
		"architecture/designs/0001-start.md":   {Data: []byte(fixtureRecord)},
		"architecture/designs/notes/README.md": {Data: []byte("not a record")},
	}
	status, stdout, stderr := runCommand(t, []string{"-code=false"}, fsys)
	if status != 0 || stdout != "archcheck: no findings\n" || stderr != "" {
		t.Errorf("run() = %d, %q, %q, want 0 and no findings", status, stdout, stderr)
	}
}

func TestRunFindings(t *testing.T) {
	fsys := fstest.MapFS{
		"architecture/model.yaml":          {Data: []byte(strings.Replace(fixtureModel, "version: 1", "version: 9", 1))},
		"architecture/designs/bad name.md": {Data: []byte(fixtureRecord)},
	}
	status, _, stderr := runCommand(t, nil, fsys)
	if status != 1 {
		t.Errorf("run() status = %d, want 1", status)
	}
	for _, want := range []string{`the value at "/version" breaks the schema`, "the file name is not NNNN-slug.md", "archcheck: 2 findings"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
}

func TestRunErrors(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		fsys   fstest.MapFS
		status int
		stderr string
	}{
		{name: "unknown flag", args: []string{"-unknown"}, status: 2, stderr: "flag provided but not defined"},
		{name: "argument", args: []string{"extra"}, status: 2, stderr: "usage: archcheck"},
		{name: "base model alone", args: []string{"-base-model", "x"}, status: 2, stderr: "usage: archcheck"},
		{name: "missing model", fsys: fstest.MapFS{}, status: 1, stderr: "read the model"},
		{name: "bad model", fsys: fstest.MapFS{"architecture/model.yaml": {Data: []byte("version: [1\n")}}, status: 1, stderr: "decode the model"},
		{name: "unreadable designs", fsys: fstest.MapFS{
			"architecture/model.yaml": {Data: []byte(fixtureModel)},
			"architecture/designs":    {Data: []byte("a file, not a directory")},
		}, args: []string{"-code=false"}, status: 1, stderr: "list the design records"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, _, stderr := runCommand(t, tc.args, tc.fsys)
			if status != tc.status || !strings.Contains(stderr, tc.stderr) {
				t.Errorf("run() = %d, %q, want %d and %q", status, stderr, tc.status, tc.stderr)
			}
		})
	}
}

func TestRunBaseChange(t *testing.T) {
	dir := t.TempDir()
	baseModel := filepath.Join(dir, "base.yaml")
	baseList := filepath.Join(dir, "base.txt")
	writeFile(t, baseModel, strings.Replace(fixtureModel, "internal/core", "internal/kernel", 1))
	writeFile(t, baseList, "architecture/designs/0001-start.md\n\n")
	fsys := fstest.MapFS{
		"architecture/model.yaml":            {Data: []byte(fixtureModel)},
		"architecture/designs/0001-start.md": {Data: []byte(fixtureRecord)},
	}
	args := []string{"-code=false", "-base-model", baseModel, "-base-designs", baseList}
	status, _, stderr := runCommand(t, args, fsys)
	if status != 1 || !strings.Contains(stderr, "adds no design record") || !strings.Contains(stderr, "archcheck: 1 finding\n") {
		t.Errorf("run() = %d, %q, want 1 and a design-record finding", status, stderr)
	}
	fsys["architecture/designs/0002-move.md"] = &fstest.MapFile{Data: []byte("---\ndesign: 2\nissue: 1\ntitle: Move\n---\n")}
	if status, _, stderr := runCommand(t, args, fsys); status != 0 {
		t.Errorf("run() with a new record = %d, %q, want 0", status, stderr)
	}
}

func TestRunBaseErrors(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	bad := filepath.Join(dir, "bad.yaml")
	writeFile(t, good, fixtureModel)
	writeFile(t, bad, "unknown: 1\n")
	fsys := fstest.MapFS{"architecture/model.yaml": {Data: []byte(fixtureModel)}}
	tests := []struct {
		name, model, list, stderr string
	}{
		{name: "missing base model", model: filepath.Join(dir, "none.yaml"), list: good, stderr: "read the base model"},
		{name: "bad base model", model: bad, list: good, stderr: "the base model"},
		{name: "missing list", model: good, list: filepath.Join(dir, "none.txt"), stderr: "read the base design record list"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, _, stderr := runCommand(t, []string{"-code=false", "-base-model", tc.model, "-base-designs", tc.list}, fsys)
			if status != 1 || !strings.Contains(stderr, tc.stderr) {
				t.Errorf("run() = %d, %q, want 1 and %q", status, stderr, tc.stderr)
			}
		})
	}
}

func TestRunCode(t *testing.T) {
	if testing.Short() {
		t.Skip("loads packages with the go command")
	}
	dir := "testdata/module"
	var out, errOut bytes.Buffer
	status := run(context.Background(), nil, os.DirFS(dir), dir, &out, &errOut)
	want := "internal/core/core.go:8: no-env: use of os.Getenv breaks the rule: Code does not read the environment.\n\tfix: Pass the value in.\narchcheck: 1 finding\n"
	if status != 1 {
		t.Errorf("run() status = %d, want 1", status)
	}
	if diff := cmp.Diff(want, errOut.String()); diff != "" {
		t.Errorf("stderr mismatch (-want +got):\n%s", diff)
	}
}

func TestRunCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fsys := fstest.MapFS{"architecture/model.yaml": {Data: []byte(fixtureModel)}}
	var out, errOut bytes.Buffer
	if status := run(ctx, nil, fsys, ".", &out, &errOut); status != 1 || !strings.Contains(errOut.String(), "context canceled") {
		t.Errorf("run() = %d, %q, want 1 and a canceled context", status, errOut.String())
	}
}

func TestBaseNames(t *testing.T) {
	got := baseNames([]byte("architecture/designs/0001-a.md\n\narchitecture/designs/README.txt\n0001-a.md\n0002-b.md\n"))
	if diff := cmp.Diff([]string{"0001-a.md", "0002-b.md"}, got); diff != "" {
		t.Errorf("baseNames() mismatch (-want +got):\n%s", diff)
	}
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
