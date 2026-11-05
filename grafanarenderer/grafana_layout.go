package grafanarenderer

import (
	"fmt"
	"math"

	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
)

// Grafana's grid: 24 columns, and rows of 30 pixels with 8 between them.
const (
	gridColumns    = 24
	gridCellHeight = 30
	gridCellGap    = 8
)

type rowsLayoutRowSpec struct {
	Title    string         `json:"title"`
	Collapse bool           `json:"collapse"`
	Repeat   *repeatOptions `json:"repeat,omitempty"`
	Layout   kindSpec       `json:"layout"`
}

type gridLayoutItemSpec struct {
	X       int              `json:"x"`
	Y       int              `json:"y"`
	Width   int              `json:"width"`
	Height  int              `json:"height"`
	Element elementReference `json:"element"`
	Repeat  *repeatOptions   `json:"repeat,omitempty"`
}

type elementReference struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type repeatOptions struct {
	Mode      string `json:"mode"`
	Value     string `json:"value"`
	Direction string `json:"direction,omitempty"`
}

// layOutRows places the document's rows as Grafana's rows layout, each row's panels left to right on its own grid, and answers the panels as elements by name.
func (render *dashboardRender) layOutRows() (map[string]element, kindSpec) {
	elements := map[string]element{}
	rows := []kindSpec{}
	nextID := 1
	for rowPosition, row := range render.dashboard.Rows {
		widths, err := panelWidths(row.Panels)
		if err != nil {
			render.refuse(fmt.Sprintf("row %d %q", rowPosition+1, row.Title), err.Error())
		}
		items := []kindSpec{}
		x := 0
		for panelPosition, panel := range row.Panels {
			name := fmt.Sprintf("panel-%d", nextID)
			elements[name] = render.panel(panelPlace(rowPosition, row, panelPosition, panel), nextID, panel)
			item := gridLayoutItemSpec{
				X: x, Y: 0, Width: widths[panelPosition], Height: gridHeight(panel.Height),
				Element: elementReference{Kind: "ElementReference", Name: name},
			}
			if panel.RepeatFor != "" {
				item.Repeat = &repeatOptions{Mode: "variable", Value: panel.RepeatFor, Direction: "h"}
			}
			items = append(items, kindSpec{Kind: "GridLayoutItem", Spec: item})
			x += widths[panelPosition]
			nextID++
		}
		rowSpec := rowsLayoutRowSpec{
			Title: row.Title, Collapse: row.Collapsed,
			Layout: kindSpec{Kind: "GridLayout", Spec: map[string]any{"items": items}},
		}
		if row.RepeatFor != "" {
			rowSpec.Repeat = &repeatOptions{Mode: "variable", Value: row.RepeatFor}
		}
		rows = append(rows, kindSpec{Kind: "RowsLayoutRow", Spec: rowSpec})
	}
	return elements, kindSpec{Kind: "RowsLayout", Spec: map[string]any{"rows": rows}}
}

// panelWidths shares the grid's columns among a row's panels: a stated width is its share of the row, and the panels without one share what the rest leave equally, any column left over going to the first of them.
func panelWidths(panels []dashboarddsl.Panel) ([]int, error) {
	widths := make([]int, len(panels))
	stated, unstated := 0, 0
	for index, panel := range panels {
		if panel.Width == nil {
			unstated++
			continue
		}
		widths[index] = max(1, int(math.Round(float64(*panel.Width)*gridColumns/100)))
		stated += widths[index]
	}
	left := gridColumns - stated
	if left < 0 || (unstated > 0 && left < unstated) {
		return widths, fmt.Errorf("the panels need more than Grafana's %d columns", gridColumns)
	}
	if unstated == 0 {
		return widths, nil
	}
	share, extra := left/unstated, left%unstated
	for index, panel := range panels {
		if panel.Width != nil {
			continue
		}
		widths[index] = share
		if extra > 0 {
			widths[index]++
			extra--
		}
	}
	return widths, nil
}

// gridHeight is a panel's height in grid rows, the nearest to its height in pixels.
func gridHeight(pixels int) int {
	return max(1, int(math.Round(float64(pixels+gridCellGap)/float64(gridCellHeight+gridCellGap))))
}
