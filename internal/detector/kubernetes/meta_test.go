package kubernetes

import (
	"testing"
)

func TestExtractAllTypeMeta(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []TypeMeta
	}{
		{
			name: "single deployment",
			content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx`,
			want: []TypeMeta{{APIVersion: "apps/v1", Kind: "Deployment"}},
		},
		{
			name: "core v1 service",
			content: `apiVersion: v1
kind: Service
metadata:
  name: my-svc`,
			want: []TypeMeta{{APIVersion: "v1", Kind: "Service"}},
		},
		{
			name: "multi-document",
			content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
---
apiVersion: v1
kind: Service
metadata:
  name: my-svc`,
			want: []TypeMeta{
				{APIVersion: "apps/v1", Kind: "Deployment"},
				{APIVersion: "v1", Kind: "Service"},
			},
		},
		{
			name: "CRD with dotted group",
			content: `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-route`,
			want: []TypeMeta{{APIVersion: "gateway.networking.k8s.io/v1", Kind: "HTTPRoute"}},
		},
		{
			name: "empty document between separators",
			content: `---
apiVersion: v1
kind: ConfigMap
---
---
apiVersion: v1
kind: Secret`,
			want: []TypeMeta{
				{APIVersion: "v1", Kind: "ConfigMap"},
				{APIVersion: "v1", Kind: "Secret"},
			},
		},
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
		{
			name:    "comments only",
			content: "# just a comment\n# another comment",
			want:    nil,
		},
		{
			name: "quoted values",
			content: `apiVersion: "apps/v1"
kind: 'Deployment'`,
			want: []TypeMeta{{APIVersion: "apps/v1", Kind: "Deployment"}},
		},
		{
			name: "helm cleaned content (no templates)",
			content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: release-nginx
spec:
  replicas: 3`,
			want: []TypeMeta{{APIVersion: "apps/v1", Kind: "Deployment"}},
		},
		{
			name: "indented apiVersion is ignored",
			content: `kind: Deployment
spec:
  apiVersion: apps/v1
apiVersion: v1`,
			want: []TypeMeta{{APIVersion: "v1", Kind: "Deployment"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAllTypeMeta(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d metas, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("meta[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTypeMeta_ParseGroup(t *testing.T) {
	tests := []struct {
		apiVersion  string
		wantGroup   string
		wantVersion string
	}{
		{"v1", "", "v1"},
		{"apps/v1", "apps", "v1"},
		{"gateway.networking.k8s.io/v1", "gateway.networking.k8s.io", "v1"},
		{"cert-manager.io/v1", "cert-manager.io", "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.apiVersion, func(t *testing.T) {
			m := TypeMeta{APIVersion: tt.apiVersion}
			g, v := m.ParseGroup()
			if g != tt.wantGroup || v != tt.wantVersion {
				t.Errorf("ParseGroup() = (%q, %q), want (%q, %q)", g, v, tt.wantGroup, tt.wantVersion)
			}
		})
	}
}

func TestTypeMeta_HasDottedGroup(t *testing.T) {
	tests := []struct {
		apiVersion string
		want       bool
	}{
		{"v1", false},
		{"apps/v1", false},
		{"batch/v1", false},
		{"gateway.networking.k8s.io/v1", true},
		{"cert-manager.io/v1", true},
		{"argoproj.io/v1alpha1", true},
	}

	for _, tt := range tests {
		t.Run(tt.apiVersion, func(t *testing.T) {
			m := TypeMeta{APIVersion: tt.apiVersion}
			if got := m.HasDottedGroup(); got != tt.want {
				t.Errorf("HasDottedGroup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasModeline(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "with modeline",
			content: "# yaml-language-server: $schema=https://example.com/schema.json\napiVersion: v1\nkind: Pod",
			want:    true,
		},
		{
			name:    "without modeline",
			content: "apiVersion: v1\nkind: Pod",
			want:    false,
		},
		{
			name:    "modeline in middle",
			content: "apiVersion: v1\n# yaml-language-server: $schema=https://example.com\nkind: Pod",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasModeline(tt.content); got != tt.want {
				t.Errorf("HasModeline() = %v, want %v", got, tt.want)
			}
		})
	}
}
