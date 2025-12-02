package main

import (
	"flag"
	"os"
)

// Config holds the configuration for the MCP server
type Config struct {
	SpectreURL string
	HTTPAddr   string
	LogLevel   string
}

// LoadConfig loads configuration from environment variables and command-line flags
func LoadConfig() Config {
	cfg := Config{
		SpectreURL: "http://localhost:8080",
		HTTPAddr:   ":8081",
		LogLevel:   "info",
	}

	// Check environment variables first
	if url := os.Getenv("SPECTRE_URL"); url != "" {
		cfg.SpectreURL = url
	}

	if addr := os.Getenv("MCP_HTTP_ADDR"); addr != "" {
		cfg.HTTPAddr = addr
	}

	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.LogLevel = level
	}

	// Command-line flags override environment variables
	flag.StringVar(&cfg.SpectreURL, "spectre-url", cfg.SpectreURL, "URL to Spectre API server")
	flag.StringVar(&cfg.HTTPAddr, "http-addr", cfg.HTTPAddr, "HTTP server address (host:port)")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")

	flag.Parse()

	return cfg
}
