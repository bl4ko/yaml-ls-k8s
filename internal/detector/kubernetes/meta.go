package kubernetes

import (
	"strings"
)

// TypeMeta holds the apiVersion and kind extracted from a YAML document.
type TypeMeta struct {
	APIVersion string
	Kind       string
}

// ParseGroup splits an apiVersion into group and version.
// "apps/v1" → ("apps", "v1"), "v1" → ("", "v1")
func (t TypeMeta) ParseGroup() (group, version string) {
	parts := strings.SplitN(t.APIVersion, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}

// HasDottedGroup returns true if the apiVersion group contains a dot (i.e., it's a CRD group).
func (t TypeMeta) HasDottedGroup() bool {
	group, _ := t.ParseGroup()
	return strings.Contains(group, ".")
}

// ExtractAllTypeMeta splits a YAML document by "---" separators and
// extracts apiVersion/kind from each sub-document.
func ExtractAllTypeMeta(content string) []TypeMeta {
	docs := splitYAMLDocuments(content)
	var metas []TypeMeta
	for _, doc := range docs {
		if m, ok := extractTypeMeta(doc); ok {
			metas = append(metas, m)
		}
	}
	return metas
}

func splitYAMLDocuments(content string) []string {
	var docs []string
	var current strings.Builder

	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "---" {
			if current.Len() > 0 {
				docs = append(docs, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if current.Len() > 0 {
		docs = append(docs, current.String())
	}
	return docs
}

func extractTypeMeta(doc string) (TypeMeta, bool) {
	var meta TypeMeta
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)

		// Skip comments, empty lines, and indented lines (nested fields)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Only look at top-level keys (no leading whitespace)
		if line != trimmed && strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}

		if strings.HasPrefix(trimmed, "apiVersion:") {
			meta.APIVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, "apiVersion:"))
			// Remove surrounding quotes if present
			meta.APIVersion = unquote(meta.APIVersion)
		} else if strings.HasPrefix(trimmed, "kind:") {
			meta.Kind = strings.TrimSpace(strings.TrimPrefix(trimmed, "kind:"))
			meta.Kind = unquote(meta.Kind)
		}

		if meta.APIVersion != "" && meta.Kind != "" {
			return meta, true
		}
	}
	return meta, false
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// HasModeline checks if the content has a yaml-language-server schema modeline.
func HasModeline(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# yaml-language-server: $schema=") {
			return true
		}
	}
	return false
}
