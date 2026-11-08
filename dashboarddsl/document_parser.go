package dashboarddsl

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
	"github.com/litebitus/yaml-dsl-spec/go/diagnostic_renderer"
	"go.yaml.in/yaml/v4"
)

const (
	dashboardSourceName = "dashboard"
	untitledSourceName  = dashboardSourceName + ":{title}"
)

// ParseDashboard reads a dashboard document from its text and holds it to the DSL: its shape to the schema its schema_version names, then the rules a schema cannot state, answering every problem found.
func ParseDashboard(specText string) (Dashboard, error) {
	documentRoot, readProblem := yamldslspec.ParseYAML(dashboardSourceName, specText, nil)
	if readProblem != nil {
		readSource := diagnostic_renderer.DiagnosticSource{Name: untitledSourceName, Text: specText}
		return Dashboard{}, problemReport(readSource, []yamldslspec.Diagnostic{*readProblem})
	}
	source := diagnostic_renderer.DiagnosticSource{Name: sourceName(documentRoot), Text: specText}
	if documentRoot.Kind == yamldslspec.NodeNull {
		emptyProblem := yamldslspec.DiagnosticAt(documentRoot.Position, "the document is empty")
		return Dashboard{}, problemReport(source, []yamldslspec.Diagnostic{emptyProblem})
	}
	shapeProblems, err := validateShape(documentRoot)
	if err != nil {
		return Dashboard{}, err
	}
	if len(shapeProblems) > 0 {
		return Dashboard{}, problemReport(source, shapeProblems)
	}
	var dashboard Dashboard
	if err := yaml.Unmarshal([]byte(specText), &dashboard); err != nil {
		return Dashboard{}, problemReport(source, []yamldslspec.Diagnostic{yamldslspec.UnplacedDiagnostic(err.Error())})
	}
	if ruleProblems := validateDashboard(dashboard, documentRoot); len(ruleProblems) > 0 {
		return Dashboard{}, problemReport(source, ruleProblems)
	}
	return dashboard, nil
}

func sourceName(documentRoot yamldslspec.Node) string {
	titleNode, _ := documentRoot.Get("title")
	title, isText := titleNode.AsString()
	if !isText {
		return untitledSourceName
	}
	var encodedTitle strings.Builder
	titleEncoder := json.NewEncoder(&encodedTitle)
	titleEncoder.SetEscapeHTML(false)
	_ = titleEncoder.Encode(title)
	return dashboardSourceName + ":" + strings.TrimSuffix(encodedTitle.String(), "\n")
}

func problemReport(source diagnostic_renderer.DiagnosticSource, problems []yamldslspec.Diagnostic) error {
	orderedProblems := slices.Clone(problems)
	slices.SortStableFunc(orderedProblems, compareProblemPositions)
	renderedProblems := make([]string, 0, len(orderedProblems)+1)
	for _, problem := range orderedProblems {
		renderedProblems = append(renderedProblems, diagnostic_renderer.RenderDiagnostic(
			problem, diagnostic_renderer.SeverityError, source, diagnostic_renderer.RenderStyle{}))
	}
	problemCount := "1 error"
	if len(orderedProblems) != 1 {
		problemCount = fmt.Sprintf("%d errors", len(orderedProblems))
	}
	return errors.New(strings.Join(append(renderedProblems, problemCount), "\n\n"))
}

func compareProblemPositions(firstProblem, secondProblem yamldslspec.Diagnostic) int {
	switch {
	case firstProblem.Position == nil && secondProblem.Position == nil:
		return 0
	case firstProblem.Position == nil:
		return -1
	case secondProblem.Position == nil:
		return 1
	}
	return cmp.Or(
		cmp.Compare(firstProblem.Position.Line, secondProblem.Position.Line),
		cmp.Compare(firstProblem.Position.Column, secondProblem.Position.Column),
	)
}
