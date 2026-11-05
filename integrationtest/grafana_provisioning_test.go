//go:build acceptance

// Package integrationtest loads rendered dashboards into a real Grafana, the release each carried target schema names, the way the monitoring UI loads them: from files, through its file provider.
package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/litebitus/terraform-provider-dashboard/dashboarddsl"
	"github.com/litebitus/terraform-provider-dashboard/grafanarenderer"
	"github.com/litebitus/terraform-provider-dashboard/internal/grafanamirror"
)

// mockFolder is the folder the dashboards are provisioned under, which Grafana's file provider makes a Grafana folder.
const mockFolder = "mock-owner"

// unnamedTitle is the title of the every-kind render provisioned without an id, which Grafana names itself.
const unnamedTitle = "Mock Every Kind Unnamed"

// runningGrafana is one Grafana release, running with a render of the every-kind spec provisioned into it.
type runningGrafana struct {
	target    grafanarenderer.TargetSchema
	container string
	baseURL   string
	rendered  dashboardResource
}

var grafanas []*runningGrafana

const (
	dashboardProvider = `apiVersion: 1
providers:
  - name: dashboard-dsl
    type: file
    allowUiUpdates: false
    options:
      path: /var/lib/grafana/dashboard-dsl
      foldersFromFilesStructure: true
`
	datasourceProvider = `apiVersion: 1
datasources:
  - name: Mock Thanos Query
    type: prometheus
    access: proxy
    url: http://127.0.0.1:9090
    isDefault: true
`
)

