// Package helpers provides reusable utilities for e2e testing.
package helpers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
)

// TestCluster represents a Kind cluster instance for testing.
type TestCluster struct {
	Provider *cluster.Provider
	Name     string
	Context  string
	t        *testing.T
}

// CreateKindCluster creates a new Kind cluster with a unique name.
// If a cluster with the same name already exists, it will be deleted first.
// Kind automatically manages kubeconfig in ~/.kube/config.
func CreateKindCluster(t *testing.T, clusterName string) (*TestCluster, error) {
	t.Logf("Creating Kind cluster: %s", clusterName)

	// Create Kind provider
	provider := cluster.NewProvider()

	// Check if cluster already exists and delete it if so
	clusters, err := provider.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list existing clusters: %w", err)
	}

	for _, existingCluster := range clusters {
		if existingCluster == clusterName {
			t.Logf("Found existing cluster %s, deleting it first...", clusterName)
			if err := provider.Delete(clusterName, ""); err != nil {
				// Log but continue - cluster might be partially deleted
				if !strings.Contains(err.Error(), "does not exist") {
					t.Logf("Warning: failed to delete existing cluster: %v", err)
				}
			} else {
				t.Logf("✓ Deleted existing cluster: %s", clusterName)
			}
			break
		}
	}

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

	// Create the cluster (Kind automatically updates ~/.kube/config)
	if err := provider.Create(clusterName, cluster.CreateWithV1Alpha4Config(cfg)); err != nil {
		return nil, fmt.Errorf("failed to create Kind cluster: %w", err)
	}

	t.Logf("✓ Kind cluster created: %s", clusterName)

	return &TestCluster{
		Provider: provider,
		Name:     clusterName,
		Context:  fmt.Sprintf("kind-%s", clusterName),
		t:        t,
	}, nil
}

// Delete removes the Kind cluster.
func (tc *TestCluster) Delete() error {
	tc.t.Logf("Deleting Kind cluster: %s", tc.Name)

	if err := tc.Provider.Delete(tc.Name, ""); err != nil {
		// Log but don't fail if deletion is already in progress
		if !strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("failed to delete cluster: %w", err)
		}
	}

	tc.t.Logf("✓ Kind cluster deleted: %s", tc.Name)
	return nil
}

// GetContext returns the Kubernetes context name for this cluster.
func (tc *TestCluster) GetContext() string {
	return tc.Context
}

// GetOrCreateKindCluster gets an existing Kind cluster or creates a new one.
// If a cluster with the given name exists and is healthy, it will be reused.
// Otherwise, a new cluster will be created.
// This allows cluster reuse across multiple test invocations for faster testing.
func GetOrCreateKindCluster(t *testing.T, clusterName string) (*TestCluster, error) {
	t.Logf("Getting or creating Kind cluster: %s", clusterName)

	// Create Kind provider
	provider := cluster.NewProvider()

	// Check if cluster already exists
	clusters, err := provider.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list existing clusters: %w", err)
	}

	clusterExists := false
	for _, existingCluster := range clusters {
		if existingCluster == clusterName {
			clusterExists = true
			break
		}
	}

	if clusterExists {
		t.Logf("Found existing cluster %s, checking health...", clusterName)

		// Check if cluster is healthy
		tc := &TestCluster{
			Provider: provider,
			Name:     clusterName,
			Context:  fmt.Sprintf("kind-%s", clusterName),
			t:        t,
		}

		if isClusterHealthy(t, tc) {
			t.Logf("✓ Reusing existing healthy cluster: %s", clusterName)
			return tc, nil
		}

		// Cluster exists but is unhealthy, delete and recreate
		t.Logf("Cluster %s is unhealthy, deleting and recreating...", clusterName)
		if err := provider.Delete(clusterName, ""); err != nil {
			if !strings.Contains(err.Error(), "does not exist") {
				t.Logf("Warning: failed to delete unhealthy cluster: %v", err)
			}
		}
	}

	// Create new cluster
	t.Logf("Creating new Kind cluster: %s", clusterName)
	return CreateKindCluster(t, clusterName)
}

// isClusterHealthy checks if a Kind cluster is healthy and accessible
func isClusterHealthy(t *testing.T, tc *TestCluster) bool {
	// Create a Kubernetes client for the specific context
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: clientcmd.RecommendedHomeFile},
		&clientcmd.ConfigOverrides{CurrentContext: tc.Context},
	)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		t.Logf("Failed to get REST config: %v", err)
		return false
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Logf("Failed to create clientset: %v", err)
		return false
	}

	// Try to list nodes with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Logf("Failed to list nodes: %v", err)
		return false
	}

	if len(nodes.Items) == 0 {
		t.Logf("No nodes found in cluster")
		return false
	}

	// Check if all nodes are ready
	for _, node := range nodes.Items {
		ready := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
				break
			}
		}
		if !ready {
			t.Logf("Node %s is not ready", node.Name)
			return false
		}
	}

	t.Logf("Cluster %s is healthy with %d ready node(s)", tc.Name, len(nodes.Items))
	return true
}
