package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Cache manages disk-cached schemas.
type Cache struct {
	dir        string
	downloader *Downloader
	logger     *log.Logger
	k8sVersion string

	objectMetaOnce sync.Once
	objectMeta     json.RawMessage // cached ObjectMeta schema extracted from core K8s

	permissiveOnce sync.Once
	permissiveURI  string // cached file:// URI for the permissive ({}) schema
}

func NewCache(dir string, downloader *Downloader, logger *log.Logger, k8sVersion string) *Cache {
	return &Cache{
		dir:        dir,
		downloader: downloader,
		logger:     logger,
		k8sVersion: k8sVersion,
	}
}

// Ensure downloads the schema at the given URL if not already cached.
// Returns a file:// URI pointing to the cached file.
func (c *Cache) Ensure(schemaURL string) (string, error) {
	localPath := c.urlToPath(schemaURL)

	// Already cached on disk
	if _, err := os.Stat(localPath); err == nil {
		return pathToFileURI(localPath), nil
	}

	// Download
	c.logger.Printf("downloading schema: %s", schemaURL)
	data, err := c.downloader.Download(schemaURL)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.logger.Printf("schema not found (404): %s", schemaURL)
		}
		return "", err
	}

	// Write to disk
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating cache dir: %w", err)
	}
	if err := os.WriteFile(localPath, data, 0o644); err != nil {
		return "", fmt.Errorf("writing cache file: %w", err)
	}

	c.logger.Printf("cached schema: %s → %s", schemaURL, localPath)
	return pathToFileURI(localPath), nil
}

// urlToPath converts a URL to a local cache file path using its path structure.
func (c *Cache) urlToPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		hash := sha256.Sum256([]byte(rawURL))
		return filepath.Join(c.dir, hex.EncodeToString(hash[:])+".json")
	}

	// Use the URL path as the cache path, stripping the leading slash
	path := strings.TrimPrefix(parsed.Path, "/")
	return filepath.Join(c.dir, parsed.Host, path)
}

// EnsureCRD downloads a CRD schema and wraps it with a full ObjectMeta definition
// so that metadata fields (annotations, labels, name, namespace, etc.) get autocompletion.
func (c *Cache) EnsureCRD(schemaURL string) (string, error) {
	// First ensure the raw CRD schema is cached
	rawURI, err := c.Ensure(schemaURL)
	if err != nil {
		return "", err
	}

	// Build a wrapper schema path
	rawPath := strings.TrimPrefix(rawURI, "file://")
	wrapperPath := strings.TrimSuffix(rawPath, ".json") + "_wrapped.json"

	// Already wrapped?
	if _, err := os.Stat(wrapperPath); err == nil {
		return pathToFileURI(wrapperPath), nil
	}

	// Get ObjectMeta schema
	objectMeta, err := c.getObjectMeta()
	if err != nil {
		c.logger.Printf("failed to get ObjectMeta schema, using raw CRD: %v", err)
		return rawURI, nil
	}

	// Read original CRD schema
	rawData, err := os.ReadFile(rawPath)
	if err != nil {
		return rawURI, nil
	}

	var crdSchema map[string]interface{}
	if err := json.Unmarshal(rawData, &crdSchema); err != nil {
		return rawURI, nil
	}

	// Replace the minimal metadata with the full ObjectMeta
	props, ok := crdSchema["properties"].(map[string]interface{})
	if !ok {
		return rawURI, nil
	}

	var metaObj interface{}
	if err := json.Unmarshal(objectMeta, &metaObj); err != nil {
		return rawURI, nil
	}
	props["metadata"] = metaObj

	wrapped, err := json.Marshal(crdSchema)
	if err != nil {
		return rawURI, nil
	}

	if err := os.WriteFile(wrapperPath, wrapped, 0o644); err != nil {
		return rawURI, nil
	}

	c.logger.Printf("created CRD wrapper with ObjectMeta: %s", wrapperPath)
	return pathToFileURI(wrapperPath), nil
}

// getObjectMeta fetches and caches the ObjectMeta schema from a core K8s resource.
func (c *Cache) getObjectMeta() (json.RawMessage, error) {
	var retErr error
	c.objectMetaOnce.Do(func() {
		// Download pod-v1 schema and extract metadata
		podURL := fmt.Sprintf(
			"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/%s-standalone-strict/pod-v1.json",
			c.k8sVersion,
		)
		podURI, err := c.Ensure(podURL)
		if err != nil {
			retErr = fmt.Errorf("downloading pod schema for ObjectMeta: %w", err)
			return
		}

		podPath := strings.TrimPrefix(podURI, "file://")
		data, err := os.ReadFile(podPath)
		if err != nil {
			retErr = fmt.Errorf("reading pod schema: %w", err)
			return
		}

		var podSchema struct {
			Properties struct {
				Metadata json.RawMessage `json:"metadata"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(data, &podSchema); err != nil {
			retErr = fmt.Errorf("parsing pod schema: %w", err)
			return
		}

		c.objectMeta = podSchema.Properties.Metadata
		c.logger.Printf("extracted ObjectMeta schema (%d bytes)", len(c.objectMeta))
	})

	if retErr != nil {
		return nil, retErr
	}
	if c.objectMeta == nil {
		return nil, fmt.Errorf("ObjectMeta not available")
	}
	return c.objectMeta, nil
}

// PermissiveSchemaURI returns a file:// URI to a permissive JSON schema ({})
// that accepts any document. Created once and reused for all subsequent calls.
func (c *Cache) PermissiveSchemaURI() (string, error) {
	var retErr error
	c.permissiveOnce.Do(func() {
		dir := filepath.Join(c.dir, "_internal")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			retErr = fmt.Errorf("creating internal schema dir: %w", err)
			return
		}
		p := filepath.Join(dir, "permissive.json")
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			retErr = fmt.Errorf("writing permissive schema: %w", err)
			return
		}
		c.permissiveURI = pathToFileURI(p)
		c.logger.Printf("created permissive schema: %s", p)
	})
	if retErr != nil {
		return "", retErr
	}
	if c.permissiveURI == "" {
		return "", fmt.Errorf("permissive schema not available")
	}
	return c.permissiveURI, nil
}

func pathToFileURI(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	return "file://" + absPath
}
