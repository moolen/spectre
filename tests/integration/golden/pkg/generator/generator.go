package generator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/moolen/spectre/tests/integration/golden/pkg/cluster"
	"github.com/moolen/spectre/tests/integration/golden/pkg/flux"
	"github.com/moolen/spectre/tests/integration/golden/pkg/scenarios"
)

// Config defines generator configuration
type Config struct {
	ClusterName        string
	OutputDir          string
	ReuseCluster       bool
	UseExistingCluster bool   // Use existing cluster from KUBECONFIG instead of creating Kind cluster
	SkipFlux           bool   // Skip Flux CD installation
	SpectreBinary      string // Path to spectre binary (default: builds from source)
	WatcherConfig      string // Path to watcher config (optional)
}

// Generator orchestrates fixture generation
type Generator struct {
	config       Config
	scenarios    map[string]scenarios.Scenario
	cluster      *cluster.Cluster
	spectreCmd   *exec.Cmd
	auditLogPath string
}

// New creates a new generator
func New(cfg Config) *Generator {
	return &Generator{
		config:    cfg,
		scenarios: make(map[string]scenarios.Scenario),
	}
}

// RegisterScenario registers a scenario to be executed
func (g *Generator) RegisterScenario(s scenarios.Scenario) {
	g.scenarios[s.Name()] = s
}

