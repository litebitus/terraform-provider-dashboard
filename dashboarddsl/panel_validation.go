package dashboarddsl

func (check *documentCheck) panel(at place, panel Panel) {
	check.repeatFor(at, panel.RepeatFor)
	check.links(at, panel.Links)
	if panel.Kind == TextKind {
		check.textPanel(at, panel)
		return
	}
	if panel.Min != nil && panel.Max != nil && *panel.Min >= *panel.Max {
		check.add(at, "min %v is not below max %v", *panel.Min, *panel.Max)
	}
	for position, threshold := range panel.Thresholds {
		if position > 0 && threshold.Value <= panel.Thresholds[position-1].Value {
			check.add(at.item("thresholds", position), "value %v is not above the threshold before it", threshold.Value)
		}
	}
	check.kindOnly(at, panel.Kind, "orientation", panel.Orientation != "", BarKind)
	check.kindOnly(at, panel.Kind, "location_field", panel.LocationField != "", MapKind)
	check.kindOnly(at, panel.Kind, "content", panel.Content != "", TextKind)
	for position, query := range panel.Queries {
		queryAt := at.item("queries", position)
		check.query(queryAt, query.Query)
		check.source(queryAt.with("source"), query.Source)
		check.references(queryAt, "legend_format", query.LegendFormat)
	}
}

// kindOnly refuses a field written on a panel of a kind other than the one it is for.
func (check *documentCheck) kindOnly(at place, panelKind, field string, written bool, forKind string) {
	if written && panelKind != forKind {
		check.add(at, "%s is for a %s panel", field, forKind)
	}
}

// textPanel holds a text panel to what it is: Markdown, with no values to draw.
func (check *documentCheck) textPanel(at place, panel Panel) {
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
		{"stacked", panel.Stacked},
		{"thresholds", len(panel.Thresholds) > 0},
		{"unit", panel.Unit != ""},
		{"value_mappings", len(panel.ValueMappings) > 0},
	} {
		if field.written {
			check.add(at, "a text panel shows its content and takes no %s", field.name)
		}
	}
}
