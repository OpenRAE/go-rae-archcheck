package record

import (
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-archcheck/internal/finding"
	"github.com/OpenRAE/go-rae-archcheck/internal/model"
)

func record(design, issue int, title string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("---\ndesign: " + strconv.Itoa(design) + "\nissue: " + strconv.Itoa(issue) + "\ntitle: " + title + "\n---\n\n# Body\n")}
}

func TestCheckValid(t *testing.T) {
	fsys := fstest.MapFS{
		"d/0001-start.md":   record(1, 3, "Start"),
		"d/0002-layers.md":  record(2, 4, "Layers"),
		"d/README.txt":      &fstest.MapFile{Data: []byte("not a record")},
		"d/sub/0003-old.md": record(3, 5, "Nested"),
	}
	got, err := Check(fsys, "d")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Check() = %v, want no findings", got)
	}
}

func TestCheckDefects(t *testing.T) {
	fsys := fstest.MapFS{
		"d/0001-start.md":    record(1, 3, "Start"),
		"d/bad name.md":      record(2, 3, "Bad"),
		"d/0003-nofront.md":  &fstest.MapFile{Data: []byte("# No front matter\n")},
		"d/0004-mismatch.md": record(5, 3, "Mismatch"),
		"d/0006-noissue.md":  record(6, 0, "No issue"),
		"d/0001-again.md":    record(1, 3, "Again"),
		"d/0007-unclosed.md": &fstest.MapFile{Data: []byte("---\ndesign: 7\n")},
		"d/0008-unknown.md":  &fstest.MapFile{Data: []byte("---\ndesign: 8\nissue: 1\ntitle: T\nextra: x\n---\n")},
	}
	got, err := Check(fsys, "d")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	want := map[string]string{
		"d/bad name.md":      "the file name is not NNNN-slug.md",
		"d/0003-nofront.md":  "no valid front matter",
		"d/0004-mismatch.md": "states design 5, but the file name states 4",
		"d/0006-noissue.md":  "needs an issue number and a title",
		"d/0001-start.md":    "design number 1 is also used by 0001-again.md",
		"d/0007-unclosed.md": "no valid front matter",
		"d/0008-unknown.md":  "no valid front matter",
	}
	for location, message := range want {
		if !hasFinding(got, location, message) {
			t.Errorf("Check() = %v, want %q at %s", got, message, location)
		}
	}
}

func TestNamesMissingDirectory(t *testing.T) {
	names, err := Names(fstest.MapFS{}, "missing")
	if err != nil || names != nil {
		t.Errorf("Names() = %v, %v, want nil, nil", names, err)
	}
}

func TestCheckChange(t *testing.T) {
	base := &model.Model{Version: 1, Module: "example.com/m", Layers: []model.Layer{{ID: "core", Directory: "internal/core", Description: "Core."}}}
	textOnly := &model.Model{Version: 1, Module: "example.com/m", Layers: []model.Layer{{ID: "core", Directory: "internal/core", Description: "The core."}}}
	structural := &model.Model{Version: 1, Module: "example.com/m", Layers: []model.Layer{{ID: "core", Directory: "internal/kernel", Description: "Core."}}}
	tests := []struct {
		name      string
		head      *model.Model
		headNames []string
		want      int
	}{
		{name: "no change", head: base, headNames: []string{"0001-a.md"}, want: 0},
		{name: "text only", head: textOnly, headNames: []string{"0001-a.md"}, want: 0},
		{name: "structural with record", head: structural, headNames: []string{"0001-a.md", "0002-b.md"}, want: 0},
		{name: "structural without record", head: structural, headNames: []string{"0001-a.md"}, want: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CheckChange(base, tc.head, []string{"0001-a.md"}, tc.headNames, "model.yaml")
			if err != nil {
				t.Fatalf("CheckChange() error = %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("CheckChange() = %v, want %d findings", got, tc.want)
			}
			if tc.want == 1 {
				wantFinding := finding.Finding{
					Location: "model.yaml",
					Rule:     finding.Record,
					Message:  "the change alters the structure of the model, but it adds no design record",
					Fix:      "Add a design record that states the reasons for the structural change, in the same change.",
				}
				if diff := cmp.Diff(wantFinding, got[0]); diff != "" {
					t.Errorf("CheckChange() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func hasFinding(findings []finding.Finding, location, message string) bool {
	for _, f := range findings {
		if f.Location == location && f.Rule == finding.Record && strings.Contains(f.Message, message) {
			return true
		}
	}
	return false
}
