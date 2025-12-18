package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/moolen/spectre/tests/e2e/helpers"
)

var (
	sharedCluster    *helpers.TestCluster
	sharedClusterMu  sync.Mutex
	sharedClusterErr error
	imageBuilt       bool
	imageBuildMu     sync.Mutex
)

// TestMain sets up shared infrastructure for all UI e2e tests
func TestMain(m *testing.M) {
	exitCode := runWithSharedCluster(m)
	os.Exit(exitCode)
}

func runWithSharedCluster(m *testing.M) int {
	log.Println("Setting up shared Kind cluster for UI e2e tests...")

	// Create shared cluster
	cluster, err := helpers.CreateKindCluster(&testing.T{}, "spectre-ui-e2e-shared")
	if err != nil {
		log.Printf("❌ Failed to create shared cluster: %v", err)
		return 1
	}
	sharedCluster = cluster
	sharedClusterErr = nil

	// Set the shared cluster in helpers so tests can access it
	helpers.SetSharedCluster(cluster)

	log.Printf("✓ Shared cluster created: %s", cluster.Name)

	// Ensure cluster is cleaned up
	defer func() {
		log.Println("Cleaning up shared Kind cluster...")
		if err := cluster.Delete(); err != nil {
			log.Printf("⚠️  Warning: failed to delete shared cluster: %v", err)
		} else {
			log.Println("✓ Shared cluster deleted")
		}
	}()

	// Build and load Docker image once
	log.Println("Building test Docker image (once for all UI tests)...")
	values, imageRef, err := helpers.LoadHelmValues()
	if err != nil {
		log.Printf("❌ Failed to load Helm values: %v", err)
		return 1
	}

	if err := helpers.BuildAndLoadTestImage(&testing.T{}, cluster.Name, imageRef); err != nil {
		log.Printf("❌ Failed to build/load test image: %v", err)
		return 1
	}
	imageBuilt = true

	// Store values for reuse
	helpers.SetCachedHelmValues(values, imageRef)

	log.Println("✓ Test image built and loaded")
	log.Println("================================================")
	log.Println("Running UI e2e tests with shared cluster...")
	log.Println("================================================")

	// Run all tests
	exitCode := m.Run()

	log.Println("================================================")
	log.Printf("UI tests completed with exit code: %d", exitCode)
	log.Println("================================================")

	return exitCode
}

// GetSharedCluster returns the shared cluster instance
func GetSharedCluster() (*helpers.TestCluster, error) {
	sharedClusterMu.Lock()
	defer sharedClusterMu.Unlock()

	if sharedClusterErr != nil {
		return nil, sharedClusterErr
	}

	if sharedCluster == nil {
		return nil, fmt.Errorf("shared cluster not initialized")
	}

	return sharedCluster, nil
}

// IsImageBuilt returns whether the Docker image has been built
func IsImageBuilt() bool {
	imageBuildMu.Lock()
	defer imageBuildMu.Unlock()
	return imageBuilt
}

// CleanupTestNamespace performs cleanup for a test namespace
func CleanupTestNamespace(t *testing.T, cluster *helpers.TestCluster, namespace string) error {
	t.Helper()

	k8sClient, err := helpers.NewK8sClient(t, cluster.GetKubeConfig())
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Delete namespace (cascading delete)
	t.Logf("Deleting test namespace: %s", namespace)
	if err := k8sClient.DeleteNamespace(ctx, namespace); err != nil {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	// Wait for namespace to be fully deleted
	if err := k8sClient.WaitForNamespaceDeleted(ctx, namespace, 90*time.Second); err != nil {
		return fmt.Errorf("timeout waiting for namespace deletion: %w", err)
	}

	t.Logf("✓ Namespace deleted: %s", namespace)
	return nil
}
