package schemas

import (
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"testing"
)

func TestAMajorFloatsToItsNewestMinorAndAMinorNamesItself(t *testing.T) {
	newest := minorSchemas[len(minorSchemas)-1]
	for stated, want := range map[string]string{
		strconv.Itoa(newest.major): newest.version(),
		"1.0":                      "1.0",
	} {
		if _, got, err := SchemaFor(stated); err != nil || got != want {
			t.Errorf("%s reads as %s (%v), want %s", stated, got, err, want)
		}
	}
	if _, _, err := SchemaFor("0.9"); err == nil {
		t.Error("an unknown version reads")
	}
}

// Every minor's file states its own version and its major's as what a document may write, and is identified by its own name.
func TestEveryMinorSchemaIsIdentifiedByItsVersion(t *testing.T) {
	for _, schema := range minorSchemas {
		text, err := schemaFiles.ReadFile(schema.fileName)
		if err != nil {
			t.Fatal(err)
		}
		var read struct {
			ID         string `json:"$id"`
			Properties struct {
				SchemaVersion struct {
					Enum []string `json:"enum"`
				} `json:"schema_version"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(text, &read); err != nil {
			t.Fatalf("%s: %v", schema.fileName, err)
		}
		want := []string{strconv.Itoa(schema.major), schema.version()}
		if read.ID != "https://github.com/litebitus/terraform-provider-dashboard/schemas/"+schema.fileName ||
			!slices.Equal(read.Properties.SchemaVersion.Enum, want) {
			t.Errorf("%s: $id %s, schema_version %v, want %v", schema.fileName, read.ID, read.Properties.SchemaVersion.Enum, want)
		}
	}
}

func TestAReleaseAttachesEveryMinorAndEachMajorAsItsNewestMinor(t *testing.T) {
	assets, err := ReleaseAssets()
	if err != nil {
		t.Fatal(err)
	}
	for _, schema := range minorSchemas {
		if _, attached := assets[schema.fileName]; !attached {
			t.Errorf("%s is not attached; assets %v", schema.fileName, slices.Sorted(maps.Keys(assets)))
		}
	}
	newest := minorSchemas[len(minorSchemas)-1]
	major := "dashboard-dsl-v" + strconv.Itoa(newest.major) + ".schema.json"
	if string(assets[major]) != string(assets[newest.fileName]) {
		t.Errorf("%s is not %s", major, newest.fileName)
	}
}
