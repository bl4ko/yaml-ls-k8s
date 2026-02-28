package kubernetes

import (
	"testing"
)

func TestK8sDetector_Detect(t *testing.T) {
	d := NewK8sDetector("v1.33.0")

	tests := []struct {
		name     string
		content  string
		wantURLs []string
	}{
		{
			name: "deployment",
			content: `apiVersion: apps/v1
kind: Deployment`,
			wantURLs: []string{
				"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/v1.33.0-standalone-strict/deployment-apps-v1.json",
			},
		},
		{
			name: "core v1 pod",
			content: `apiVersion: v1
kind: Pod`,
			wantURLs: []string{
				"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/v1.33.0-standalone-strict/pod-v1.json",
			},
		},
		{
			name: "core v1 service",
			content: `apiVersion: v1
kind: Service`,
			wantURLs: []string{
				"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/v1.33.0-standalone-strict/service-v1.json",
			},
		},
		{
			name: "CRD is skipped",
			content: `apiVersion: cert-manager.io/v1
kind: Certificate`,
			wantURLs: nil,
		},
		{
			name: "modeline bypasses detection",
			content: `# yaml-language-server: $schema=https://example.com/schema.json
apiVersion: apps/v1
kind: Deployment`,
			wantURLs: nil,
		},
		{
			name: "multi-doc with core resources only",
			content: `apiVersion: apps/v1
kind: Deployment
---
apiVersion: v1
kind: Service`,
			wantURLs: []string{
				"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/v1.33.0-standalone-strict/deployment-apps-v1.json",
				"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/v1.33.0-standalone-strict/service-v1.json",
			},
		},
		{
			name: "batch cronjob",
			content: `apiVersion: batch/v1
kind: CronJob`,
			wantURLs: []string{
				"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/v1.33.0-standalone-strict/cronjob-batch-v1.json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := d.Detect(tt.content)
			if len(results) != len(tt.wantURLs) {
				t.Fatalf("got %d results, want %d", len(results), len(tt.wantURLs))
			}
			for i, r := range results {
				if r.URL != tt.wantURLs[i] {
					t.Errorf("result[%d].URL = %s, want %s", i, r.URL, tt.wantURLs[i])
				}
			}
		})
	}
}
