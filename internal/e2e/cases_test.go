package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestE2E_CoreK8s_Deployment(t *testing.T) {
	requireYamlLS(t)
	binary := findBinary(t)
	cacheDir := t.TempDir()

	c := newLSPClient(t, binary, cacheDir)
	defer c.close()
	c.initialize()

	uri := "file:///tmp/e2e_deployment.yaml"
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 3
  invalidField: true
`
	c.openFile(uri, content)
	time.Sleep(5 * time.Second)

	// Verify schema was cached
	schemaPath := filepath.Join(cacheDir, "raw.githubusercontent.com", "yannh", "kubernetes-json-schema", "master")
	matches, _ := filepath.Glob(filepath.Join(schemaPath, "*", "deployment-apps-v1.json"))
	if len(matches) == 0 {
		t.Fatal("deployment schema not cached")
	}

	// Verify diagnostics: invalidField should be flagged
	diags := c.getDiagnostics(uri)
	found := false
	for _, d := range diags {
		if d.Message == "Property invalidField is not allowed." {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Property invalidField is not allowed.' diagnostic, got %d diagnostics: %+v", len(diags), diags)
	}

	// Verify completions at metadata level
	labels := c.completion(uri, 3, 2)
	if !contains(labels, "name") {
		t.Errorf("expected 'name' in metadata completions, got: %v", labels)
	}
}

func TestE2E_CRD_HTTPRoute(t *testing.T) {
	requireYamlLS(t)
	binary := findBinary(t)
	cacheDir := t.TempDir()

	c := newLSPClient(t, binary, cacheDir)
	defer c.close()
	c.initialize()

	uri := "file:///tmp/e2e_httproute.yaml"
	content := `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:

spec:
  parentRefs:
    - name: gw
  invalidField: true
`
	c.openFile(uri, content)
	time.Sleep(5 * time.Second)

	// Verify CRD schema was cached and wrapped with ObjectMeta
	wrappedGlob := filepath.Join(cacheDir, "raw.githubusercontent.com", "datreeio", "CRDs-catalog", "main", "gateway.networking.k8s.io", "httproute_v1_wrapped.json")
	if _, err := os.Stat(wrappedGlob); err != nil {
		t.Fatal("CRD wrapped schema not cached")
	}

	// Verify diagnostics
	diags := c.getDiagnostics(uri)
	found := false
	for _, d := range diags {
		if d.Message == "Property invalidField is not allowed." {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Property invalidField is not allowed.' diagnostic, got %d diagnostics: %+v", len(diags), diags)
	}

	// Verify metadata completions (ObjectMeta wrapping)
	labels := c.completion(uri, 3, 2)
	for _, want := range []string{"annotations", "labels", "name", "namespace"} {
		if !contains(labels, want) {
			t.Errorf("expected %q in metadata completions, got: %v", want, labels)
		}
	}
}

func TestE2E_CRD_ExternalSecrets(t *testing.T) {
	requireYamlLS(t)
	binary := findBinary(t)
	cacheDir := t.TempDir()

	c := newLSPClient(t, binary, cacheDir)
	defer c.close()
	c.initialize()

	uri := "file:///tmp/e2e_secretstore.yaml"
	content := `apiVersion: external-secrets.io/v1
kind: SecretStore
metadata:
  name: secretstore-sample
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-east-1
`
	c.openFile(uri, content)
	time.Sleep(5 * time.Second)

	// Verify provider completions
	labels := c.completion(uri, 6, 4)
	if len(labels) == 0 {
		t.Fatal("expected provider completions, got none")
	}
	if !contains(labels, "aws") {
		t.Errorf("expected 'aws' in provider completions, got: %v", labels)
	}
}

func TestE2E_Modeline_Bypass(t *testing.T) {
	requireYamlLS(t)
	binary := findBinary(t)
	cacheDir := t.TempDir()

	c := newLSPClient(t, binary, cacheDir)
	defer c.close()
	c.initialize()

	uri := "file:///tmp/e2e_modeline.yaml"
	content := `# yaml-language-server: $schema=https://json.schemastore.org/github-workflow
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
`
	c.openFile(uri, content)
	time.Sleep(3 * time.Second)

	// With modeline, our detector should skip — no K8s schema injected.
	// The schema store modeline should take precedence.
	wrappedGlob := filepath.Join(cacheDir, "raw.githubusercontent.com", "yannh", "kubernetes-json-schema", "**", "deployment-apps-v1.json")
	matches, _ := filepath.Glob(wrappedGlob)
	if len(matches) > 0 {
		t.Error("K8s schema should NOT be downloaded when modeline is present")
	}
}
