package dashboarddsl

import (
	"errors"

	"go.yaml.in/yaml/v4"
)

// ParseDashboard reads a dashboard document from its text and holds it to the DSL: its shape to the schema its schema_version names, then the rules a schema cannot state, answering every problem found.
func ParseDashboard(specText string) (Dashboard, error) {
	var document any
	if err := yaml.Unmarshal([]byte(specText), &document); err != nil {
		return Dashboard{}, err
	}
	if document == nil {
		return Dashboard{}, errors.New("the document is empty")
	}
	if err := validateShape(document); err != nil {
		return Dashboard{}, err
	}
	var dashboard Dashboard
	if err := yaml.Unmarshal([]byte(specText), &dashboard); err != nil {
		return Dashboard{}, err
	}
	if err := validateDashboard(dashboard); err != nil {
		return Dashboard{}, err
	}
	return dashboard, nil
}
