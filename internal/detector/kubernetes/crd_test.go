package kubernetes

import (
	"testing"
)

func TestCRDDetector_Detect(t *testing.T) {
	d := NewCRDDetector()

	tests := []struct {
		name     string
		content  string
		wantURLs []string
	}{
		{
			name: "cert-manager certificate",
			content: `apiVersion: cert-manager.io/v1
kind: Certificate`,
			wantURLs: []string{
				"https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/cert-manager.io/certificate_v1.json",
			},
		},
		{
			name: "gateway API HTTPRoute",
			content: `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute`,
			wantURLs: []string{
				"https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/gateway.networking.k8s.io/httproute_v1.json",
			},
		},
		{
			name: "ArgoCD Application",
			content: `apiVersion: argoproj.io/v1alpha1
kind: Application`,
			wantURLs: []string{
				"https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/argoproj.io/application_v1alpha1.json",
			},
		},
		{
			name: "core K8s is skipped",
			content: `apiVersion: apps/v1
kind: Deployment`,
			wantURLs: nil,
		},
		{
			name: "modeline bypasses detection",
			content: `# yaml-language-server: $schema=https://example.com/schema.json
apiVersion: cert-manager.io/v1
kind: Certificate`,
			wantURLs: nil,
		},
		{
			name: "k8s.io extension group (gateway)",
			content: `apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway`,
			wantURLs: []string{
				"https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/gateway.networking.k8s.io/gateway_v1beta1.json",
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
				if !r.IsCRD {
					t.Errorf("result[%d].IsCRD = false, want true", i)
				}
			}
		})
	}
}
