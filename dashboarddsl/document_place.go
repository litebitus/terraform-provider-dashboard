package dashboarddsl

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// place is where something stands in a document, as a reader counts: "row 1, panel 2, query 1".
type place []string

var listItemNames = map[string]string{
	"annotations":    "annotation",
	"links":          "link",
	"panels":         "panel",
	"queries":        "query",
	"rows":           "row",
	"tags":           "tag",
	"target_schemas": "target schema",
	"thresholds":     "threshold",
}

var mapEntryNames = map[string]string{
	"value_mappings": "value mapping",
	"variables":      "variable",
}

func (at place) with(segment string) place { return append(slices.Clone(at), segment) }

// item is the place of a list's item, counted from 1.
func (at place) item(listField string, position int) place {
	return at.with(fmt.Sprintf("%s %d", listItemNames[listField], position+1))
}

// entry is the place of a map's entry, by its key.
func (at place) entry(mapField, key string) place {
	return at.with(fmt.Sprintf("%s %s", mapEntryNames[mapField], key))
}

func (at place) problem(format string, arguments ...any) error {
	message := fmt.Sprintf(format, arguments...)
	if len(at) == 0 {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %s", strings.Join(at, ", "), message)
}

// oneOf is a set of values as a sentence names them: `"a", "b" or "c"`.
func oneOf(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = fmt.Sprintf("%q", value)
	}
	if len(quoted) == 1 {
		return quoted[0]
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " or " + quoted[len(quoted)-1]
}
