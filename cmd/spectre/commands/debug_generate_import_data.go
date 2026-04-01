package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/moolen/spectre/internal/importexport/synthetic"
	"github.com/spf13/cobra"
)

type generateImportDataOptions struct {
	OutputDir      string
	Seed           int64
	KindCount      int
	ResourceCount  int
	NamespaceCount int
}

var generateImportDataOpts generateImportDataOptions

var debugGenerateImportDataCmd = &cobra.Command{
	Use:   "generate-import-data",
	Short: "Generate synthetic importer-ready JSON files and summary.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		if generateImportDataOpts.OutputDir == "" {
			return fmt.Errorf("output directory is required")
		}

		summary, err := synthetic.GenerateDataset(generateImportDataOpts.OutputDir, synthetic.Config{
			Seed:           generateImportDataOpts.Seed,
			KindCount:      generateImportDataOpts.KindCount,
			ResourceCount:  generateImportDataOpts.ResourceCount,
			NamespaceCount: generateImportDataOpts.NamespaceCount,
		})
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal summary for stdout: %w", err)
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return fmt.Errorf("write summary to stdout: %w", err)
		}
		if _, err := os.Stdout.WriteString("\n"); err != nil {
			return fmt.Errorf("write trailing newline: %w", err)
		}

		return nil
	},
}

func init() {
	debugGenerateImportDataCmd.Flags().StringVar(&generateImportDataOpts.OutputDir, "output-dir", "", "Directory to write generated dataset")
	debugGenerateImportDataCmd.Flags().Int64Var(&generateImportDataOpts.Seed, "seed", 42, "PRNG seed for deterministic generation")
	debugGenerateImportDataCmd.Flags().IntVar(&generateImportDataOpts.KindCount, "kinds", 55, "Number of kinds to generate")
	debugGenerateImportDataCmd.Flags().IntVar(&generateImportDataOpts.ResourceCount, "resources", 5000, "Number of resources to generate")
	debugGenerateImportDataCmd.Flags().IntVar(&generateImportDataOpts.NamespaceCount, "namespaces", 20, "Number of namespaces to spread resources across")
}