// RunAll executes all registered scenarios
func (g *Generator) RunAll(ctx context.Context) error {
	// Setup cluster
	if err := g.setupCluster(ctx); err != nil {
		return fmt.Errorf("cluster setup failed: %w", err)
	}
	defer g.teardownCluster(ctx)

	// Ensure output directory exists
	if err := os.MkdirAll(g.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Run each scenario
	for name, scenario := range g.scenarios {
		fmt.Printf("\n=== Running scenario: %s ===\n", name)
		if err := g.runScenario(ctx, name, scenario); err != nil {
			fmt.Printf("Scenario %s failed: %v\n", name, err)
			return fmt.Errorf("scenario %s failed: %w", name, err)
		}
		fmt.Printf("Scenario %s completed\n", name)
	}

	return nil
}

// RunScenario runs a single scenario by name
func (g *Generator) RunScenario(ctx context.Context, scenarioName string) error {
	scenario, ok := g.scenarios[scenarioName]
	if !ok {
		return fmt.Errorf("scenario %s not found", scenarioName)
	}

	// Setup cluster
	if err := g.setupCluster(ctx); err != nil {
		return fmt.Errorf("cluster setup failed: %w", err)
	}
	defer g.teardownCluster(ctx)

	// Ensure output directory exists
	if err := os.MkdirAll(g.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	return g.runScenario(ctx, scenarioName, scenario)
}

func (g *Generator) setupCluster(ctx context.Context) error {
	if g.config.UseExistingCluster {
		fmt.Println("Using existing cluster from KUBECONFIG")
	} else {
		fmt.Printf("Setting up Kind cluster: %s\n", g.config.ClusterName)
	}

	cfg := cluster.Config{
		Name:        g.config.ClusterName,
		Reuse:       g.config.ReuseCluster,
		UseExisting: g.config.UseExistingCluster,
	}

	clusterObj, err := cluster.New(ctx, cfg)
	if err != nil {
		return err
	}

	// Wait for cluster to be ready
	fmt.Println("Waiting for cluster to be ready...")
	if err := clusterObj.WaitForReady(ctx); err != nil {
		return err
	}

	g.cluster = clusterObj
	fmt.Println("Cluster is ready")

	if !g.config.SkipFlux {
		if err := g.installFlux(ctx); err != nil {
			return fmt.Errorf("failed to install Flux: %w", err)
		}
	} else {
		fmt.Println("Skipping Flux CD installation (--skip-flux)")
	}

	// Start Spectre locally in audit-only mode
	if err := g.startSpectreLocal(ctx); err != nil {
		return fmt.Errorf("failed to start Spectre: %w", err)
	}

	return nil
}

func (g *Generator) startSpectreLocal(ctx context.Context) error {
	// Create temp file for audit log
	tmpDir, err := os.MkdirTemp("", "spectre-golden-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	g.auditLogPath = filepath.Join(tmpDir, "audit.jsonl")

	// Determine spectre binary path
	spectreBinary := g.config.SpectreBinary
	if spectreBinary == "" {
		// Build from source
		fmt.Println("Building Spectre from source...")
		spectreBinary = filepath.Join(tmpDir, "spectre")
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", spectreBinary, "./cmd/spectre")
		buildCmd.Dir = findProjectRoot()
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("failed to build spectre: %w", err)
		}
		fmt.Println("Spectre built successfully")
	}

	// Prepare watcher config path
	watcherConfig := g.config.WatcherConfig
	if watcherConfig == "" {
		// Use default watcher config that watches common resources
		watcherConfig = filepath.Join(tmpDir, "watcher.yaml")
		if err := os.WriteFile(watcherConfig, []byte(defaultWatcherConfig), 0644); err != nil {
			return fmt.Errorf("failed to write watcher config: %w", err)
		}
	}

	// Get kubeconfig from Kind cluster
	kubeconfigPath := g.cluster.KubeConfig()

	fmt.Printf("Starting Spectre in audit-only mode...\n")
	fmt.Printf("  Kubeconfig: %s\n", kubeconfigPath)
	fmt.Printf("  Audit log: %s\n", g.auditLogPath)
	fmt.Printf("  Watcher config: %s\n", watcherConfig)

	// Start Spectre in audit-only mode (no graph database)
	g.spectreCmd = exec.CommandContext(ctx, spectreBinary, "server",
		"--audit-log="+g.auditLogPath,
		"--watcher-config="+watcherConfig,
		"--watcher-enabled=true",
		"--graph-enabled=false",
	)

	// Set KUBECONFIG environment variable
	g.spectreCmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)

	// Redirect output for debugging
	g.spectreCmd.Stdout = os.Stdout
	g.spectreCmd.Stderr = os.Stderr

	if err := g.spectreCmd.Start(); err != nil {
		return fmt.Errorf("failed to start spectre: %w", err)
	}

	fmt.Printf("Spectre started (PID: %d)\n", g.spectreCmd.Process.Pid)

	// Wait for Spectre to be ready (give it time to connect to cluster)
	fmt.Println("Waiting for Spectre to initialize...")
	time.Sleep(5 * time.Second)

	// Verify process is still running
	if g.spectreCmd.ProcessState != nil && g.spectreCmd.ProcessState.Exited() {
		return fmt.Errorf("spectre process exited unexpectedly")
	}

	fmt.Println("Spectre is ready")
	return nil
}

func (g *Generator) stopSpectre() error {
	if g.spectreCmd == nil || g.spectreCmd.Process == nil {
		return nil
	}

	fmt.Println("Stopping Spectre...")

	// Send SIGTERM for graceful shutdown
	if err := g.spectreCmd.Process.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("Warning: failed to send SIGTERM: %v\n", err)
		// Try SIGKILL as fallback
		if err := g.spectreCmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill spectre process: %w", err)
		}
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- g.spectreCmd.Wait()
	}()

	select {
	case <-done:
		fmt.Println("Spectre stopped")
	case <-time.After(10 * time.Second):
		fmt.Println("Timeout waiting for Spectre to stop, killing...")
		g.spectreCmd.Process.Kill()
	}

	return nil
}

func (g *Generator) installFlux(ctx context.Context) error {
	// Install Flux CD for HelmRelease scenarios
	client := g.cluster.Client()
	kubeconfigPath := g.cluster.KubeConfig()

	if err := flux.InstallFlux(ctx, client, kubeconfigPath); err != nil {
		return err
	}

	fmt.Println("Flux CD installed successfully")
	return nil
}

func (g *Generator) teardownCluster(ctx context.Context) error {
	// Stop Spectre first
	if err := g.stopSpectre(); err != nil {
		fmt.Printf("Warning: failed to stop Spectre: %v\n", err)
	}

	// Cleanup cluster (skipped for external clusters)
	if g.cluster != nil {
		if g.cluster.IsExternal() {
			fmt.Println("Skipping cluster cleanup (external cluster)")
		} else {
			fmt.Printf("Cleaning up cluster: %s\n", g.config.ClusterName)
		}
		return g.cluster.Cleanup(ctx)
	}
	return nil
}

