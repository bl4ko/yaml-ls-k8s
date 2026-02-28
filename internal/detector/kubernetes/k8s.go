package kubernetes

import (
	"fmt"
	"strings"

	"github.com/bl4ko/yaml-ls-k8s/internal/detector"
)

const k8sSchemaBase = "https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master"

// K8sDetector detects core Kubernetes resources (groups WITHOUT dots).
type K8sDetector struct {
	Version string // e.g. "v1.33.0"
}

func NewK8sDetector(version string) *K8sDetector {
	return &K8sDetector{Version: version}
}

func (d *K8sDetector) Detect(content string) []detector.SchemaResult {
	if HasModeline(content) {
		return nil
	}

	metas := ExtractAllTypeMeta(content)
	var results []detector.SchemaResult
	seen := make(map[string]bool)

	for _, meta := range metas {
		if meta.HasDottedGroup() {
			continue // CRDs are handled by CRDDetector
		}
		url := d.buildURL(meta)
		if url != "" && !seen[url] {
			seen[url] = true
			results = append(results, detector.SchemaResult{URL: url})
		}
	}
	return results
}

func (d *K8sDetector) buildURL(meta TypeMeta) string {
	kind := strings.ToLower(meta.Kind)
	group, version := meta.ParseGroup()

	// Core API (apiVersion: v1) → deployment-v1.json
	// Named group (apiVersion: apps/v1) → deployment-apps-v1.json
	var suffix string
	if group == "" {
		suffix = fmt.Sprintf("%s-%s.json", kind, version)
	} else {
		suffix = fmt.Sprintf("%s-%s-%s.json", kind, group, version)
	}

	return fmt.Sprintf("%s/%s-standalone-strict/%s", k8sSchemaBase, d.Version, suffix)
}
