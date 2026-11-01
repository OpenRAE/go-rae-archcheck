package record

import "testing"

// FuzzParseFrontMatter checks that parseFrontMatter never panics, and that it
// returns the zero front matter when it reports no front matter.
func FuzzParseFrontMatter(f *testing.F) {
	for _, seed := range []string{"---\ndesign: 1\nissue: 1\ntitle: Start\n---\n", "---\n---\n", "", "---\ndesign: [\n---\n"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, content []byte) {
		meta, ok := parseFrontMatter(content)
		if !ok && meta != (frontMatter{}) {
			t.Fatalf("parseFrontMatter() = %+v, false, want the zero front matter", meta)
		}
	})
}
