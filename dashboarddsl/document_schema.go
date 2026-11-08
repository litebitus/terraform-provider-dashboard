package dashboarddsl

import (
	"fmt"
	"strconv"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"

	"github.com/litebitus/terraform-provider-dashboard/schemas"
)

// documentSchemas are the document schemas the provider carries, each loaded once.
var documentSchemas = yamldslspec.NewSchemaCache()

// validateShape holds a document to the schema its schema_version names, answering every problem; an error where that schema does not load.
func validateShape(documentRoot yamldslspec.Node) ([]yamldslspec.Diagnostic, error) {
	if documentRoot.Kind != yamldslspec.NodeMap {
		return []yamldslspec.Diagnostic{
			yamldslspec.DiagnosticAt(documentRoot.Position, "a dashboard document is a mapping of its fields"),
		}, nil
	}
	versionNode, isWritten := documentRoot.Get("schema_version")
	stated, isText := versionNode.AsString()
	if !isText {
		var quotedVersions []string
		for _, version := range schemas.StatedVersions() {
			quotedVersions = append(quotedVersions, strconv.Quote(version))
		}
		versionPosition := documentRoot.Position
		if isWritten {
			versionPosition = versionNode.Position
		}
		return []yamldslspec.Diagnostic{
			yamldslspec.DiagnosticAt(versionPosition, "`schema_version` is required, quoted, one of "+oneOf(quotedVersions)),
		}, nil
	}
	schemaText, minorVersion, err := schemas.SchemaFor(stated)
	if err != nil {
		return []yamldslspec.Diagnostic{
			yamldslspec.DiagnosticAt(versionNode.Position, "`schema_version` "+err.Error()),
		}, nil
	}
	loadedSchema, err := documentSchemas.Load("dashboard-dsl-v"+minorVersion+".schema.json", string(schemaText))
	if err != nil {
		return nil, fmt.Errorf("schema version %s cannot be read: %w", minorVersion, err)
	}
	return loadedSchema.Validate(yamldslspec.ResolvedValueFromNode(documentRoot), yamldslspec.ValidationRules{}), nil
}
