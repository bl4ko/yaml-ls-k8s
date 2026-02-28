package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Cache manages disk-cached schemas.
type Cache struct {
	dir        string
	downloader *Downloader
	logger     *log.Logger
}

func NewCache(dir string, downloader *Downloader, logger *log.Logger) *Cache {
	return &Cache{
		dir:        dir,
		downloader: downloader,
		logger:     logger,
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

func pathToFileURI(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	return "file://" + absPath
}
