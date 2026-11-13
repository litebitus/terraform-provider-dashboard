package dashboarddsl

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
)

// InputValues are the values a render is passed, by input name: a string, a
// json.Number, a bool, a []any or a map[string]any, nested as deep as a value is.
type InputValues map[string]any

// InputsProblem is a problem with the values a render is passed, rather than
// with the document.
type InputsProblem struct{ problemMessage string }

func (inputsProblem InputsProblem) Error() string { return inputsProblem.problemMessage }

const inputsField = "inputs"

// The kinds of value an input takes, as its `type` names them.
const (
	inputKindString  = "string"
	inputKindNumber  = "number"
	inputKindBoolean = "boolean"
	inputKindList    = "list"
	inputKindMap     = "map"
	nullValueKind    = "null"
)

// inputPlaceholderPattern is a placeholder reading an input, `${inputs.<name>}`
// with a key path into a map input after the name, and the escape prefix `$`
// written before it.
var inputPlaceholderPattern = regexp.MustCompile(`(\$?)\$\{inputs\.([^}\n]*)\}`)

var inputNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// declaredInput is an input the document declares, with the value a render reads
// for it.
type declaredInput struct {
	declaredKind string
	keyPosition  yamldslspec.Position
	readValue    *yamldslspec.Node
}

// inputSubstitution is one document's inputs read against the values passed.
type inputSubstitution struct {
	declaredInputs map[string]declaredInput
	problems       []yamldslspec.Diagnostic
	textChanged    bool
}

// substituteInputs is the document with every input it reads replaced by its
// value: a placeholder that is a whole value takes the value as it is, and one
// inside text takes a scalar. The `inputs` block itself stays as written.
func substituteInputs(documentRoot yamldslspec.Node, passedValues InputValues) (
	yamldslspec.Node, bool, []yamldslspec.Diagnostic, error,
) {
	documentInputs := &inputSubstitution{declaredInputs: map[string]declaredInput{}}
	inputsBlock, _ := documentRoot.Get(inputsField)
	if err := documentInputs.declare(inputsBlock, passedValues); err != nil {
		return documentRoot, false, nil, err
	}
	if documentRoot.Kind != yamldslspec.NodeMap {
		return documentRoot, false, documentInputs.problems, nil
	}
	substitutedRoot := documentRoot
	substitutedRoot.Entries = make([]yamldslspec.NodeEntry, len(documentRoot.Entries))
	for entryIndex, rootEntry := range documentRoot.Entries {
		if rootEntry.Key != inputsField {
			rootEntry.Value = documentInputs.substituted(rootEntry.Value)
		}
		substitutedRoot.Entries[entryIndex] = rootEntry
	}
	return substitutedRoot, documentInputs.textChanged, documentInputs.problems, nil
}

// declare reads each declared input and the value a render reads for it: the
// one passed, else its default. A passed value the document does not declare,
// or of another kind than declared, is a problem with what was passed.
func (documentInputs *inputSubstitution) declare(inputsBlock yamldslspec.Node, passedValues InputValues) error {
	for _, inputEntry := range inputsBlock.Entries {
		kindNode, _ := inputEntry.Value.Get("type")
		declaredKind, _ := kindNode.AsString()
		declaration := declaredInput{declaredKind: declaredKind, keyPosition: inputEntry.KeyPosition}
		if defaultNode, hasDefault := inputEntry.Value.Get("default"); hasDefault {
			if writtenKind := nodeKind(defaultNode); declaredKind != "" && writtenKind != declaredKind {
				documentInputs.problems = append(documentInputs.problems, yamldslspec.DiagnosticAt(
					defaultNode.Position,
					fmt.Sprintf("input `%s` is declared `%s`, and its `default` is a %s",
						inputEntry.Key, declaredKind, writtenKind)))
			}
			declaration.readValue = &defaultNode
		}
		documentInputs.declaredInputs[inputEntry.Key] = declaration
	}
	for _, inputName := range slices.Sorted(maps.Keys(passedValues)) {
		passedValue := passedValues[inputName]
		declaration, isDeclared := documentInputs.declaredInputs[inputName]
		if !isDeclared {
			return InputsProblem{fmt.Sprintf("the document declares no input `%s`", inputName)}
		}
		if passedValue == nil {
			continue
		}
		passedNode, err := valueNode(passedValue)
		if err != nil {
			return InputsProblem{fmt.Sprintf("input `%s`: %v", inputName, err)}
		}
		if passedKind := nodeKind(passedNode); declaration.declaredKind != "" && passedKind != declaration.declaredKind {
			return InputsProblem{fmt.Sprintf("input `%s` is declared `%s`, and is passed a %s",
				inputName, declaration.declaredKind, passedKind)}
		}
		declaration.readValue = &passedNode
		documentInputs.declaredInputs[inputName] = declaration
	}
	for _, inputName := range slices.Sorted(maps.Keys(documentInputs.declaredInputs)) {
		if declaration := documentInputs.declaredInputs[inputName]; declaration.readValue == nil {
			documentInputs.problems = append(documentInputs.problems, yamldslspec.DiagnosticAt(
				declaration.keyPosition,
				fmt.Sprintf("input `%s` is required, and the render is passed none", inputName)))
		}
	}
	return nil
}

