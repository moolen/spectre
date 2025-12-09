// Package helpers provides Helm deployment utilities for e2e testing.
package helpers

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
)

// HelmDeployer manages Helm chart deployments.
type HelmDeployer struct {
	Config    *action.Configuration
	Namespace string
	t         *testing.T
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
		Config:    cfg,
		Namespace: namespace,
		t:         t,
	}, nil
}

// InstallOrUpgrade installs/upgrades a Helm chart.
func (hd *HelmDeployer) InstallOrUpgrade(releaseName, chartPath string, values map[string]interface{}) error {
	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	// Check if release exists
	getAction := action.NewGet(hd.Config)
	_, err = getAction.Run(releaseName)
	releaseExists := err == nil

	if releaseExists {
		hd.t.Logf("Upgrading existing Helm release %s from %s", releaseName, chartPath)
		upgrade := action.NewUpgrade(hd.Config)
		upgrade.Wait = true
		upgrade.Timeout = 90 * time.Second
		_, err = upgrade.Run(releaseName, chart, values)
		if err != nil {
			return fmt.Errorf("failed to upgrade chart: %w", err)
		}
		hd.t.Logf("✓ Chart upgraded: %s", releaseName)
	} else {
		hd.t.Logf("Installing new Helm chart %s as %s", chartPath, releaseName)
		install := action.NewInstall(hd.Config)
		install.ReleaseName = releaseName
		install.Namespace = hd.Namespace
		install.Wait = true
		install.Timeout = 90 * time.Second
		_, err = install.Run(chart, values)
		if err != nil {
			return fmt.Errorf("failed to install chart: %w", err)
		}
		hd.t.Logf("✓ Chart installed: %s", releaseName)
	}
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
	upgrade.Wait = true
	upgrade.Timeout = 90 * time.Second
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

		var resource map[string]interface{}
		if err := yaml.Unmarshal(doc, &resource); err != nil {
			return nil, fmt.Errorf("failed to parse YAML document: %w", err)
		}
		if len(resource) > 0 {
			resources = append(resources, resource)
		}
	}

	return resources, nil
}
