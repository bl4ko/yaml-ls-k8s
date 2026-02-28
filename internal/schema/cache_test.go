package schema

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPermissiveSchemaURI(t *testing.T) {
	dir := t.TempDir()
	logger := log.New(os.Stderr, "test: ", 0)
	c := NewCache(dir, nil, logger, "v1.33.0")

	uri, err := c.PermissiveSchemaURI()
	if err != nil {
		t.Fatalf("PermissiveSchemaURI() error: %v", err)
	}

	if !strings.HasPrefix(uri, "file://") {
		t.Fatalf("expected file:// URI, got: %s", uri)
	}

	// Verify file content
	path := strings.TrimPrefix(uri, "file://")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading permissive schema: %v", err)
	}
	if string(data) != "{}" {
		t.Fatalf("expected '{}', got: %s", string(data))
	}

	// Verify path is under _internal/
	rel, _ := filepath.Rel(dir, path)
	if !strings.HasPrefix(rel, "_internal") {
		t.Fatalf("expected path under _internal/, got: %s", rel)
	}

	// Verify idempotent — second call returns same URI
	uri2, err := c.PermissiveSchemaURI()
	if err != nil {
		t.Fatalf("second PermissiveSchemaURI() error: %v", err)
	}
	if uri != uri2 {
		t.Fatalf("expected same URI, got %s vs %s", uri, uri2)
	}
}
