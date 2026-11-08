package dashboarddsl

import (
	"slices"
	"strings"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
)

// documentPath is where a value stands in a document: the keys and list indexes from the root down to it.
type documentPath []pathStep

type pathStep struct {
	entryKey  string
	itemIndex int
	isItem    bool
}

// field is the path of a map's value, by its key.
func (path documentPath) field(entryKey string) documentPath {
	return append(slices.Clip(path), pathStep{entryKey: entryKey})
}

// item is the path of a list's item, by its index.
func (path documentPath) item(itemIndex int) documentPath {
	return append(slices.Clip(path), pathStep{itemIndex: itemIndex, isItem: true})
}

func (path documentPath) nodePositions(documentRoot yamldslspec.Node) (yamldslspec.Position, yamldslspec.Position) {
	heldNode, keyPosition := documentRoot, documentRoot.Position
	for _, step := range path {
		childNode, childKeyPosition, isFound := step.childOf(heldNode)
		if !isFound {
			return heldNode.Position, heldNode.Position
		}
		heldNode, keyPosition = childNode, childKeyPosition
	}
	return heldNode.Position, keyPosition
}

func (step pathStep) childOf(holderNode yamldslspec.Node) (yamldslspec.Node, yamldslspec.Position, bool) {
	if step.isItem {
		if holderNode.Kind != yamldslspec.NodeList || step.itemIndex < 0 || step.itemIndex >= len(holderNode.Items) {
			return yamldslspec.Node{}, yamldslspec.Position{}, false
		}
		itemNode := holderNode.Items[step.itemIndex]
		return itemNode, itemNode.Position, true
	}
	for _, mapEntry := range holderNode.Entries {
		if mapEntry.Key == step.entryKey {
			return mapEntry.Value, mapEntry.KeyPosition, true
		}
	}
	return yamldslspec.Node{}, yamldslspec.Position{}, false
}

// oneOf is a set of values as a sentence names them, each fenced: "`a`, `b` or `c`".
func oneOf(values []string) string {
	fenced := make([]string, len(values))
	for index, value := range values {
		fenced[index] = yamldslspec.CodeSpan(value)
	}
	if len(fenced) == 1 {
		return fenced[0]
	}
	return strings.Join(fenced[:len(fenced)-1], ", ") + " or " + fenced[len(fenced)-1]
}
