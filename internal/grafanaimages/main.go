// Command grafanaimages prints the Docker Hub image of every carried target schema's Grafana release, one per line, for CI to mirror.
package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/litebitus/terraform-provider-dashboard/grafanarenderer"
)

func main() {
	carried, err := grafanarenderer.TargetSchemas()
	if err != nil {
		fmt.Fprintln(os.Stderr, "grafanaimages:", err)
		os.Exit(1)
	}
	var images []string
	for _, target := range carried {
		images = append(images, target.GrafanaImage())
	}
	slices.Sort(images)
	for _, image := range slices.Compact(images) {
		fmt.Println(image)
	}
}
