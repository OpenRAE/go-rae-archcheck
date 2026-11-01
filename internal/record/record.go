// Package record checks the design records of a model: their names and
// front matter, and a new record for each structural change of the model.
package record

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/OpenRAE/go-rae-archcheck/internal/finding"
	"github.com/OpenRAE/go-rae-archcheck/internal/model"
)

// namePattern matches the file name of a design record: a four-digit number,
// a slug of lowercase words joined by hyphens, and the .md extension.
var namePattern = regexp.MustCompile(`^([0-9]{4})-[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)

// frontMatter is the metadata block at the start of a design record.
type frontMatter struct {
	Design int    `yaml:"design"`
	Issue  int    `yaml:"issue"`
	Title  string `yaml:"title"`
}

// Check returns the defects of the design records in dir of fsys. Each
// record is a Markdown file that starts with front matter between two "---"
// lines, with its number, its issue and its title.
func Check(fsys fs.FS, dir string) ([]finding.Finding, error) {
	names, err := Names(fsys, dir)
	if err != nil {
		return nil, err
	}
	var findings []finding.Finding
	numbers := map[int]string{}
	for _, name := range names {
		location := path.Join(dir, name)
		add := func(message string) {
			findings = append(findings, finding.Finding{
				Location: location,
				Rule:     finding.Record,
				Message:  message,
				Fix:      "Name the record NNNN-slug.md, and start it with front matter that states its design number, its issue and its title.",
			})
		}
		match := namePattern.FindStringSubmatch(name)
		if match == nil {
			add("the file name is not NNNN-slug.md")
			continue
		}
		content, err := fs.ReadFile(fsys, location)
		if err != nil {
			return nil, err
		}
		meta, ok := parseFrontMatter(content)
		number, _ := strconv.Atoi(match[1])
		switch {
		case !ok:
			add("the record has no valid front matter")
		case meta.Design != number:
			add(fmt.Sprintf("the front matter states design %d, but the file name states %d", meta.Design, number))
		case meta.Issue <= 0 || strings.TrimSpace(meta.Title) == "":
			add("the front matter needs an issue number and a title")
		}
		if prior, used := numbers[number]; used {
			add(fmt.Sprintf("design number %d is also used by %s", number, prior))
		}
		numbers[number] = name
	}
	return finding.Sort(findings), nil
}

// Names returns the names of the Markdown files in dir of fsys, in order. A
// missing directory has no records.
func Names(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("list the design records: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}

// parseFrontMatter decodes the front matter at the start of content.
func parseFrontMatter(content []byte) (frontMatter, bool) {
	rest, ok := bytes.CutPrefix(content, []byte("---\n"))
	if !ok {
		return frontMatter{}, false
	}
	block, _, ok := bytes.Cut(rest, []byte("\n---\n"))
	if !ok {
		return frontMatter{}, false
	}
	var meta frontMatter
	if err := model.DecodeYAML(block, &meta); err != nil {
		return frontMatter{}, false
	}
	return meta, true
}

// CheckChange returns a finding when the structure of the model changed
// between base and head, and no record name is new in head. base and head
// are the models at the base revision and in the change; baseNames and
// headNames are the record file names at the same revisions.
func CheckChange(base, head *model.Model, baseNames, headNames []string, location string) ([]finding.Finding, error) {
	same, err := sameStructure(base, head)
	if err != nil {
		return nil, err
	}
	if same || slices.ContainsFunc(headNames, func(name string) bool { return !slices.Contains(baseNames, name) }) {
		return nil, nil
	}
	return []finding.Finding{{
		Location: location,
		Rule:     finding.Record,
		Message:  "the change alters the structure of the model, but it adds no design record",
		Fix:      "Add a design record that states the reasons for the structural change, in the same change.",
	}}, nil
}

// sameStructure reports whether two models have the same structure. It
// compares the YAML encodings of their structures, which follow the field
// order of the types, so descriptive text and formatting do not matter.
func sameStructure(a, b *model.Model) (bool, error) {
	ya, err := yaml.Marshal(a.Structure())
	if err != nil {
		return false, fmt.Errorf("encode the base model: %w", err)
	}
	yb, err := yaml.Marshal(b.Structure())
	if err != nil {
		return false, fmt.Errorf("encode the model: %w", err)
	}
	return bytes.Equal(ya, yb), nil
}
