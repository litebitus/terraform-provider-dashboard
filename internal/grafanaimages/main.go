// Command grafanaimages prints, one pair per line, the Docker Hub image of every carried target schema's Grafana release and the repository's copy of it, for CI to mirror.
package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/litebitus/terraform-provider-dashboard/grafanarenderer"
	"github.com/litebitus/terraform-provider-dashboard/internal/grafanamirror"
)

func main() {
	carried, err := grafanarenderer.TargetSchemas()
	if err != nil {
		fmt.Fprintln(os.Stderr, "grafanaimages:", err)
		os.Exit(1)
	}
	var pairs []string
	for _, target := range carried {
		pairs = append(pairs, "docker.io/"+target.GrafanaImage()+" "+grafanamirror.Image(target))
	}
	slices.Sort(pairs)
	for _, pair := range slices.Compact(pairs) {
		fmt.Println(pair)
	}
}