// substituted is a value with its inputs substituted, its keys as written.
func (documentInputs *inputSubstitution) substituted(writtenNode yamldslspec.Node) yamldslspec.Node {
	switch writtenNode.Kind {
	case yamldslspec.NodeString:
		return documentInputs.substitutedText(writtenNode)
	case yamldslspec.NodeList:
		substitutedNode := writtenNode
		substitutedNode.Items = make([]yamldslspec.Node, len(writtenNode.Items))
		for itemIndex, listItem := range writtenNode.Items {
			substitutedNode.Items[itemIndex] = documentInputs.substituted(listItem)
		}
		return substitutedNode
	case yamldslspec.NodeMap:
		substitutedNode := writtenNode
		substitutedNode.Entries = make([]yamldslspec.NodeEntry, len(writtenNode.Entries))
		for entryIndex, mapEntry := range writtenNode.Entries {
			mapEntry.Value = documentInputs.substituted(mapEntry.Value)
			substitutedNode.Entries[entryIndex] = mapEntry
		}
		return substitutedNode
	}
	return writtenNode
}

// substitutedText is a string with its input placeholders substituted: one that
// is the whole string becomes the input's value, and one inside text its scalar
// text; an escaped one is written as text without its escape prefix.
func (documentInputs *inputSubstitution) substitutedText(writtenNode yamldslspec.Node) yamldslspec.Node {
	placeholderMatches := inputPlaceholderPattern.FindAllStringSubmatchIndex(writtenNode.Text, -1)
	if len(placeholderMatches) == 0 {
		return writtenNode
	}
	documentInputs.textChanged = true
	firstMatch := placeholderMatches[0]
	isWholeValue := len(placeholderMatches) == 1 && firstMatch[0] == 0 &&
		firstMatch[1] == len(writtenNode.Text) && firstMatch[2] == firstMatch[3]
	if isWholeValue {
		readValue, isRead := documentInputs.read(writtenNode, writtenNode.Text[firstMatch[4]:firstMatch[5]])
		if !isRead {
			return writtenNode
		}
		return positioned(readValue, writtenNode.Position)
	}
	var substitutedText strings.Builder
	copiedUpTo := 0
	for _, placeholderMatch := range placeholderMatches {
		substitutedText.WriteString(writtenNode.Text[copiedUpTo:placeholderMatch[0]])
		copiedUpTo = placeholderMatch[1]
		placeholderText := writtenNode.Text[placeholderMatch[0]:placeholderMatch[1]]
		if placeholderMatch[2] != placeholderMatch[3] {
			substitutedText.WriteString(placeholderText[1:])
			continue
		}
		placeholderBody := writtenNode.Text[placeholderMatch[4]:placeholderMatch[5]]
		readValue, isRead := documentInputs.read(writtenNode, placeholderBody)
		if !isRead {
			substitutedText.WriteString(placeholderText)
			continue
		}
		scalarText, isScalar := readValue.ScalarText()
		if !isScalar || readValue.Kind == yamldslspec.NodeNull {
			documentInputs.problems = append(documentInputs.problems, yamldslspec.DiagnosticAt(
				writtenNode.Position,
				fmt.Sprintf("`%s` is a %s inside text, which takes a scalar", placeholderText, nodeKind(readValue))))
			substitutedText.WriteString(placeholderText)
			continue
		}
		substitutedText.WriteString(scalarText)
	}
	substitutedText.WriteString(writtenNode.Text[copiedUpTo:])
	substitutedNode := writtenNode
	substitutedNode.Text = substitutedText.String()
	return substitutedNode
}

