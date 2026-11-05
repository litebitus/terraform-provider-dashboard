package targetschema

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Request is what decides the target schema a render uses, highest priority first: the call's target_schema, the document's target_schemas, then the defaults.
type Request struct {
	// TargetSchema is the call's target_schema, a whole identifier or a kind's leading words; empty when the call names none.
	TargetSchema string
	// Declared are the document's target_schemas.
	Declared []string
	// Defaults are the provider's default_target_schemas, a filter to the whole identifier it picks; nil where a caller has none.
	Defaults map[string]string
}

// Select is the one target schema a request leaves: the document's candidates narrowed by the call's filter, then by the defaults when several remain.
func Select(request Request) (ID, error) {
	var filter *Filter
	if request.TargetSchema != "" {
		parsed, err := ParseFilter(request.TargetSchema)
		if err != nil {
			return ID{}, fmt.Errorf("target_schema: %w", err)
		}
		filter = &parsed
	}
	defaults, err := parseDefaults(request.Defaults)
	if err != nil {
		return ID{}, err
	}
	if len(request.Declared) == 0 {
		return selectUndeclared(filter, defaults)
	}
	declared := make([]ID, 0, len(request.Declared))
	for _, text := range request.Declared {
		id, err := ParseID(text)
		if err != nil {
			return ID{}, fmt.Errorf("target_schemas: %w", err)
		}
		declared = append(declared, id)
	}
	candidates := declared
	if filter != nil {
		candidates = matching(declared, *filter)
		if len(candidates) == 0 {
			return ID{}, fmt.Errorf("target_schema %s matches none of the document's target schemas, %s", filter, listed(declared))
		}
	}
	if len(candidates) > 1 {
		candidates = narrowedByDefaults(candidates, defaults)
	}
	if len(candidates) > 1 {
		return ID{}, fmt.Errorf("the document's target schemas leave %s; name one with target_schema", listed(candidates))
	}
	return candidates[0], nil
}

// selectUndeclared is the target schema of a document that declares none: the call's whole identifier, else the one default its filter leaves.
func selectUndeclared(filter *Filter, defaults []defaultTarget) (ID, error) {
	if filter != nil {
		if whole, named := filter.Whole(); named {
			return whole, nil
		}
	}
	var chosen []ID
	for _, target := range defaults {
		if filter == nil || filter.Matches(target.picks) {
			chosen = append(chosen, target.picks)
		}
	}
	switch {
	case len(chosen) == 1:
		return chosen[0], nil
	case filter != nil:
		return ID{}, fmt.Errorf("the document declares no target_schemas, and target_schema %s is not a whole identifier and leaves no single default", filter)
	}
	return ID{}, errors.New("the document declares no target_schemas; name a whole identifier with target_schema")
}

// CheckDefaults holds defaults to their form: each key a filter, each value a whole identifier the key matches.
func CheckDefaults(defaults map[string]string) error {
	_, err := parseDefaults(defaults)
	return err
}

// defaultTarget is one default: the filter it applies to and the identifier it picks.
type defaultTarget struct {
	appliesTo Filter
	picks     ID
}

func parseDefaults(defaults map[string]string) ([]defaultTarget, error) {
	var parsed []defaultTarget
	for _, key := range slices.Sorted(maps.Keys(defaults)) {
		appliesTo, err := ParseFilter(key)
		if err != nil {
			return nil, fmt.Errorf("default_target_schemas: %w", err)
		}
		picks, err := ParseID(defaults[key])
		if err != nil {
			return nil, fmt.Errorf("default_target_schemas[%s]: %w", key, err)
		}
		if !appliesTo.Matches(picks) {
			return nil, fmt.Errorf("default_target_schemas[%s] picks %s, which %s does not match", key, picks, key)
		}
		parsed = append(parsed, defaultTarget{appliesTo: appliesTo, picks: picks})
	}
	return parsed, nil
}

// narrowedByDefaults keeps, among the candidates a default applies to, only the one it picks; a candidate no default applies to stays.
func narrowedByDefaults(candidates []ID, defaults []defaultTarget) []ID {
	var kept []ID
	for _, candidate := range candidates {
		applying := mostSpecificDefault(candidate, defaults)
		if applying == nil || wholeFilter(applying.picks).Matches(candidate) {
			kept = append(kept, candidate)
		}
	}
	return kept
}

// wholeFilter is the filter naming an identifier in full.
func wholeFilter(id ID) Filter { return Filter{words: strings.Split(id.Kind, "-"), whole: &id} }

// mostSpecificDefault is the default with the most words that applies to a candidate.
func mostSpecificDefault(candidate ID, defaults []defaultTarget) *defaultTarget {
	var chosen *defaultTarget
	for index := range defaults {
		target := &defaults[index]
		if target.appliesTo.Matches(candidate) && (chosen == nil || len(target.appliesTo.words) > len(chosen.appliesTo.words)) {
			chosen = target
		}
	}
	return chosen
}

func matching(ids []ID, filter Filter) []ID {
	var matched []ID
	for _, id := range ids {
		if filter.Matches(id) {
			matched = append(matched, id)
		}
	}
	return matched
}
