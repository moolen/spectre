// Package helpers provides Helm deployment utilities for e2e testing.
package helpers

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
)

// HelmDeployer manages Helm chart deployments.
type HelmDeployer struct {
	Config *action.Configuration
	t      *testing.T
}

// NewHelmDeployer creates a new Helm deployer.
func NewHelmDeployer(t *testing.T, kubeConfig, namespace string) (*HelmDeployer, error) {
	t.Logf("Creating Helm deployer for namespace %s", namespace)

	// Create Helm settings
	settings := cli.New()
	settings.KubeConfig = kubeConfig
	settings.KubeContext = ""
	settings.KubeAsUser = ""
	settings.KubeAPIServer = ""

	// Create action configuration
	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), debugLog); err != nil {
		return nil, fmt.Errorf("failed to initialize Helm config: %w", err)
	}

	t.Logf("✓ Helm deployer created")

	return &HelmDeployer{
		Config: cfg,
		t:      t,
	}, nil
}

// InstallChart installs a Helm chart.
func (hd *HelmDeployer) InstallChart(releaseName, chartPath string, values map[string]interface{}) error {
	hd.t.Logf("Installing Helm chart %s as %s", chartPath, releaseName)

	// Load chart
	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	// Create install action
	install := action.NewInstall(hd.Config)
	install.ReleaseName = releaseName

	// Install release
	release, err := install.Run(chart, values)
	if err != nil {
		return fmt.Errorf("failed to install chart: %w", err)
	}

	hd.t.Logf("✓ Chart installed: %s (revision %d)", release.Name, release.Version)
	return nil
}

// UpgradeChart upgrades a Helm release.
func (hd *HelmDeployer) UpgradeChart(releaseName, chartPath string, values map[string]interface{}) error {
	hd.t.Logf("Upgrading Helm release %s from %s", releaseName, chartPath)

	// Load chart
	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	// Create upgrade action
	upgrade := action.NewUpgrade(hd.Config)

	// Upgrade release
	release, err := upgrade.Run(releaseName, chart, values)
	if err != nil {
		return fmt.Errorf("failed to upgrade chart: %w", err)
	}

	hd.t.Logf("✓ Chart upgraded: %s (revision %d)", release.Name, release.Version)
	return nil
}

// UninstallChart uninstalls a Helm release.
func (hd *HelmDeployer) UninstallChart(releaseName string) error {
	hd.t.Logf("Uninstalling Helm release %s", releaseName)

	uninstall := action.NewUninstall(hd.Config)

	_, err := uninstall.Run(releaseName)
	if err != nil {
		return fmt.Errorf("failed to uninstall chart: %w", err)
	}

	hd.t.Logf("✓ Chart uninstalled: %s", releaseName)
	return nil
}

// GetRelease gets information about a release.
func (hd *HelmDeployer) GetRelease(releaseName string) error {
	get := action.NewGet(hd.Config)

	release, err := get.Run(releaseName)
	if err != nil {
		return fmt.Errorf("release not found: %w", err)
	}

	hd.t.Logf("Release %s (revision %d): %s", release.Name, release.Version, release.Info.Status.String())
	return nil
}

// ListReleases lists all releases in the namespace.
func (hd *HelmDeployer) ListReleases() ([]string, error) {
	list := action.NewList(hd.Config)

	releases, err := list.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}

	var names []string
	for _, rel := range releases {
		names = append(names, rel.Name)
	}

	return names, nil
}

// debugLog is a Helm-compatible logger.
func debugLog(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", v...)
}

// ManifestHelper provides utilities for working with Helm manifests.
type ManifestHelper struct {
	t *testing.T
}

// NewManifestHelper creates a new manifest helper.
func NewManifestHelper(t *testing.T) *ManifestHelper {
	return &ManifestHelper{t: t}
}

// ParseYAML parses a YAML manifest string into individual resources.
func (mh *ManifestHelper) ParseYAML(manifest string) ([]map[string]interface{}, error) {
	var resources []map[string]interface{}

	// Split YAML documents
	documents := bytes.Split([]byte(manifest), []byte("---\n"))

	for _, doc := range documents {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		// TODO: Parse each YAML document into a structured format
		// For now, this is a placeholder for the real implementation
	}

	return resources, nil
}
