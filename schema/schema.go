// Package schema holds the JSON schema of the architecture model, version 1,
// and validates a decoded model against it. The schema states the structure
// of the model, and the checker adds the semantic rules that the schema
// cannot express.
package schema

import (
	"bytes"
	// The go:embed directive below needs the embed package.
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// modelSchema is the JSON schema of the model. Editors can use the same file
// through its identifier.
//
//go:embed model.schema.json
var modelSchema []byte

// id is the identifier of the schema, as its $id states.
const id = "https://github.com/OpenRAE/go-rae-archcheck/schema/model.schema.json"

// A Violation is one value of a model that breaks the schema.
type Violation struct {
	// Location is the JSON pointer of the value in the model, such as
	// /layers/0/id. The root is the empty string.
	Location string
	// Message states the constraint that the value breaks.
	Message string
}

// Validate validates a model document against the schema. The document is
// the result of decoding a YAML or JSON model into maps with string keys,
// slices and scalars. Validate returns the violations in document order, or
// an error when the document has a value that JSON cannot hold.
func Validate(document any) ([]Violation, error) {
	sch, err := compile(modelSchema)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("convert the model to JSON: %w", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("convert the model to JSON: %w", err)
	}
	err = sch.Validate(instance)
	var invalid *jsonschema.ValidationError
	if !errors.As(err, &invalid) {
		return nil, err
	}
	var violations []Violation
	collect(invalid, message.NewPrinter(language.English), &violations)
	slices.SortStableFunc(violations, func(a, b Violation) int { return strings.Compare(a.Location, b.Location) })
	return slices.Compact(violations), nil
}

// compile compiles the schema in data under the identifier of the model
// schema.
func compile(data []byte) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("read the model schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err = c.AddResource(id, doc); err != nil {
		return nil, fmt.Errorf("add the model schema: %w", err)
	}
	sch, err := c.Compile(id)
	if err != nil {
		return nil, fmt.Errorf("compile the model schema: %w", err)
	}
	return sch, nil
}

// collect appends the leaf errors of e, which name the constraints that the
// values break, in the words of p.
func collect(e *jsonschema.ValidationError, p *message.Printer, violations *[]Violation) {
	if len(e.Causes) > 0 {
		for _, cause := range e.Causes {
			collect(cause, p, violations)
		}
		return
	}
	location := ""
	if len(e.InstanceLocation) > 0 {
		location = "/" + strings.Join(e.InstanceLocation, "/")
	}
	*violations = append(*violations, Violation{Location: location, Message: e.ErrorKind.LocalizedString(p)})
}