// read is the value a placeholder's body names: an input, and a key path into it.
func (documentInputs *inputSubstitution) read(writtenNode yamldslspec.Node, placeholderBody string) (
	yamldslspec.Node, bool,
) {
	placeholderText := "`${inputs." + placeholderBody + "}`"
	bodySteps := strings.Split(placeholderBody, ".")
	inputName := bodySteps[0]
	if !inputNamePattern.MatchString(inputName) {
		documentInputs.problems = append(documentInputs.problems, yamldslspec.DiagnosticAt(writtenNode.Position,
			placeholderText+" names no input: an input's name is letters, digits and underscores"))
		return yamldslspec.Node{}, false
	}
	declaration, isDeclared := documentInputs.declaredInputs[inputName]
	if !isDeclared {
		documentInputs.problems = append(documentInputs.problems, yamldslspec.DiagnosticAt(writtenNode.Position,
			fmt.Sprintf("%s reads input `%s`, which `inputs` does not declare", placeholderText, inputName)))
		return yamldslspec.Node{}, false
	}
	if declaration.readValue == nil {
		return yamldslspec.Node{}, false
	}
	readValue := *declaration.readValue
	for _, keyStep := range bodySteps[1:] {
		if readValue.Kind != yamldslspec.NodeMap {
			documentInputs.problems = append(documentInputs.problems, yamldslspec.DiagnosticAt(writtenNode.Position,
				fmt.Sprintf("%s reads key `%s` of a %s, which holds no keys",
					placeholderText, keyStep, nodeKind(readValue))))
			return yamldslspec.Node{}, false
		}
		heldValue, holdsKey := readValue.Get(keyStep)
		if !holdsKey {
			documentInputs.problems = append(documentInputs.problems, yamldslspec.DiagnosticAt(writtenNode.Position,
				fmt.Sprintf("%s reads key `%s`, which the value does not hold", placeholderText, keyStep)))
			return yamldslspec.Node{}, false
		}
		readValue = heldValue
	}
	return readValue, true
}

// nodeKind is the kind of value a node holds, as an input's `type` names it.
func nodeKind(valueNode yamldslspec.Node) string {
	switch valueNode.Kind {
	case yamldslspec.NodeString:
		return inputKindString
	case yamldslspec.NodeNumber:
		return inputKindNumber
	case yamldslspec.NodeBool:
		return inputKindBoolean
	case yamldslspec.NodeList:
		return inputKindList
	case yamldslspec.NodeMap:
		return inputKindMap
	}
	return nullValueKind
}

// valueNode is a passed value as a node.
func valueNode(passedValue any) (yamldslspec.Node, error) {
	switch typedValue := passedValue.(type) {
	case string:
		return yamldslspec.Node{Kind: yamldslspec.NodeString, Text: typedValue}, nil
	case json.Number:
		return yamldslspec.Node{Kind: yamldslspec.NodeNumber, Text: typedValue.String()}, nil
	case bool:
		return yamldslspec.Node{Kind: yamldslspec.NodeBool, Flag: typedValue}, nil
	case []any:
		listNode := yamldslspec.Node{Kind: yamldslspec.NodeList}
		for _, listItem := range typedValue {
			itemNode, err := valueNode(listItem)
			if err != nil {
				return yamldslspec.Node{}, err
			}
			listNode.Items = append(listNode.Items, itemNode)
		}
		return listNode, nil
	case map[string]any:
		mapNode := yamldslspec.Node{Kind: yamldslspec.NodeMap}
		for _, mapKey := range slices.Sorted(maps.Keys(typedValue)) {
			keyValueNode, err := valueNode(typedValue[mapKey])
			if err != nil {
				return yamldslspec.Node{}, err
			}
			mapNode.Entries = append(mapNode.Entries, yamldslspec.NodeEntry{Key: mapKey, Value: keyValueNode})
		}
		return mapNode, nil
	case nil:
		return yamldslspec.Node{Kind: yamldslspec.NodeNull}, nil
	}
	return yamldslspec.Node{}, fmt.Errorf("a %T is not a value an input takes", passedValue)
}

// positioned is a copy of a value placed where the placeholder that read it was
// written, so a problem with it is reported there.
func positioned(readValue yamldslspec.Node, placeholderPosition yamldslspec.Position) yamldslspec.Node {
	placedValue := readValue
	placedValue.Position = placeholderPosition
	placedValue.Items = make([]yamldslspec.Node, len(readValue.Items))
	for itemIndex, listItem := range readValue.Items {
		placedValue.Items[itemIndex] = positioned(listItem, placeholderPosition)
	}
	placedValue.Entries = make([]yamldslspec.NodeEntry, len(readValue.Entries))
	for entryIndex, mapEntry := range readValue.Entries {
		mapEntry.KeyPosition = placeholderPosition
		mapEntry.Value = positioned(mapEntry.Value, placeholderPosition)
		placedValue.Entries[entryIndex] = mapEntry
	}
	return placedValue
}
