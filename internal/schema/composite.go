package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CompositeBuilder creates anyOf composite schemas for multi-document YAML files.
type CompositeBuilder struct {
	dir    string
	logger *log.Logger
}

func NewCompositeBuilder(dir string, logger *log.Logger) *CompositeBuilder {
	return &CompositeBuilder{dir: dir, logger: logger}
}

// Build creates an anyOf composite schema from multiple schema file URIs.
// Returns a file:// URI to the composite schema.
// If there's only one schema, returns it directly.
func (b *CompositeBuilder) Build(fileURIs []string) (string, error) {
	if len(fileURIs) == 1 {
		return fileURIs[0], nil
	}

	// Deduplicate and sort for deterministic hashing
	unique := dedupStrings(fileURIs)
	sort.Strings(unique)

	// Generate deterministic filename from schema set
	key := strings.Join(unique, "\n")
	hash := sha256.Sum256([]byte(key))
	filename := "composite_" + hex.EncodeToString(hash[:8]) + ".json"
	localPath := filepath.Join(b.dir, "_composites", filename)

	// Check if already exists
	if _, err := os.Stat(localPath); err == nil {
		return pathToFileURI(localPath), nil
	}

	// Build anyOf schema
	var refs []map[string]string
	for _, uri := range unique {
		path := strings.TrimPrefix(uri, "file://")
		refs = append(refs, map[string]string{"$ref": path})
	}

	composite := map[string]interface{}{
		"anyOf": refs,
	}

	data, err := json.MarshalIndent(composite, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling composite schema: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return "", fmt.Errorf("creating composite dir: %w", err)
	}
	if err := os.WriteFile(localPath, data, 0o644); err != nil {
		return "", fmt.Errorf("writing composite schema: %w", err)
	}

	b.logger.Printf("created composite schema: %s (%d schemas)", localPath, len(unique))
	return pathToFileURI(localPath), nil
}

func dedupStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
