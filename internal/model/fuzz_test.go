package model

import "testing"

// FuzzParse checks that Parse never panics, and that it returns exactly one
// of a model, schema findings or an error. A model then passes Validate and
// Structure without a panic.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{validYAML, "", "version: 1\n", "---\n---\n", "version: .inf\n", "[1, 2]\n"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		m, findings, err := Parse(data, "model.yaml")
		switch {
		case err != nil:
			if m != nil || findings != nil {
				t.Fatalf("Parse() = %v, %v with error %v, want only the error", m, findings, err)
			}
		case m == nil:
			if len(findings) == 0 {
				t.Fatal("Parse() returned no model, no findings and no error")
			}
		default:
			if len(findings) != 0 {
				t.Fatalf("Parse() returned a model with findings %v", findings)
			}
			_ = Validate(m, "model.yaml")
			_ = m.Structure()
		}
	})
}
