package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/bl4ko/yaml-ls-k8s/internal/config"
	"github.com/bl4ko/yaml-ls-k8s/internal/detector"
	"github.com/bl4ko/yaml-ls-k8s/internal/detector/kubernetes"
	"github.com/bl4ko/yaml-ls-k8s/internal/proxy"
	"github.com/bl4ko/yaml-ls-k8s/internal/schema"
)

var version = "0.1.0"

func main() {
	cfg := config.DefaultConfig()

	showVersion := flag.Bool("version", false, "print version and exit")
	flag.StringVar(&cfg.YamlLSPath, "yamlls-path", cfg.YamlLSPath, "path to yaml-language-server binary")
	flag.StringVar(&cfg.LogFile, "log-file", cfg.LogFile, "log file path")
	flag.StringVar(&cfg.K8sVersion, "k8s-version", cfg.K8sVersion, "Kubernetes schema version")
	flag.StringVar(&cfg.CacheDir, "cache-dir", cfg.CacheDir, "schema cache directory")
	stdio := flag.Bool("stdio", true, "use stdio transport (ignored, for LSP client compat)")
	flag.Parse()

	if *showVersion {
		fmt.Println("yaml-ls-k8s " + version)
		os.Exit(0)
	}

	_ = stdio // always stdio

	// Set up logging
	if err := os.MkdirAll(filepath.Dir(cfg.LogFile), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log dir: %v\n", err)
		os.Exit(1)
	}
	logFile, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	logger := log.New(logFile, "[yaml-ls-k8s] ", log.LstdFlags|log.Lmicroseconds)
	logger.Printf("starting yaml-ls-k8s (yamlls=%s, k8s=%s, cache=%s)", cfg.YamlLSPath, cfg.K8sVersion, cfg.CacheDir)

	// Set up detection chain
	downloader := schema.NewDownloader(30*time.Second, 1*time.Hour)
	cache := schema.NewCache(cfg.CacheDir, downloader, logger, cfg.K8sVersion)
	composite := schema.NewCompositeBuilder(cfg.CacheDir, logger)
	chain := detector.NewChain(
		kubernetes.NewK8sDetector(cfg.K8sVersion),
		kubernetes.NewCRDDetector(),
	)

	proxy.SetDetectDeps(&proxy.DetectDeps{
		Chain:     chain,
		Cache:     cache,
		Composite: composite,
	})

	// Run proxy
	p := proxy.New(cfg, logger)
	if err := p.Run(); err != nil {
		logger.Printf("proxy exited: %v", err)
		os.Exit(1)
	}
}
