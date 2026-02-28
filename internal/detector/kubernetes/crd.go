package kubernetes

import (
	"fmt"
	"strings"

	"github.com/bl4ko/yaml-ls-k8s/internal/detector"
)

const crdSchemaBase = "https://raw.githubusercontent.com/datreeio/CRDs-catalog/main"

// CRDDetector detects CRD resources (groups WITH dots).
type CRDDetector struct{}

func NewCRDDetector() *CRDDetector {
	return &CRDDetector{}
}

func (d *CRDDetector) Detect(content string) []detector.SchemaResult {
	if HasModeline(content) {
		return nil
	}

	metas := ExtractAllTypeMeta(content)
	var results []detector.SchemaResult
	seen := make(map[string]bool)

	for _, meta := range metas {
		if !meta.HasDottedGroup() {
			continue // Core K8s handled by K8sDetector
		}
		url := d.buildURL(meta)
		if url != "" && !seen[url] {
			seen[url] = true
			results = append(results, detector.SchemaResult{URL: url})
		}
	}
	return results
}

func (d *CRDDetector) buildURL(meta TypeMeta) string {
	group, _ := meta.ParseGroup()
	kind := strings.ToLower(meta.Kind)

	// datreeio format: {group}/{kind}_{version}.json
	_, version := meta.ParseGroup()
	return fmt.Sprintf("%s/%s/%s_%s.json", crdSchemaBase, group, kind, version)
}
