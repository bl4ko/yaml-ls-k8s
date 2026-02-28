package schema

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Downloader handles HTTP downloads with negative caching.
type Downloader struct {
	client       *http.Client
	negativeTTL  time.Duration
	negativeCache map[string]time.Time
	mu           sync.RWMutex
}

func NewDownloader(timeout, negativeTTL time.Duration) *Downloader {
	return &Downloader{
		client:        &http.Client{Timeout: timeout},
		negativeTTL:   negativeTTL,
		negativeCache: make(map[string]time.Time),
	}
}

// Download fetches a URL. Returns the body bytes, or an error.
// Returns ErrNotFound for 404s and caches the negative result.
func (d *Downloader) Download(url string) ([]byte, error) {
	if d.isNegativelyCached(url) {
		return nil, ErrNotFound
	}

	resp, err := d.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.cacheNegative(url)
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}

func (d *Downloader) isNegativelyCached(url string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	t, ok := d.negativeCache[url]
	if !ok {
		return false
	}
	if time.Since(t) > d.negativeTTL {
		return false
	}
	return true
}

func (d *Downloader) cacheNegative(url string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.negativeCache[url] = time.Now()
}

// ErrNotFound indicates a 404 response (possibly cached).
var ErrNotFound = fmt.Errorf("schema not found (404)")
