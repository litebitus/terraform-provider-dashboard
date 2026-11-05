// Command targetschemagen writes each Grafana target schema's JSON Schema into the folder it is given, converted by the cue command it is given from Grafana's CUE at the target's release.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/litebitus/terraform-provider-dashboard/grafanarenderer"
)

const grafanaSourceURL = "https://raw.githubusercontent.com/grafana/grafana/%s/%s"

var sourceClient = &http.Client{Timeout: 30 * time.Second}

func main() {
	if err := writeTargetSchemas(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "targetschemagen:", err)
		os.Exit(1)
	}
}

func writeTargetSchemas(arguments []string) error {
	if len(arguments) < 3 || arguments[1] != "--" {
		return fmt.Errorf("usage: targetschemagen <folder> -- <cue command>")
	}
	folder, cueCommand := arguments[0], arguments[2:]
	carried, err := grafanarenderer.TargetSchemas()
	if err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", "targetschemagen-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	for _, target := range carried {
		source, err := fetchSource(target, workspace)
		if err != nil {
			return fmt.Errorf("%s: %w", target.ID, err)
		}
		convert := append(append([]string{}, cueCommand[1:]...), "def", "--out", "jsonschema", "-e", target.CUEExpression, source)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		converted, err := exec.CommandContext(ctx, cueCommand[0], convert...).Output()
		cancel()
		if err != nil {
			return fmt.Errorf("%s: converting %s: %w", target.ID, target.CUEFile, err)
		}
		if err := os.WriteFile(filepath.Join(folder, target.SchemaFileName()), converted, 0o644); err != nil {
			return err
		}
		fmt.Printf("%s: %s at %s\n", target.SchemaFileName(), target.CUEFile, target.GrafanaRelease)
	}
	return nil
}

// fetchSource writes the target's CUE file, as Grafana released it, into the workspace.
func fetchSource(target grafanarenderer.TargetSchema, workspace string) (string, error) {
	url := fmt.Sprintf(grafanaSourceURL, target.GrafanaRelease, target.CUEFile)
	response, err := sourceClient.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, response.Status)
	}
	text, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	path := filepath.Join(workspace, target.ID.String()+".cue")
	return path, os.WriteFile(path, text, 0o644)
}
