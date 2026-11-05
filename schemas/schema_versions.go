// Package schemas holds the dashboard DSL's document schema, one file per minor version, and reads which of them a document's schema_version names.
package schemas

import (
	"embed"
	"fmt"
	"regexp"
	"slices"
	"strconv"
)

//go:embed dashboard-dsl-v*.schema.json
var schemaFiles embed.FS

var minorFileName = regexp.MustCompile(`^dashboard-dsl-v(\d+)\.(\d+)\.schema\.json$`)

// minorSchema is one minor version's schema file.
type minorSchema struct {
	major, minor int
	fileName     string
}

func (schema minorSchema) version() string { return fmt.Sprintf("%d.%d", schema.major, schema.minor) }

// minorSchemas are the schema files the provider carries, oldest first.
var minorSchemas = readMinorSchemas()

func readMinorSchemas() []minorSchema {
	entries, err := schemaFiles.ReadDir(".")
	if err != nil {
		panic(fmt.Sprintf("the embedded schemas cannot be listed: %v", err))
	}
	var read []minorSchema
	for _, entry := range entries {
		named := minorFileName.FindStringSubmatch(entry.Name())
		if named == nil {
			panic(fmt.Sprintf("%s is not named dashboard-dsl-v<major>.<minor>.schema.json", entry.Name()))
		}
		major, _ := strconv.Atoi(named[1])
		minor, _ := strconv.Atoi(named[2])
		read = append(read, minorSchema{major: major, minor: minor, fileName: entry.Name()})
	}
	slices.SortFunc(read, func(left, right minorSchema) int {
		if left.major != right.major {
			return left.major - right.major
		}
		return left.minor - right.minor
	})
	return read
}

// SchemaFor is the schema a schema_version names, and the minor version it is: a minor names its own file, and a major floats to its newest minor.
func SchemaFor(stated string) (text []byte, minorVersion string, err error) {
	for index := len(minorSchemas) - 1; index >= 0; index-- {
		schema := minorSchemas[index]
		if stated == schema.version() || stated == strconv.Itoa(schema.major) {
			text, err := schemaFiles.ReadFile(schema.fileName)
			return text, schema.version(), err
		}
	}
	return nil, "", fmt.Errorf("%q is not a version this provider reads; it reads %q", stated, StatedVersions())
}

// StatedVersions are the versions a document may state: each major, then each minor.
func StatedVersions() []string {
	var stated []string
	for _, schema := range minorSchemas {
		if major := strconv.Itoa(schema.major); !slices.Contains(stated, major) {
			stated = append(stated, major)
		}
	}
	for _, schema := range minorSchemas {
		stated = append(stated, schema.version())
	}
	return stated
}

// ReleaseAssets are the schema files a release attaches, by asset name: every minor's file, and each major's as its newest minor, for a modeline to name.
func ReleaseAssets() (map[string][]byte, error) {
	assets := map[string][]byte{}
	for _, schema := range minorSchemas {
		text, err := schemaFiles.ReadFile(schema.fileName)
		if err != nil {
			return nil, err
		}
		assets[schema.fileName] = text
		assets[fmt.Sprintf("dashboard-dsl-v%d.schema.json", schema.major)] = text
	}
	return assets, nil
}
