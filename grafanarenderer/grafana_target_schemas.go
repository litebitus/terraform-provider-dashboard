package grafanarenderer

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v4"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/litebitus/terraform-provider-dashboard/targetschema"
)

//go:embed target-schemas
var targetSchemaFiles embed.FS

const targetSchemaFolder = "target-schemas"

// TargetSchema is one Grafana target schema the renderer carries: where its JSON Schema is generated from, and what a render for it states.
type TargetSchema struct {
	ID             targetschema.ID `yaml:"-"`
	GrafanaRelease string          `yaml:"grafana_release"`
	APIVersion     string          `yaml:"api_version"`
	PluginVersion  string          `yaml:"plugin_version"`
	CUEFile        string          `yaml:"cue_file"`
	CUEExpression  string          `yaml:"cue_expression"`
}

// SchemaFileName is the name of the target schema's JSON Schema, beside the manifest and among a release's assets.
func (target TargetSchema) SchemaFileName() string { return target.ID.String() + ".schema.json" }

// GrafanaImage is the Docker Hub image of the target schema's Grafana release.
func (target TargetSchema) GrafanaImage() string {
	return "grafana/grafana:" + strings.TrimPrefix(target.GrafanaRelease, "v")
}

// TargetSchemas are the target schemas the renderer carries, as its manifest states them.
func TargetSchemas() ([]TargetSchema, error) {
	manifest, err := targetSchemaFiles.ReadFile(path.Join(targetSchemaFolder, "target-schemas.yml"))
	if err != nil {
		return nil, err
	}
	var entries map[string]TargetSchema
	if err := yaml.Load(manifest, &entries, yaml.WithKnownFields()); err != nil {
		return nil, fmt.Errorf("the target schema manifest: %w", err)
	}
	var carried []TargetSchema
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		id, err := targetschema.ParseID(name)
		if err != nil {
			return nil, fmt.Errorf("the target schema manifest: %w", err)
		}
		entry := entries[name]
		entry.ID = id
		carried = append(carried, entry)
	}
	return carried, nil
}

// TargetSchemaFiles are the carried target schemas' JSON Schemas, by file name, for a release to publish: every carried minor's, and each major's as the minor it resolves to.
func TargetSchemaFiles() (map[string][]byte, error) {
	carried, err := TargetSchemas()
	if err != nil {
		return nil, err
	}
	carriedIDs, err := CarriedIDs()
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, target := range carried {
		text, err := targetSchemaFiles.ReadFile(path.Join(targetSchemaFolder, target.SchemaFileName()))
		if err != nil {
			return nil, fmt.Errorf("target schema %s has no JSON Schema; run make target-schemas: %w", target.ID, err)
		}
		files[target.SchemaFileName()] = text
	}
	for _, target := range carried {
		major := targetschema.ID{Kind: target.ID.Kind, Version: strings.SplitN(target.ID.Version, ".", 2)[0]}
		newest, err := targetschema.Resolve(major, carriedIDs)
		if err != nil {
			return nil, err
		}
		files[major.String()+".schema.json"] = files[newest.String()+".schema.json"]
	}
	return files, nil
}

// CarriedIDs are the identifiers of the target schemas the renderer carries.
func CarriedIDs() ([]targetschema.ID, error) {
	carried, err := TargetSchemas()
	if err != nil {
		return nil, err
	}
	ids := make([]targetschema.ID, len(carried))
	for index, target := range carried {
		ids[index] = target.ID
	}
	return ids, nil
}

func targetSchemaFor(id targetschema.ID) (TargetSchema, error) {
	carried, err := TargetSchemas()
	if err != nil {
		return TargetSchema{}, err
	}
	for _, target := range carried {
		if target.ID == id {
			return target, nil
		}
	}
	return TargetSchema{}, fmt.Errorf("target schema %s is not one this renderer carries", id)
}

var (
	compiledTargetSchemas sync.Map
	printer               = message.NewPrinter(language.English)
)

// conforms holds a rendered dashboard spec to its target's JSON Schema; a spec that does not conform is the renderer's defect.
func conforms(target TargetSchema, spec any) error {
	schema, err := compiledTargetSchema(target)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	var invalid *jsonschema.ValidationError
	if err := schema.Validate(instance); !errors.As(err, &invalid) {
		return err
	}
	var problems []string
	for _, leaf := range leafErrors(invalid) {
		problems = append(problems, "/"+strings.Join(leaf.InstanceLocation, "/")+": "+leaf.ErrorKind.LocalizedString(printer))
	}
	slices.Sort(problems)
	return fmt.Errorf("the render does not conform to target schema %s, a defect of this provider:\n%s",
		target.ID, strings.Join(slices.Compact(problems), "\n"))
}

func compiledTargetSchema(target TargetSchema) (*jsonschema.Schema, error) {
	if compiled, held := compiledTargetSchemas.Load(target.ID); held {
		return compiled.(*jsonschema.Schema), nil
	}
	text, err := targetSchemaFiles.ReadFile(path.Join(targetSchemaFolder, target.SchemaFileName()))
	if err != nil {
		return nil, fmt.Errorf("target schema %s has no JSON Schema: %w", target.ID, err)
	}
	parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(text))
	if err != nil {
		return nil, fmt.Errorf("target schema %s is not JSON: %w", target.ID, err)
	}
	url := "https://github.com/litebitus/terraform-provider-dashboard/grafanarenderer/target-schemas/" + target.SchemaFileName()
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(url, parsed); err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("target schema %s does not compile: %w", target.ID, err)
	}
	compiledTargetSchemas.Store(target.ID, compiled)
	return compiled, nil
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
