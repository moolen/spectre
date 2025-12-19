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

// TestMain sets up shared infrastructure for all e2e tests
func TestMain(m *testing.M) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("TestMain starting...")
	exitCode := runWithSharedCluster(m)
	log.Println("TestMain exiting with code:", exitCode)
	os.Exit(exitCode)
}

func runWithSharedCluster(m *testing.M) int {
	log.Println("Setting up shared Kind cluster for e2e tests...")
	startTime := time.Now()

	// Get or create shared cluster (reuses existing healthy clusters)
	clusterStartTime := time.Now()
	cluster, err := helpers.GetOrCreateKindCluster(&testing.T{}, "spectre-e2e-shared")
	if err != nil {
		log.Printf("❌ Failed to get/create shared cluster: %v", err)
		return 1
	}
	sharedCluster = cluster
	sharedClusterErr = nil

	// Set the shared cluster in helpers so tests can access it
	helpers.SetSharedCluster(cluster)

	log.Printf("✓ Shared cluster ready: %s (took %v)", cluster.Name, time.Since(clusterStartTime))
	log.Printf("✓ Cluster context: %s", cluster.Context)

	// Note: Cluster is NOT deleted after tests to allow reuse across test invocations.
	// To manually clean up, run: kind delete cluster --name spectre-e2e-shared
	// Or use: make clean-test-clusters

	// Build and load Docker image once
	log.Println("Building test Docker image (once for all tests)...")
	imageStartTime := time.Now()
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

	log.Printf("✓ Test image built and loaded (took %v)", time.Since(imageStartTime))
	log.Printf("📊 Total setup time: %v", time.Since(startTime))
	log.Println("================================================")
	log.Println("Running e2e tests with shared cluster...")
	log.Println("================================================")

	// Run all tests
	testStartTime := time.Now()
	exitCode := m.Run()
	testDuration := time.Since(testStartTime)

	log.Println("================================================")
	log.Printf("Tests completed with exit code: %d (execution took %v)", exitCode, testDuration)
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

	k8sClient, err := helpers.NewK8sClient(t, cluster.GetContext())
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