func (g *Generator) runScenario(ctx context.Context, name string, s scenarios.Scenario) error {
	// Create isolated namespace
	namespace := fmt.Sprintf("golden-%s-%d", name, time.Now().Unix())
	fmt.Printf("Creating namespace: %s\n", namespace)

	client := g.cluster.Client()
	kubeconfigPath := g.cluster.KubeConfig()

	// Set kubeconfig in environment for scenarios to use
	oldKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", kubeconfigPath)
	defer os.Setenv("KUBECONFIG", oldKubeconfig)

	// Create namespace
	nsObj := &metav1.ObjectMeta{
		Name: namespace,
	}

	if _, err := client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: *nsObj,
	}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	// Execute scenario phases
	fmt.Printf("Setting up scenario...\n")
	if err := s.Setup(ctx, client, namespace); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	// Wait for initial state to settle
	fmt.Printf("Waiting for initial state to settle...\n")
	time.Sleep(5 * time.Second)

	// Start event capture
	fmt.Printf("Executing scenario action...\n")
	if err := s.Execute(ctx, client, namespace); err != nil {
		return fmt.Errorf("execute failed: %w", err)
	}

	// Wait for condition with timeout
	fmt.Printf("Waiting for scenario condition...\n")
	waitCtx, cancel := context.WithTimeout(ctx, s.Timeout())
	defer cancel()

	if err := s.WaitCondition(waitCtx, client, namespace); err != nil {
		return fmt.Errorf("wait condition failed: %w", err)
	}

	// Allow additional time for events to propagate
	fmt.Printf("Collecting events...\n")
	time.Sleep(10 * time.Second)

	// Capture events from local audit log
	if err := g.captureEvents(ctx, namespace, name); err != nil {
		return fmt.Errorf("failed to capture events: %w", err)
	}

	// Cleanup
	fmt.Printf("Cleaning up scenario...\n")
	if err := s.Cleanup(ctx, client, namespace); err != nil {
		fmt.Printf("Warning: cleanup failed for %s: %v\n", name, err)
	}

	// Delete namespace
	fmt.Printf("Deleting namespace: %s\n", namespace)
	if err := client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil {
		fmt.Printf("Warning: failed to delete namespace: %v\n", err)
	}

	// Write metadata file alongside fixture
	metaFile := filepath.Join(g.config.OutputDir, fmt.Sprintf("%s.meta.json", name))
	if err := g.writeMetadata(metaFile, s); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

func (g *Generator) captureEvents(ctx context.Context, namespace, scenarioName string) error {
	fmt.Printf("Capturing audit events from local file: %s\n", g.auditLogPath)

	// Read audit log from local file
	auditLogContent, err := os.ReadFile(g.auditLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Warning: No audit log file found, creating empty fixture\n")
			auditLogContent = []byte("")
		} else {
			return fmt.Errorf("failed to read audit log: %w", err)
		}
	}

	if len(auditLogContent) == 0 {
		fmt.Printf("Warning: No audit log content found, creating empty fixture\n")
	}

	// Filter events for the specific namespace if needed
	filteredContent, err := g.filterAuditLogByNamespace(auditLogContent, namespace)
	if err != nil {
		fmt.Printf("Warning: Failed to filter audit log, using all events: %v\n", err)
		filteredContent = auditLogContent
	}

	fixtureFile := filepath.Join(g.config.OutputDir, fmt.Sprintf("%s.jsonl", scenarioName))
	if err := os.WriteFile(fixtureFile, filteredContent, 0644); err != nil {
		return fmt.Errorf("failed to write fixture file: %w", err)
	}

	// Count lines to report
	lineCount := bytes.Count(filteredContent, []byte("\n"))
	if len(filteredContent) > 0 && filteredContent[len(filteredContent)-1] != '\n' {
		lineCount++ // Last line without newline
	}
	fmt.Printf("Fixture written to: %s (%d events)\n", fixtureFile, lineCount)

	return nil
}

