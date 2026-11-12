// Package model reads and validates an architecture model.
//
// A model describes one Go module: its C4 context and containers, the layers
// that own its directories, the components that own its packages, and the
// rules that forbid imports and symbols in parts of the code. The format is
// version 1, which docs/model.md describes.
package model

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"

	"github.com/OpenRAE/go-rae-archcheck/internal/finding"
	"github.com/OpenRAE/go-rae-archcheck/schema"
)

// Model is the architecture model of a Go module.
type Model struct {
	// Version is the format version of the model.
	Version int `yaml:"version"`
	// Module is the path of the Go module that the model describes.
	Module string `yaml:"module"`
	// System is the software system.
	System Element `yaml:"system"`
	// People are the people who use the system.
	People []Element `yaml:"people"`
	// ExternalSystems are the systems outside the system.
	ExternalSystems []Element `yaml:"external_systems"`
	// Relationships connect the system, people, external systems and containers.
	Relationships []Relationship `yaml:"relationships"`
	// Containers are the deployable and runtime units of the system.
	Containers []Container `yaml:"containers"`
	// Layers own the directories of the module.
	Layers []Layer `yaml:"layers"`
	// Components own the packages of the module.
	Components []Component `yaml:"components"`
	// TestModules are the external modules that any test file can import.
	TestModules []string `yaml:"test_modules"`
	// TestSupport are the packages that only test files and other test
	// support packages can import.
	TestSupport []string `yaml:"test_support"`
	// Rules forbid imports and symbols in parts of the code.
	Rules []Rule `yaml:"rules"`
}

// An Element is the system, a person or an external system.
type Element struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// A Relationship connects two elements or containers.
type Relationship struct {
	From        string `yaml:"from"`
	To          string `yaml:"to"`
	Description string `yaml:"description"`
	Technology  string `yaml:"technology"`
}

// A Container is a deployable or runtime unit of the system.
type Container struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Technology  string `yaml:"technology"`
	Description string `yaml:"description"`
}

// A Layer owns one directory of the module and every package below it.
type Layer struct {
	ID          string `yaml:"id"`
	Directory   string `yaml:"directory"`
	Description string `yaml:"description"`
	// Uses are the other layers that the packages of the layer can import.
	Uses []string `yaml:"uses"`
	// External are the packages outside the module and the standard library
	// that the packages of the layer can import.
	External []External `yaml:"external"`
	// Isolated forbids dependencies between the components of the layer.
	Isolated bool `yaml:"isolated"`
}

// An External allows the packages of a layer to import one package, or the
// packages below a path in the same module when the path ends in "/...".
type External struct {
	Path   string `yaml:"path"`
	Reason string `yaml:"reason"`
}

// A Component owns packages of the module.
type Component struct {
	ID string `yaml:"id"`
	// Containers are the containers that run the component.
	Containers  []string `yaml:"containers"`
	Description string   `yaml:"description"`
	// Packages are import paths, and a path that ends in "/..." also owns the
	// packages below it.
	Packages []string `yaml:"packages"`
	// DependsOn are the components that the component can import.
	DependsOn []string `yaml:"depends_on"`
	// Modules are the external modules that the component can import.
	Modules []string `yaml:"modules"`
}

// A Rule forbids imports and symbols in a scope of the code.
type Rule struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Scope       Scope  `yaml:"scope"`
	// ForbidImports are import paths, and a path that ends in "/..." also
	// matches the paths below it.
	ForbidImports []string `yaml:"forbid_imports"`
	// ForbidSymbols are "importpath.Name" symbols or builtin function names.
	ForbidSymbols []string `yaml:"forbid_symbols"`
	// Fix states how to correct a finding of the rule.
	Fix string `yaml:"fix"`
	// ExceptComponents are the components that the rule does not apply to.
	ExceptComponents []string `yaml:"except_components"`
}

// A Scope selects the code that a rule applies to.
type Scope struct {
	// Layers are the layers whose packages the rule applies to.
	Layers []string `yaml:"layers"`
	// TestFiles makes the rule apply to test files and test support packages.
	TestFiles bool `yaml:"test_files"`
}

// Parse decodes a model and validates it against the JSON schema of the
// model format. It returns a finding for each value that breaks the schema,
// with location as the path of the model file. It returns an error when the
// data is not one YAML document.
func Parse(data []byte, location string) (*Model, []finding.Finding, error) {
	var document any
	if err := DecodeYAML(data, &document); err != nil {
		return nil, nil, fmt.Errorf("decode the model: %w", err)
	}
	violations, err := schema.Validate(document)
	if err != nil {
		return nil, nil, fmt.Errorf("validate the model: %w", err)
	}
	if len(violations) > 0 {
		findings := make([]finding.Finding, len(violations))
		for i, v := range violations {
			findings[i] = finding.Finding{
				Location: location,
				Rule:     finding.Model,
				Message:  fmt.Sprintf("the value at %q breaks the schema: %s", v.Location, v.Message),
				Fix:      modelFix,
			}
		}
		return nil, findings, nil
	}
	var m Model
	if err := DecodeYAML(data, &m); err != nil {
		return nil, nil, fmt.Errorf("decode the model: %w", err)
	}
	return &m, nil, nil
}

// DecodeYAML decodes one YAML document into v. Fields that a struct in v
// does not declare fail, and a second document fails.
func DecodeYAML(data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decode the YAML document: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("decode the YAML document: more than one document")
	}
	return nil
}

// Structure returns a copy of the model without its descriptive text: names,
// descriptions, technologies, reasons and fix messages. Two models with the
// same structure differ only in text, so a change between them needs no
// design record.
func (m *Model) Structure() *Model {
	s := *m
	s.System = Element{ID: m.System.ID}
	s.People = elementIDs(m.People)
	s.ExternalSystems = elementIDs(m.ExternalSystems)
	s.Relationships = make([]Relationship, len(m.Relationships))
	for i, r := range m.Relationships {
		s.Relationships[i] = Relationship{From: r.From, To: r.To}
	}
	s.Containers = make([]Container, len(m.Containers))
	for i, c := range m.Containers {
		s.Containers[i] = Container{ID: c.ID}
	}
	s.Layers = make([]Layer, len(m.Layers))
	for i, l := range m.Layers {
		l.Description = ""
		l.External = make([]External, len(m.Layers[i].External))
		for j, e := range m.Layers[i].External {
			l.External[j] = External{Path: e.Path}
		}
		s.Layers[i] = l
	}
	s.Components = make([]Component, len(m.Components))
	for i, c := range m.Components {
		c.Description = ""
		s.Components[i] = c
	}
	s.Rules = make([]Rule, len(m.Rules))
	for i, r := range m.Rules {
		r.Description, r.Fix = "", ""
		s.Rules[i] = r
	}
	return &s
}

func elementIDs(elements []Element) []Element {
	ids := make([]Element, len(elements))
	for i, e := range elements {
		ids[i] = Element{ID: e.ID}
	}
	return ids
}
