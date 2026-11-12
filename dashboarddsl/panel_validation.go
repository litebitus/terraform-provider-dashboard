package dashboarddsl

import (
	"fmt"
	"slices"

	"github.com/litebitus/dql-compiler/dqlsyntax"
	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
)

func (check *documentCheck) panel(at documentPath, panel Panel) {
	check.repeatFor(at.field("repeat_for"), panel.RepeatFor)
	check.links(at.field("links"), panel.Links)
	if panel.Kind == TextKind {
		check.textPanel(at, panel)
		return
	}
	if panel.Min != nil && panel.Max != nil && *panel.Min >= *panel.Max {
		check.addAtValue(at.field("min"), "`min` %s is not below `max` %s",
			yamldslspec.CodeSpan(fmt.Sprint(*panel.Min)), yamldslspec.CodeSpan(fmt.Sprint(*panel.Max)))
	}
	for position, threshold := range panel.Thresholds {
		if position == 0 || threshold.Value == nil {
			continue
		}
		previous := panel.Thresholds[position-1].Value
		if previous != nil && *threshold.Value <= *previous {
			check.addAtValue(at.field("thresholds").item(position).field("value"),
				"`value` %s is not above the threshold before it", yamldslspec.CodeSpan(fmt.Sprint(*threshold.Value)))
		}
	}
	check.kindOnly(at, panel.Kind, "orientation", panel.Orientation != "", BarKind)
	check.kindOnly(at, panel.Kind, "value_order", panel.ValueOrder != "", BarKind)
	check.kindOnly(at, panel.Kind, "row_order", len(panel.RowOrder) > 0, TableKind)
	if panel.Kind == TableKind {
		check.rowOrder(at.field("row_order"), panel)
	}
	check.kindOnly(at, panel.Kind, "location_field", panel.LocationField != "", MapKind)
	check.kindOnly(at, panel.Kind, "content", panel.Content != "", TextKind)
	for position, query := range panel.Queries {
		queryAt := at.field("queries").item(position)
		check.query(queryAt.field("query"), query.Query)
		check.source(queryAt.field("source"), query.Source)
		check.references(queryAt.field("legend_format"), "legend_format", query.LegendFormat)
		check.kindOnly(queryAt, panel.Kind, "value_column_header", query.ValueColumnHeader != "", TableKind)
		check.kindOnly(queryAt, panel.Kind, "value_format", query.ValueFormat != "", TableKind)
		check.valueFormat(queryAt.field("value_format"), query.ValueFormat)
	}
}

// rowOrder holds a table's row_order to columns the table shows, each ordered once.
func (check *documentCheck) rowOrder(at documentPath, panel Panel) {
	shownColumns := tableColumns(panel.Queries)
	orderedColumns := map[string]bool{}
	for position, rowOrderColumn := range panel.RowOrder {
		column := rowOrderColumn.Column
		columnAt := at.item(position).field("column")
		switch {
		case orderedColumns[column]:
			check.addAtValue(columnAt, "column %s is ordered by an earlier entry", yamldslspec.CodeSpan(column))
		case !slices.Contains(shownColumns, column):
			check.addAtValue(columnAt,
				"column %s is not one the table shows: a query's `value_column_header`, or a field a query groups by",
				yamldslspec.CodeSpan(column))
		}
		orderedColumns[column] = true
	}
}

// tableColumns are the columns a table's queries show by name: each query's value_column_header, and the fields
// its result carries where they are known.
func tableColumns(queries []Query) []string {
	var columns []string
	for _, query := range queries {
		if query.ValueColumnHeader != "" {
			columns = append(columns, query.ValueColumnHeader)
		}
		parsed, err := dqlsyntax.ParseQuery(query.Query, QueryGrammarVersion())
		if err != nil {
			continue
		}
		if fields, known := parsed.QueryResultFields(); known {
			columns = append(columns, fields...)
		}
	}
	return columns
}

// valueFormat holds a query's value_format to a layout of the format function whose one verb takes the value.
func (check *documentCheck) valueFormat(at documentPath, layout string) {
	if layout == "" {
		return
	}
	layoutParts, err := yamldslspec.ParseLayout(layout)
	if err != nil {
		check.addAtValue(at, "`value_format`: %v", err)
		return
	}
	verbCount := 0
	for _, layoutPart := range layoutParts {
		if layoutPart.Verb != nil {
			verbCount++
		}
	}
	if verbCount != 1 {
		check.addAtValue(at, "`value_format` holds one verb, for the query's value, not %d", verbCount)
	}
}

// kindOnly refuses a field written on a panel of a kind other than the one it is for.
func (check *documentCheck) kindOnly(at documentPath, panelKind, field string, written bool, forKind string) {
	if written && panelKind != forKind {
		check.addAtKey(at.field(field), "%s is for a %s panel", yamldslspec.CodeSpan(field), yamldslspec.CodeSpan(forKind))
	}
}

// textPanel holds a text panel to what it is: Markdown, with no values to draw.
func (check *documentCheck) textPanel(at documentPath, panel Panel) {
	for _, field := range []struct {
		name    string
		written bool
	}{
		{"axis", panel.Axis != nil},
		{"calculation", panel.Calculation != ""},
		{"decimals", panel.Decimals != nil},
		{"legend", panel.Legend != nil},
		{"location_field", panel.LocationField != ""},
		{"max", panel.Max != nil},
		{"min", panel.Min != nil},
		{"orientation", panel.Orientation != ""},
		{"queries", len(panel.Queries) > 0},
		{"row_order", len(panel.RowOrder) > 0},
		{"stacked", panel.Stacked},
		{"thresholds", len(panel.Thresholds) > 0},
		{"unit", panel.Unit != ""},
		{"value_mappings", len(panel.ValueMappings) > 0},
		{"value_order", panel.ValueOrder != ""},
	} {
		if field.written {
			check.addAtKey(at.field(field.name),
				"a `text` panel shows its `content` and takes no %s", yamldslspec.CodeSpan(field.name))
		}
	}
}
