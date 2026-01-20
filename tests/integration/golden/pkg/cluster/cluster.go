package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
)

// Cluster manages a Kind cluster for golden testing
type Cluster struct {
	name       string
	provider   *cluster.Provider
	client     kubernetes.Interface
	kubeconfig string
	isExternal bool // true if using external cluster from KUBECONFIG (not managed by us)
}

// Config defines cluster creation options
type Config struct {
	Name              string
	Reuse             bool
	KubernetesVersion string
	UseExisting       bool // Use existing cluster from KUBECONFIG instead of creating Kind
}

// New creates or reuses a Kind cluster, or connects to an existing cluster from KUBECONFIG
func New(ctx context.Context, cfg Config) (*Cluster, error) {
	// If using existing cluster from KUBECONFIG
	if cfg.UseExisting {
		return newFromKubeconfig(ctx)
	}

	return newKindCluster(ctx, cfg)
}

// newFromKubeconfig creates a Cluster that uses the existing cluster from KUBECONFIG
func newFromKubeconfig(ctx context.Context) (*Cluster, error) {
	// Get kubeconfig path from environment or default location
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	}

	// Verify kubeconfig file exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("kubeconfig file not found at %s", kubeconfigPath)
	}

	// Load kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", kubeconfigPath, err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Cluster{
		name:       "external",
		provider:   nil, // No Kind provider for external clusters
		client:     clientset,
		kubeconfig: kubeconfigPath,
		isExternal: true,
	}, nil
}

// newKindCluster creates or reuses a Kind cluster
func newKindCluster(ctx context.Context, cfg Config) (*Cluster, error) {
	provider := cluster.NewProvider()

	// Check if cluster exists
	clusters, err := provider.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	clusterExists := false
	for _, c := range clusters {
		if c == cfg.Name {
			clusterExists = true
			break
		}
	}

	if clusterExists && !cfg.Reuse {
		// Delete existing cluster
		if err := provider.Delete(cfg.Name, ""); err != nil {
			return nil, fmt.Errorf("failed to delete existing cluster: %w", err)
		}
	}

	if !clusterExists || !cfg.Reuse {
		// Create new cluster
		node := v1alpha4.Node{
			Role: v1alpha4.ControlPlaneRole,
		}

		// Default to latest stable version (v1.33.1 as of Jan 2026)
		nodeImage := "kindest/node:v1.33.1"
		if cfg.KubernetesVersion != "" {
			nodeImage = fmt.Sprintf("kindest/node:%s", cfg.KubernetesVersion)
		}
		node.Image = nodeImage

		clusterConfig := &v1alpha4.Cluster{
			TypeMeta: v1alpha4.TypeMeta{
				APIVersion: "kind.x-k8s.io/v1alpha4",
				Kind:       "Cluster",
			},
			Name: cfg.Name,
			Nodes: []v1alpha4.Node{node},
		}

		if err := provider.Create(cfg.Name, cluster.CreateWithV1Alpha4Config(clusterConfig)); err != nil {
			return nil, fmt.Errorf("failed to create cluster: %w", err)
		}
	}

	// Get kubeconfig
	kubeconfig, err := provider.KubeConfig(cfg.Name, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Create clientset from kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Write kubeconfig to file for external tools
	kubeconfigPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-kubeconfig.yaml", cfg.Name))
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600); err != nil {
		return nil, fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return &Cluster{
		name:       cfg.Name,
		provider:   provider,
		client:     clientset,
		kubeconfig: kubeconfigPath,
		isExternal: false,
	}, nil
}

// Client returns the Kubernetes client
func (c *Cluster) Client() kubernetes.Interface {
	return c.client
}

// KubeConfig returns the kubeconfig path
func (c *Cluster) KubeConfig() string {
	return c.kubeconfig
}

// Cleanup tears down the cluster (skipped for external clusters)
func (c *Cluster) Cleanup(ctx context.Context) error {
	// Never delete external clusters - they are not managed by us
	if c.isExternal {
		return nil
	}

	if c.provider == nil {
		return nil
	}

	if err := c.provider.Delete(c.name, ""); err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	// Clean up kubeconfig file (only for Kind clusters where we created a temp file)
	_ = os.Remove(c.kubeconfig)

	return nil
}

// IsExternal returns true if this is an external cluster (not managed by us)
func (c *Cluster) IsExternal() bool {
	return c.isExternal
}

// WaitForReady waits for the cluster to be ready
func (c *Cluster) WaitForReady(ctx context.Context) error {
	// Try to list namespaces as a health check
	return WaitCondition(ctx, 5*time.Minute, func() (bool, error) {
		_, err := c.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
		return err == nil, err
	})
}

// EventCapture manages event capture from a cluster
type EventCapture struct {
	ctx      context.Context
	filepath string
}

// StartEventCapture starts capturing events from the cluster (uses DinD)
func (c *Cluster) StartEventCapture(ctx context.Context, namespace, outputFile string) (*EventCapture, error) {
	// For now, return a no-op implementation
	// In real implementation, this would use Spectre agent or kubectl watch

	return &EventCapture{
		ctx:      ctx,
		filepath: outputFile,
	}, nil
}

// Stop stops event capture
func (ec *EventCapture) Stop() error {
	// No-op for now
	return nil
}

// GetEvents returns captured events as JSONL
func (ec *EventCapture) GetEvents() (string, error) {
	// Would read from file
	return "", nil
}

// WaitCondition is a helper for polling
func WaitCondition(ctx context.Context, timeout time.Duration, check func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(timeout):
			return fmt.Errorf("timeout waiting for condition")
		case <-ticker.C:
			done, err := check()
			if err != nil {
				// Don't fail on transient errors
				continue
			}
			if done {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for condition")
			}
		}
	}
}