func (g *Generator) filterAuditLogByNamespace(content []byte, namespace string) ([]byte, error) {
	if len(content) == 0 {
		return content, nil
	}

	var filtered bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Try to parse as JSON to check namespace
		var event map[string]interface{}
		if err := json.Unmarshal(line, &event); err != nil {
			// If we can't parse, include it anyway
			filtered.Write(line)
			filtered.WriteByte('\n')
			continue
		}

		// Check if event belongs to the namespace or is cluster-scoped
		if resource, ok := event["resource"].(map[string]interface{}); ok {
			ns, _ := resource["namespace"].(string)
			// Include if: namespace matches OR is cluster-scoped (empty namespace)
			if ns == namespace || ns == "" {
				filtered.Write(line)
				filtered.WriteByte('\n')
			}
		} else {
			// If no resource field at all, include it
			filtered.Write(line)
			filtered.WriteByte('\n')
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning audit log: %w", err)
	}

	return filtered.Bytes(), nil
}

func (g *Generator) writeMetadata(metaFile string, s scenarios.Scenario) error {
	metadata := map[string]interface{}{
		"scenario": map[string]interface{}{
			"name":        s.Name(),
			"description": s.Description(),
		},
		"expected": map[string]interface{}{
			"anomalies":   s.ExpectedAnomalies(),
			"causal_path": s.ExpectedCausalPath(),
		},
		"timestamp": time.Now().Unix(),
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaFile, data, 0644)
}

// findProjectRoot finds the root of the spectre project
func findProjectRoot() string {
	// Start from current directory and walk up looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "."
		}
		dir = parent
	}
}

// Default watcher config for golden tests - watches common K8s resources
const defaultWatcherConfig = `
resources:
  # Core resources
  - group: ""
    version: "v1"
    kind: "Pod"
  - group: ""
    version: "v1"
    kind: "Service"
  - group: ""
    version: "v1"
    kind: "ConfigMap"
  - group: ""
    version: "v1"
    kind: "Secret"
  - group: ""
    version: "v1"
    kind: "Event"
  - group: ""
    version: "v1"
    kind: "Namespace"
  - group: ""
    version: "v1"
    kind: "ServiceAccount"
  # Cluster-scoped resources (for Node taints, etc.)
  - group: ""
    version: "v1"
    kind: "Node"

  # Apps
  - group: "apps"
    version: "v1"
    kind: "Deployment"
  - group: "apps"
    version: "v1"
    kind: "ReplicaSet"
  - group: "apps"
    version: "v1"
    kind: "StatefulSet"
  - group: "apps"
    version: "v1"
    kind: "DaemonSet"

  # Networking
  - group: "networking.k8s.io"
    version: "v1"
    kind: "Ingress"
  - group: "networking.k8s.io"
    version: "v1"
    kind: "NetworkPolicy"

  # RBAC
  - group: "rbac.authorization.k8s.io"
    version: "v1"
    kind: "Role"
  - group: "rbac.authorization.k8s.io"
    version: "v1"
    kind: "ClusterRole"
  - group: "rbac.authorization.k8s.io"
    version: "v1"
    kind: "RoleBinding"
  - group: "rbac.authorization.k8s.io"
    version: "v1"
    kind: "ClusterRoleBinding"

  # Batch
  - group: "batch"
    version: "v1"
    kind: "Job"
  - group: "batch"
    version: "v1"
    kind: "CronJob"

  # Flux CD - Helm Controller
  - group: "helm.toolkit.fluxcd.io"
    version: "v2"
    kind: "HelmRelease"

  # Flux CD - Source Controller
  - group: "source.toolkit.fluxcd.io"
    version: "v1"
    kind: "HelmRepository"
  - group: "source.toolkit.fluxcd.io"
    version: "v1"
    kind: "HelmChart"
  - group: "source.toolkit.fluxcd.io"
    version: "v1"
    kind: "GitRepository"
  - group: "source.toolkit.fluxcd.io"
    version: "v1"
    kind: "OCIRepository"

  # Flux CD - Kustomize Controller
  - group: "kustomize.toolkit.fluxcd.io"
    version: "v1"
    kind: "Kustomization"
`
