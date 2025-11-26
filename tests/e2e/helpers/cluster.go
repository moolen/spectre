// Package helpers provides reusable utilities for e2e testing.
package helpers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
)

// TestCluster represents a Kind cluster instance for testing.
type TestCluster struct {
	Provider   *cluster.Provider
	Name       string
	KubeConfig string
	Context    string
	t           *testing.T
}

// CreateKindCluster creates a new Kind cluster with a unique name.
func CreateKindCluster(t *testing.T, clusterName string) (*TestCluster, error) {
	t.Logf("Creating Kind cluster: %s", clusterName)

	// Create Kind provider
	provider := cluster.NewProvider()

	// Define cluster configuration
	cfg := &v1alpha4.Cluster{
		TypeMeta: v1alpha4.TypeMeta{
			APIVersion: "kind.x-k8s.io/v1alpha4",
			Kind:       "Cluster",
		},
		Name: clusterName,
		Nodes: []v1alpha4.Node{
			{
				Role: v1alpha4.ControlPlaneRole,
			},
		},
	}

	// Create the cluster
	if err := provider.Create(clusterName, cluster.CreateWithV1Alpha4Config(cfg)); err != nil {
		return nil, fmt.Errorf("failed to create Kind cluster: %w", err)
	}

	t.Logf("✓ Kind cluster created: %s", clusterName)

	// Get kubeconfig
	kubeConfigPath := filepath.Join(os.TempDir(), fmt.Sprintf("kubeconfig-%s", clusterName))
	if err := provider.ExportKubeConfig(clusterName, kubeConfigPath, false); err != nil {
		return nil, fmt.Errorf("failed to export kubeconfig: %w", err)
	}

	t.Logf("✓ Kubeconfig exported to: %s", kubeConfigPath)

	return &TestCluster{
		Provider:   provider,
		Name:       clusterName,
		KubeConfig: kubeConfigPath,
		Context:    fmt.Sprintf("kind-%s", clusterName),
		t:           t,
	}, nil
}

// Delete removes the Kind cluster.
func (tc *TestCluster) Delete() error {
	tc.t.Logf("Deleting Kind cluster: %s", tc.Name)

	if err := tc.Provider.Delete(tc.Name, kubeConfigPath(tc.KubeConfig)); err != nil {
		// Log but don't fail if deletion is already in progress
		if !strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("failed to delete cluster: %w", err)
		}
	}

	// Clean up kubeconfig file
	if err := os.Remove(tc.KubeConfig); err != nil && !os.IsNotExist(err) {
		tc.t.Logf("Warning: failed to remove kubeconfig: %v", err)
	}

	tc.t.Logf("✓ Kind cluster deleted: %s", tc.Name)
	return nil
}

// GetContext returns the Kubernetes context name for this cluster.
func (tc *TestCluster) GetContext() string {
	return tc.Context
}

// GetKubeConfig returns the path to the kubeconfig file.
func (tc *TestCluster) GetKubeConfig() string {
	return tc.KubeConfig
}

// kubeConfigPath returns the kubeconfig path for the given Kind cluster.
func kubeConfigPath(kubeconfigPath string) string {
	// Kind provider expects empty string for default kubeconfig location
	if kubeconfigPath == "" {
		return ""
	}
	return kubeconfigPath
}