type dashboardResource struct {
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		Name        string            `json:"name"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Elements map[string]struct {
			Spec struct {
				VizConfig struct {
					Group string `json:"group"`
				} `json:"vizConfig"`
			} `json:"spec"`
		} `json:"elements"`
		Variables []struct {
			Kind string `json:"kind"`
		} `json:"variables"`
	} `json:"spec"`
}

func TestMain(m *testing.M) {
	code, err := runAgainstGrafana(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}

// runAgainstGrafana starts every carried target schema's Grafana with its render provisioned, runs the tests, and removes each Grafana whatever they answered.
func runAgainstGrafana(m *testing.M) (int, error) {
	carried, err := grafanarenderer.TargetSchemas()
	if err != nil {
		return 0, err
	}
	root, err := os.MkdirTemp("", "dashboard-dsl-grafana-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(root)
	for _, target := range carried {
		grafana := &runningGrafana{target: target}
		grafanas = append(grafanas, grafana)
		err := grafana.start(filepath.Join(root, target.ID.String()))
		if grafana.container != "" {
			defer exec.Command("docker", "rm", "--force", grafana.container).Run()
		}
		if err != nil {
			return 0, fmt.Errorf("%s: %w", target.ID, err)
		}
	}
	return m.Run(), nil
}

// mirrorPullBound is the longest the pull of the repository's copy of a Grafana image may take.
const mirrorPullBound = 5 * time.Minute

// grafanaImage is the repository's copy of the target schema's Grafana release, or Docker Hub's image when the copy cannot be pulled.
func grafanaImage(target grafanarenderer.TargetSchema) string {
	mirrored := grafanamirror.Image(target)
	ctx, cancel := context.WithTimeout(context.Background(), mirrorPullBound)
	defer cancel()
	pulled, err := exec.CommandContext(ctx, "docker", "pull", "--quiet", mirrored).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to pull %s, so the test runs %s from Docker Hub: %s\n",
			mirrored, target.GrafanaImage(), strings.TrimSpace(string(pulled)))
		return target.GrafanaImage()
	}
	return mirrored
}

func (grafana *runningGrafana) start(folder string) error {
	if err := grafana.provision(folder); err != nil {
		return err
	}
	image := grafanaImage(grafana.target)
	started, err := exec.Command("docker", "run", "--detach", "--rm",
		"--publish", "127.0.0.1::3000",
		"--volume", filepath.Join(folder, "provisioning")+":/etc/grafana/provisioning:ro",
		"--volume", filepath.Join(folder, "dashboards")+":/var/lib/grafana/dashboard-dsl:ro",
		image).Output()
	if err != nil {
		return fmt.Errorf("start %s: %w", image, err)
	}
	grafana.container = strings.TrimSpace(string(started))
	port, err := exec.Command("docker", "port", grafana.container, "3000/tcp").Output()
	if err != nil {
		return fmt.Errorf("read the published port: %w", err)
	}
	grafana.baseURL = "http://" + strings.TrimSpace(strings.Split(string(port), "\n")[0])
	if err := grafana.awaitDashboard(time.Minute); err != nil {
		logs, _ := exec.Command("docker", "logs", grafana.container).CombinedOutput()
		return fmt.Errorf("%w\n%s", err, logs)
	}
	return nil
}

func (grafana *runningGrafana) provision(folder string) error {
	spec, err := os.ReadFile(filepath.Join("..", "grafanarenderer", "testdata", "every_kind.yml"))
	if err != nil {
		return err
	}
	dashboard, err := dashboarddsl.ParseDashboard(string(spec))
	if err != nil {
		return err
	}
	rendered, err := grafanarenderer.RenderDashboard(dashboard, grafana.target.ID)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(rendered, &grafana.rendered); err != nil {
		return err
	}
	unnamed := dashboard
	unnamed.ID, unnamed.Title = "", unnamedTitle
	renderedUnnamed, err := grafanarenderer.RenderDashboard(unnamed, grafana.target.ID)
	if err != nil {
		return err
	}
	files := map[string][]byte{
		filepath.Join("provisioning", "dashboards", "dashboard-dsl.yml"):   []byte(dashboardProvider),
		filepath.Join("provisioning", "datasources", "datasources.yml"):    []byte(datasourceProvider),
		filepath.Join("dashboards", mockFolder, "every-kind.json"):         rendered,
		filepath.Join("dashboards", mockFolder, "every-kind-unnamed.json"): renderedUnnamed,
	}
	for name, content := range files {
		path := filepath.Join(folder, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// resourcePath is where Grafana's API answers the provisioned dashboard, in the API version its target schema states.
func (grafana *runningGrafana) resourcePath() string {
	return "/apis/" + grafana.target.APIVersion + "/namespaces/default/dashboards/" + grafana.rendered.Metadata.Name
}

// awaitDashboard waits, at most the bound, for Grafana to answer the provisioned dashboard.
func (grafana *runningGrafana) awaitDashboard(bound time.Duration) error {
	deadline := time.Now().Add(bound)
	for {
		status, _, err := grafana.get(grafana.resourcePath())
		if err == nil && status == http.StatusOK {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("grafana did not answer %s within %s: status %d, %v", grafana.resourcePath(), bound, status, err)
		}
		time.Sleep(time.Second)
	}
}

func (grafana *runningGrafana) get(path string) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, grafana.baseURL+path, nil)
	if err != nil {
		return 0, nil, err
	}
	request.SetBasicAuth("admin", "admin")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	return response.StatusCode, body, err
}

func (grafana *runningGrafana) read(t *testing.T, path string, into any) {
	t.Helper()
	status, body, err := grafana.get(path)
	if err != nil || status != http.StatusOK {
		t.Fatalf("GET %s: status %d, %v: %s", path, status, err, body)
	}
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatal(err)
	}
}

// The dashboard loads from its file into its folder, under the document's id, holding every panel as rendered.
func TestGrafanaLoadsEveryPanelKindAsRendered(t *testing.T) {
	for _, grafana := range grafanas {
		var stored dashboardResource
		grafana.read(t, grafana.resourcePath(), &stored)
		var folder struct {
			Title string `json:"title"`
		}
		grafana.read(t, "/api/folders/"+stored.Metadata.Annotations["grafana.app/folder"], &folder)
		if folder.Title != mockFolder {
			t.Errorf("%s: folder %q, want %q", grafana.target.ID, folder.Title, mockFolder)
		}
		if got, want := panelGroups(stored), panelGroups(grafana.rendered); !slices.Equal(got, want) {
			t.Errorf("%s: panels %v, want %v", grafana.target.ID, got, want)
		}
	}
}

// A render without an id loads too, under a name Grafana assigns.
func TestGrafanaNamesADashboardWithoutAnID(t *testing.T) {
	for _, grafana := range grafanas {
		var found []struct {
			UID   string `json:"uid"`
			Title string `json:"title"`
		}
		grafana.read(t, "/api/search?type=dash-db&query="+strings.ReplaceAll(unnamedTitle, " ", "%20"), &found)
		if len(found) != 1 || found[0].UID == "" {
			t.Errorf("%s: dashboards titled %q: %+v, want one, named by Grafana", grafana.target.ID, unnamedTitle, found)
		}
	}
}

// The variables load as rendered, each of the kind its choices render as.
func TestGrafanaLoadsEveryVariableAsRendered(t *testing.T) {
	for _, grafana := range grafanas {
		var stored dashboardResource
		grafana.read(t, grafana.resourcePath(), &stored)
		if got, want := variableKinds(stored), variableKinds(grafana.rendered); !slices.Equal(got, want) {
			t.Errorf("%s: variables %v, want %v", grafana.target.ID, got, want)
		}
	}
}

// panelGroups is each element's name and visualization, in name order.
func panelGroups(dashboard dashboardResource) []string {
	var named []string
	for name, element := range dashboard.Spec.Elements {
		named = append(named, name+":"+element.Spec.VizConfig.Group)
	}
	slices.Sort(named)
	return named
}

func variableKinds(dashboard dashboardResource) []string {
	var kinds []string
	for _, variable := range dashboard.Spec.Variables {
		kinds = append(kinds, variable.Kind)
	}
	return kinds
}
