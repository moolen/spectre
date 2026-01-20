package main

import (
	"context"
	"flag"
	"log"

	"github.com/moolen/spectre/tests/integration/golden/pkg/generator"
	"github.com/moolen/spectre/tests/integration/golden/pkg/scenarios"
)

func main() {
	var (
		clusterName        = flag.String("cluster", "spectre-golden", "Kind cluster name (ignored when --use-existing-cluster is set)")
		outputDir          = flag.String("output", "tests/integration/fixtures/golden", "Output directory")
		scenario           = flag.String("scenario", "", "Specific scenario to run (empty = all)")
		reuseCluster       = flag.Bool("reuse-cluster", false, "Reuse existing Kind cluster")
		useExistingCluster = flag.Bool("use-existing-cluster", false, "Use existing cluster from current KUBECONFIG instead of creating a Kind cluster")
		skipFlux           = flag.Bool("skip-flux", false, "Skip Flux CD installation (useful if Flux is already installed)")
	)
	flag.Parse()

	// Create output directory if it doesn't exist
	if err := createOutputDir(*outputDir); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	g := generator.New(generator.Config{
		ClusterName:        *clusterName,
		OutputDir:          *outputDir,
		ReuseCluster:       *reuseCluster,
		UseExistingCluster: *useExistingCluster,
		SkipFlux:           *skipFlux,
	})

	// Register all scenarios
	// Case 1: CrashLoopConfigMap
	g.RegisterScenario(scenarios.NewCrashLoopConfigMap())
	// Case 2: CrashLoopPullSecret
	g.RegisterScenario(scenarios.NewCrashLoopPullSecret())
	// Case 3: PendingTaint
	g.RegisterScenario(scenarios.NewPendingTaint())
	// Case 4: NodeEphemeralStoragePressure
	g.RegisterScenario(scenarios.NewNodeEphemeralStoragePressure())
	// Case 5: DeploymentRollout
	g.RegisterScenario(scenarios.NewDeploymentRollout())
	// Case 6: RBACViolation
	g.RegisterScenario(scenarios.NewRBACViolation())
	// Case 7a: HelmReleaseUpgrade
	g.RegisterScenario(scenarios.NewHelmReleaseUpgrade())
	// Case 7b: HelmReleaseValueFromFailure
	g.RegisterScenario(scenarios.NewHelmReleaseValueFromFailure())
	// Case 8: StatefulSetRollout
	g.RegisterScenario(scenarios.NewStatefulSetRollout())
	// Case 9: ServiceEndpointMismatch
	g.RegisterScenario(scenarios.NewServiceEndpointMismatch())
	// Case 9b: ServiceSelectorChange
	g.RegisterScenario(scenarios.NewServiceSelectorChange())
	// Case 10b: NetworkPolicyBlocking
	g.RegisterScenario(scenarios.NewNetworkPolicyBlocking())
	// Case 11: InitContainerFailed
	g.RegisterScenario(scenarios.NewInitContainerFailed())

	ctx := context.Background()

	if *scenario != "" {
		log.Printf("Running scenario: %s\n", *scenario)
		if err := g.RunScenario(ctx, *scenario); err != nil {
			log.Fatalf("Failed to run scenario %s: %v", *scenario, err)
		}
		log.Printf("✅ Scenario %s completed successfully\n", *scenario)
	} else {
		log.Println("Running all scenarios...")
		if err := g.RunAll(ctx); err != nil {
			log.Fatalf("Failed to run all scenarios: %v", err)
		}
		log.Println("✅ All scenarios completed successfully")
	}
}

func createOutputDir(dir string) error {
	return nil // Already handled in generator
}
