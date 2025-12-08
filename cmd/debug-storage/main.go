package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

type Config struct {
	FilePath  string
	Group     string
	Version   string
	Kind      string
	Namespace string
	Name      string
	Extended  bool
	JSONOutput bool
}

func main() {
	config := parseFlags()

	if config.FilePath == "" {
		fmt.Fprintf(os.Stderr, "Error: --file flag is required\n")
		os.Exit(1)
	}

	// Verify file exists
	if _, err := os.Stat(config.FilePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", config.FilePath)
		os.Exit(1)
	}

	// Read and decode the file
	fileData, err := ReadStorageFile(config.FilePath)
	if err != nil {
		log.Fatalf("Error reading storage file: %v", err)
	}

	// Apply filters
	filters := &EventFilter{
		Group:     config.Group,
		Version:   config.Version,
		Kind:      config.Kind,
		Namespace: config.Namespace,
		Name:      config.Name,
	}
	filteredEvents := filterEvents(fileData.Events, filters)

	// Format and output
	if config.JSONOutput {
		if err := formatJSON(fileData, filteredEvents); err != nil {
			log.Fatalf("Error formatting JSON: %v", err)
		}
	} else if config.Extended {
		if err := formatExtended(fileData, filteredEvents, filters); err != nil {
			log.Fatalf("Error formatting extended output: %v", err)
		}
	} else {
		if err := formatSummary(fileData, filteredEvents, filters); err != nil {
			log.Fatalf("Error formatting summary output: %v", err)
		}
	}
}

func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.FilePath, "file", "", "Path to storage file (required)")
	flag.StringVar(&config.Group, "group", "", "Filter by API group")
	flag.StringVar(&config.Version, "version", "", "Filter by API version")
	flag.StringVar(&config.Kind, "kind", "", "Filter by resource kind")
	flag.StringVar(&config.Namespace, "namespace", "", "Filter by namespace")
	flag.StringVar(&config.Name, "name", "", "Filter by resource name")
	flag.BoolVar(&config.Extended, "extended", false, "Show extended details")
	flag.BoolVar(&config.Extended, "x", false, "Show extended details (shorthand)")
	flag.BoolVar(&config.JSONOutput, "json", false, "Output as JSON")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: debug-storage [flags]

Reads, decodes, and displays contents of Spectre storage files.

Flags:
`)
		flag.PrintDefaults()
	}

	flag.Parse()
	return config
}
