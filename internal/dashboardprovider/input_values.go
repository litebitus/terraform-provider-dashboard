package dashboardprovider

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
)

// inputValues are the values a render is passed, read from the map or object a
// configuration states them in; a null leaves every input to its default.
func inputValues(statedInputs basetypes.DynamicValue) (dashboarddsl.InputValues, error) {
	if statedInputs.IsNull() || statedInputs.IsUnderlyingValueNull() {
		return nil, nil
	}
	plainValue, err := plainValueOf(statedInputs.UnderlyingValue())
	if err != nil {
		return nil, err
	}
	namedValues, isMapping := plainValue.(map[string]any)
	if !isMapping {
		return nil, fmt.Errorf("the inputs are a map of input name to value")
	}
	return namedValues, nil
}

// plainValueOf is a Terraform value as the dashboard DSL reads a passed value.
func plainValueOf(statedValue attr.Value) (any, error) {
	if statedValue == nil || statedValue.IsNull() {
		return nil, nil
	}
	if statedValue.IsUnknown() {
		return nil, fmt.Errorf("an input's value is not known yet")
	}
	switch typedValue := statedValue.(type) {
	case basetypes.DynamicValue:
		return plainValueOf(typedValue.UnderlyingValue())
	case basetypes.StringValue:
		return typedValue.ValueString(), nil
	case basetypes.BoolValue:
		return typedValue.ValueBool(), nil
	case basetypes.NumberValue:
		return json.Number(typedValue.ValueBigFloat().Text('g', -1)), nil
	case basetypes.ListValue:
		return plainItems(typedValue.Elements())
	case basetypes.TupleValue:
		return plainItems(typedValue.Elements())
	case basetypes.SetValue:
		return plainItems(typedValue.Elements())
	case basetypes.MapValue:
		return plainEntries(typedValue.Elements())
	case basetypes.ObjectValue:
		return plainEntries(typedValue.Attributes())
	}
	return nil, fmt.Errorf("a %s is not a value an input takes", statedValue.Type(nil))
}

func plainItems(statedItems []attr.Value) (any, error) {
	plainList := make([]any, 0, len(statedItems))
	for _, statedItem := range statedItems {
		plainItem, err := plainValueOf(statedItem)
		if err != nil {
			return nil, err
		}
		plainList = append(plainList, plainItem)
	}
	return plainList, nil
}

func plainEntries(statedEntries map[string]attr.Value) (any, error) {
	plainMap := make(map[string]any, len(statedEntries))
	for entryKey, statedEntry := range statedEntries {
		plainEntry, err := plainValueOf(statedEntry)
		if err != nil {
			return nil, err
		}
		plainMap[entryKey] = plainEntry
	}
	return plainMap, nil
}
