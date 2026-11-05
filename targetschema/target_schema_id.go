// Package targetschema reads target schema identifiers, a UI's dashboard format and its version, and picks the one a render uses.
package targetschema

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ID is a target schema identifier, such as grafana-v2-13.3: its kind, grafana-v2, and its version, a major or major.minor.
type ID struct {
	Kind    string
	Version string
}

var (
	idPattern     = regexp.MustCompile(`^([a-z][a-z0-9]*(?:-[a-z][a-z0-9]*)*)-([0-9]+(?:\.[0-9]+)?)$`)
	prefixPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z][a-z0-9]*)*$`)
)

// ParseID reads a whole identifier: its kind's words, then its version.
func ParseID(text string) (ID, error) {
	parts := idPattern.FindStringSubmatch(text)
	if parts == nil {
		return ID{}, fmt.Errorf("%q is not a target schema identifier, its kind then its version, such as grafana-v2-13", text)
	}
	return ID{Kind: parts[1], Version: parts[2]}, nil
}

func (id ID) String() string { return id.Kind + "-" + id.Version }

func (id ID) major() string {
	major, _, _ := strings.Cut(id.Version, ".")
	return major
}

func (id ID) isMinor() bool { return strings.Contains(id.Version, ".") }

// Filter is a target schema as a call or a default states it: a whole identifier, or the leading words of a kind.
type Filter struct {
	words []string
	whole *ID
}

// ParseFilter reads a filter.
func ParseFilter(text string) (Filter, error) {
	if id, err := ParseID(text); err == nil {
		return Filter{words: strings.Split(id.Kind, "-"), whole: &id}, nil
	}
	if !prefixPattern.MatchString(text) {
		return Filter{}, fmt.Errorf("%q is neither a target schema identifier nor the leading words of one, such as grafana", text)
	}
	return Filter{words: strings.Split(text, "-")}, nil
}

func (filter Filter) String() string {
	if filter.whole != nil {
		return filter.whole.String()
	}
	return strings.Join(filter.words, "-")
}

// Whole is the identifier a filter names in full, when it does.
func (filter Filter) Whole() (ID, bool) {
	if filter.whole == nil {
		return ID{}, false
	}
	return *filter.whole, true
}

// Matches reports whether a filter selects an identifier: its words lead the identifier's kind, never part of a word, and a version it states is the identifier's or the identifier's major.
func (filter Filter) Matches(id ID) bool {
	kindWords := strings.Split(id.Kind, "-")
	if filter.whole != nil {
		if filter.whole.Kind != id.Kind {
			return false
		}
		return filter.whole.Version == id.Version || (!filter.whole.isMinor() && filter.whole.Version == id.major())
	}
	return len(filter.words) <= len(kindWords) && slices.Equal(filter.words, kindWords[:len(filter.words)])
}

// Resolve is the carried target schema an identifier names: a minor names itself, and a major its newest carried minor.
func Resolve(id ID, carried []ID) (ID, error) {
	var newest *ID
	for index, candidate := range carried {
		if candidate.Kind != id.Kind {
			continue
		}
		if candidate.Version == id.Version {
			return candidate, nil
		}
		if !id.isMinor() && candidate.major() == id.Version && (newest == nil || newerMinor(candidate, *newest)) {
			newest = &carried[index]
		}
	}
	if newest != nil {
		return *newest, nil
	}
	return ID{}, fmt.Errorf("target schema %s is not one this provider renders; it renders %s", id, listed(carried))
}

// newerMinor reports whether one minor of a major is newer than another of the same major.
func newerMinor(candidate, than ID) bool { return minorNumber(candidate) > minorNumber(than) }

func minorNumber(id ID) int {
	_, minor, _ := strings.Cut(id.Version, ".")
	number, _ := strconv.Atoi(minor)
	return number
}

func listed(ids []ID) string {
	names := make([]string, len(ids))
	for index, id := range ids {
		names[index] = id.String()
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}
