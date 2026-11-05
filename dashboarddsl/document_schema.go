package dashboarddsl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/litebitus/terraform-provider-dashboard/schemas"
)

// compiledSchemas are the document schemas the provider carries, compiled once, by minor version.
var compiledSchemas = map[string]*jsonschema.Schema{}

func init() {
	for _, stated := range schemas.StatedVersions() {
		text, minorVersion, err := schemas.SchemaFor(stated)
		if err != nil {
			panic(fmt.Sprintf("schema version %s cannot be read: %v", stated, err))
		}
		if _, compiled := compiledSchemas[minorVersion]; compiled {
			continue
		}
		parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(text))
		if err != nil {
			panic(fmt.Sprintf("schema version %s is not JSON: %v", minorVersion, err))
		}
		url := "https://github.com/litebitus/terraform-provider-dashboard/schemas/dashboard-dsl-v" + minorVersion + ".schema.json"
		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource(url, parsed); err != nil {
			panic(fmt.Sprintf("schema version %s is not a schema: %v", minorVersion, err))
		}
		compiledSchemas[minorVersion] = compiler.MustCompile(url)
	}
}

// validateShape holds a decoded document to the schema its schema_version names, naming each problem at its place.
func validateShape(document any) error {
	fields, isMap := document.(map[string]any)
	if !isMap {
		return errors.New("a dashboard document is a mapping of its fields")
	}
	stated, isText := fields["schema_version"].(string)
	if !isText {
		return fmt.Errorf("schema_version is required, quoted, one of %q", schemas.StatedVersions())
	}
	_, minorVersion, err := schemas.SchemaFor(stated)
	if err != nil {
		return fmt.Errorf("schema_version %w", err)
	}
	encoded, err := json.Marshal(withStringKeys(document))
	if err != nil {
		return err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	var invalid *jsonschema.ValidationError
	if err := compiledSchemas[minorVersion].Validate(instance); !errors.As(err, &invalid) {
		return err
	}
	printer := message.NewPrinter(language.English)
	var problems []string
	for _, leaf := range leafErrors(invalid) {
		problems = append(problems, placeOf(leaf.InstanceLocation).problem("%s", leaf.ErrorKind.LocalizedString(printer)).Error())
	}
	slices.Sort(problems)
	return errors.New(strings.Join(slices.Compact(problems), "\n"))
}

func leafErrors(validation *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(validation.Causes) == 0 {
		return []*jsonschema.ValidationError{validation}
	}
	var leaves []*jsonschema.ValidationError
	for _, cause := range validation.Causes {
		leaves = append(leaves, leafErrors(cause)...)
	}
	return leaves
}

// placeOf is an instance location as a reader counts it: ["rows", "0", "panels", "1"] is row 1, panel 2.
func placeOf(location []string) place {
	var at place
	for index := 0; index < len(location); index++ {
		token := location[index]
		if index+1 < len(location) {
			if _, listed := listItemNames[token]; listed {
				position, _ := strconv.Atoi(location[index+1])
				at, index = at.item(token, position), index+1
				continue
			}
			if _, mapped := mapEntryNames[token]; mapped {
				at, index = at.entry(token, location[index+1]), index+1
				continue
			}
		}
		at = at.with(token)
	}
	return at
}

// withStringKeys is a decoded YAML value whose every map is keyed by text, as JSON is: a YAML key such as 0 reads as "0".
func withStringKeys(value any) any {
	switch node := value.(type) {
	case map[string]any:
		converted := make(map[string]any, len(node))
		for key, item := range node {
			converted[key] = withStringKeys(item)
		}
		return converted
	case map[any]any:
		converted := make(map[string]any, len(node))
		for key, item := range node {
			converted[fmt.Sprint(key)] = withStringKeys(item)
		}
		return converted
	case []any:
		converted := make([]any, len(node))
		for index, item := range node {
			converted[index] = withStringKeys(item)
		}
		return converted
	}
	return value
}
