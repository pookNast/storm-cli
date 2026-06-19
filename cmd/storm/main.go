// Command storm is the STORM research CLI.
// Usage: storm research --topic "<topic>" --role "<role>" [flags]
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pookNast/storm-cli/internal/briefing"
	"github.com/pookNast/storm-cli/internal/config"
	"github.com/pookNast/storm-cli/internal/obs"
	"github.com/pookNast/storm-cli/internal/storm"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("storm", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, usageText)
		fs.PrintDefaults()
	}

	var (
		topic   string
		role    string
		format  string
		outDir  string
		verbose bool
	)

	fs.StringVar(&topic, "topic", "", "Research topic (required)")
	fs.StringVar(&role, "role", "analyst", "Analyst role context (e.g. analyst, investor, policymaker)")
	fs.StringVar(&format, "format", "both", "Output format: json, md, or both")
	fs.StringVar(&outDir, "out", "./out", "Output directory for briefing files")
	fs.BoolVar(&verbose, "verbose", false, "Enable debug-level logging")

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usageText)
		return 2
	}

	// consume 'research' subcommand if provided
	remaining := args
	if args[0] == "research" {
		remaining = args[1:]
	}

	if err := fs.Parse(remaining); err != nil {
		if err == flag.ErrHelp {
			return 2
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	if strings.TrimSpace(topic) == "" {
		fmt.Fprintln(os.Stderr, "error: --topic is required")
		fs.Usage()
		return 2
	}

	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "error: unexpected arguments: %v (did you mean to quote --topic or use --out?)\n", fs.Args())
		return 2
	}

	switch format {
	case "json", "md", "both":
	default:
		fmt.Fprintf(os.Stderr, "error: --format must be json, md, or both (got %q)\n", format)
		return 2
	}

	// Setup observability
	obs.Setup(verbose)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		return 1
	}

	if verbose {
		slog.Debug("config loaded", "config", cfg.String())
	}

	// Confine and clean the output path. Relative paths must stay within the
	// working directory; absolute paths are honored as explicit operator intent.
	rawOut := outDir
	outDir = filepath.Clean(outDir)
	if !filepath.IsAbs(outDir) {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error resolving output directory: %v\n", err)
			return 1
		}
		abs := filepath.Join(cwd, outDir)
		rel, err := filepath.Rel(cwd, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			fmt.Fprintf(os.Stderr, "error: --out %q escapes the working directory; pass an absolute path for locations outside it\n", rawOut)
			return 2
		}
		outDir = abs
	}

	// Create output directory and enforce perms (MkdirAll is subject to umask).
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output directory %q: %v\n", outDir, err)
		return 1
	}
	if err := os.Chmod(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error setting output directory perms %q: %v\n", outDir, err)
		return 1
	}

	// Run the 4-phase STORM loop
	slog.Info("storm_start",
		"topic_hash", obs.TopicHash(topic),
		"role", role,
		"format", format,
		"key_status", obs.RedactKey(cfg.APIKey),
	)

	ctx := context.Background()
	b, err := storm.Run(ctx, cfg, topic, role)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: storm run failed: %v\n", err)
		slog.Error("storm_failed", "err", err.Error())
		return 1
	}

	slog.Info("storm_done", "findings", len(b.Findings), "topic_hash", obs.TopicHash(topic))

	// Write outputs per --format
	if format == "json" || format == "both" {
		jsonPath := filepath.Join(outDir, "briefing.json")
		data, err := briefing.RenderJSON(b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error rendering JSON: %v\n", err)
			return 1
		}
		if err := writeFile(jsonPath, data); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", jsonPath, err)
			return 1
		}
		slog.Info("wrote", "path", jsonPath)
	}

	if format == "md" || format == "both" {
		mdPath := filepath.Join(outDir, "briefing.md")
		md := briefing.RenderMarkdown(b)
		if err := writeFile(mdPath, []byte(md)); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", mdPath, err)
			return 1
		}
		slog.Info("wrote", "path", mdPath)
	}

	return 0
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

const usageText = `storm — STORM research CLI

Usage:
  storm research --topic "<topic>" [flags]
  storm --topic "<topic>" [flags]

Subcommands:
  research    Run the 4-phase STORM research loop (default)

Flags:`
