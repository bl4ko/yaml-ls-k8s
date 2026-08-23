package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	YamlLSPath string
	LogFile    string
	K8sVersion string
	CacheDir   string
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		YamlLSPath: "yaml-language-server",
		LogFile:    filepath.Join(home, ".config", "yaml-ls-k8s", "server.log"),
		// renovate: datasource=github-releases depName=kubernetes/kubernetes
		K8sVersion: "v1.36.4",
		CacheDir:   filepath.Join(home, ".cache", "yaml-ls-k8s", "schemas"),
	}
}
