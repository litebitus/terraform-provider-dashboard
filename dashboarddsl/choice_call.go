package dashboarddsl

import (
	"fmt"

	"github.com/litebitus/dql-compiler/dqlsyntax"
	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
)

// The choice functions a variable's choices call.
const (
	FieldValuesChoices = "field_values"
	ListChoices        = "list"
	FreeTextChoices    = "free_text"
	RangeChoices       = "range"
)

// ChoiceCall is a variable's choices read as a call to one of the choice functions.
type ChoiceCall struct {
	Function string
	Field    string
	Query    string
	Values   []string
}

// ChoiceCall reads the variable's choices.
func (variable Variable) ChoiceCall() (ChoiceCall, error) {
	call, err := dqlsyntax.ParseCall(variable.Choices, QueryGrammarVersion())
	if err != nil {
		return ChoiceCall{}, err
	}
	read := ChoiceCall{Function: call.Function}
	arguments := call.Arguments
	switch call.Function {
	case FieldValuesChoices:
		if len(arguments) < 1 || len(arguments) > 2 || arguments[0].Kind != dqlsyntax.FieldCallArgument ||
			(len(arguments) == 2 && arguments[1].Kind != dqlsyntax.QuotedCallArgument) {
			return ChoiceCall{}, fmt.Errorf("%s takes a field and, optionally, a quoted query",
				yamldslspec.CodeSpan(call.Function))
		}
		read.Field = arguments[0].Text
		if len(arguments) == 2 {
			read.Query = arguments[1].Text
		}
	case ListChoices:
		if len(arguments) == 0 {
			return ChoiceCall{}, fmt.Errorf("%s takes the values, quoted", yamldslspec.CodeSpan(call.Function))
		}
		for _, argument := range arguments {
			if argument.Kind != dqlsyntax.QuotedCallArgument {
				return ChoiceCall{}, fmt.Errorf("%s takes the values, quoted", yamldslspec.CodeSpan(call.Function))
			}
			read.Values = append(read.Values, argument.Text)
		}
	case FreeTextChoices:
		if len(arguments) != 0 {
			return ChoiceCall{}, fmt.Errorf("%s takes no arguments", yamldslspec.CodeSpan(call.Function))
		}
	case RangeChoices:
		if len(arguments) != 1 || arguments[0].Kind != dqlsyntax.FieldCallArgument {
			return ChoiceCall{}, fmt.Errorf("%s takes a field", yamldslspec.CodeSpan(call.Function))
		}
		read.Field = arguments[0].Text
	default:
		return ChoiceCall{}, fmt.Errorf("unknown choice function %s; choices call %s",
			yamldslspec.CodeSpan(call.Function), oneOf(choiceFunctions))
	}
	return read, nil
}

var choiceFunctions = []string{FieldValuesChoices, FreeTextChoices, ListChoices, RangeChoices}

// QueryGrammarVersion is the DQL grammar version a document's queries and calls are read in: the latest the compiler supports.
func QueryGrammarVersion() string {
	supported := dqlsyntax.SupportedGrammarVersions()
	return supported[len(supported)-1]
}
