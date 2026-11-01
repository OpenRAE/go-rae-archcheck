// Command archcheck checks a Go module against its architecture model.
//
// Run it from the module root:
//
//	archcheck [-model path] [-designs dir] [-base-model path] [-base-designs path] [-code=false]
//
// It validates the model and its design records, and then checks the Go
// packages of the module against the model. With -base-model and
// -base-designs, it also requires a new design record when the structure of
// the model changed since the base revision. -base-designs names a file that
// lists the record file names of the base revision, one on each line, as
// "git ls-tree --name-only" prints them.
//
// The exit status is 0 with no findings, 1 with findings or an error, and 2
// for a usage error.
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path"
	"slices"
	"strings"

	"github.com/OpenRAE/go-rae-archcheck/internal/code"
	"github.com/OpenRAE/go-rae-archcheck/internal/finding"
	"github.com/OpenRAE/go-rae-archcheck/internal/model"
	"github.com/OpenRAE/go-rae-archcheck/internal/record"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	status := run(ctx, os.Args[1:], os.DirFS("."), ".", os.Stdout, os.Stderr)
	stop()
	os.Exit(status)
}

// options are the parsed command-line flags.
type options struct {
	model       string
	designs     string
	baseModel   string
	baseDesigns string
	code        bool
}

// run executes the command on the module in dir, which fsys also reads, and
// returns its exit status. A failed write to the output streams has no
// recovery, so the exit status alone reports it.
func run(ctx context.Context, args []string, fsys fs.FS, dir string, stdout, stderr io.Writer) int {
	opts, ok := parse(args, stderr)
	if !ok {
		return 2
	}
	findings, err := check(ctx, opts, fsys, dir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "archcheck: %v\n", err)
		return 1
	}
	for _, f := range findings {
		_, _ = fmt.Fprintln(stderr, f)
	}
	if len(findings) > 0 {
		_, _ = fmt.Fprintf(stderr, "archcheck: %d %s\n", len(findings), plural(len(findings), "finding", "findings"))
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "archcheck: no findings")
	return 0
}

func parse(args []string, stderr io.Writer) (options, bool) {
	var opts options
	flags := flag.NewFlagSet("archcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.model, "model", "architecture/model.yaml", "the path of the architecture model")
	flags.StringVar(&opts.designs, "designs", "architecture/designs", "the directory of the design records")
	flags.StringVar(&opts.baseModel, "base-model", "", "the path of the model at the base revision")
	flags.StringVar(&opts.baseDesigns, "base-designs", "", "a file that lists the design record names at the base revision")
	flags.BoolVar(&opts.code, "code", true, "check the Go packages of the module")
	if err := flags.Parse(args); err != nil {
		return options{}, false
	}
	if flags.NArg() != 0 || (opts.baseModel == "") != (opts.baseDesigns == "") {
		_, _ = fmt.Fprintln(stderr, "usage: archcheck [-model path] [-designs dir] [-base-model path -base-designs path] [-code=false]")
		return options{}, false
	}
	return opts, true
}

// check returns the findings of the model, the design records and the code.
// It checks the code only when the model has no defect.
func check(ctx context.Context, opts options, fsys fs.FS, dir string) ([]finding.Finding, error) {
	m, err := readModel(fsys, opts.model)
	if err != nil {
		return nil, err
	}
	findings := model.Validate(m, opts.model)
	recordFindings, err := record.Check(fsys, opts.designs)
	if err != nil {
		return nil, err
	}
	findings = append(findings, recordFindings...)
	if opts.baseModel != "" {
		changeFindings, changeErr := checkChange(fsys, opts, m)
		if changeErr != nil {
			return nil, changeErr
		}
		findings = append(findings, changeFindings...)
	}
	if len(findings) > 0 || !opts.code {
		return finding.Sort(findings), nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	g, err := code.Load(dir)
	if err != nil {
		return nil, err
	}
	return code.Check(m, opts.model, g), nil
}

// checkChange compares the model with the base model. The base files can sit
// outside the module, so it reads them by their operating system paths.
func checkChange(fsys fs.FS, opts options, head *model.Model) ([]finding.Finding, error) {
	data, err := os.ReadFile(opts.baseModel)
	if err != nil {
		return nil, fmt.Errorf("read the base model: %w", err)
	}
	base, err := model.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("the base model: %w", err)
	}
	listing, err := os.ReadFile(opts.baseDesigns)
	if err != nil {
		return nil, fmt.Errorf("read the base design record list: %w", err)
	}
	headNames, err := record.Names(fsys, opts.designs)
	if err != nil {
		return nil, err
	}
	return record.CheckChange(base, head, baseNames(listing), headNames, opts.model)
}

// baseNames returns the record file names in a listing with one path on each
// line. Blank lines and paths that are not Markdown files do not count.
func baseNames(listing []byte) []string {
	var names []string
	scanner := bufio.NewScanner(bytes.NewReader(listing))
	for scanner.Scan() {
		name := path.Base(strings.TrimSpace(scanner.Text()))
		if strings.HasSuffix(name, ".md") && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

func readModel(fsys fs.FS, name string) (*model.Model, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read the model: %w", err)
	}
	m, err := model.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return m, nil
}

// plural returns one when n is 1, and many otherwise.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
