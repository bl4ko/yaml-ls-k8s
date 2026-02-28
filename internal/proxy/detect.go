package proxy

import (
	"github.com/bl4ko/yaml-ls-k8s/internal/detector"
	"github.com/bl4ko/yaml-ls-k8s/internal/schema"
)

// DetectDeps holds the dependencies for schema detection (injected from main).
type DetectDeps struct {
	Chain     *detector.Chain
	Cache     *schema.Cache
	Composite *schema.CompositeBuilder
}

var deps *DetectDeps

// SetDetectDeps sets the detection dependencies (called once from main).
func SetDetectDeps(d *DetectDeps) {
	deps = d
}

// detectAndNotify detects schemas for a URI's content, caches them, and notifies yamlls.
func (p *Proxy) detectAndNotify(uri string) {
	if deps == nil {
		return
	}

	content, ok := p.state.GetContent(uri)
	if !ok {
		return
	}

	results := deps.Chain.Detect(content)
	if len(results) == 0 {
		// Clear any previous schemas for this URI
		p.state.SetSchemas(uri, nil)
		p.sendDidChangeConfiguration()
		return
	}

	// Download/cache each schema
	var fileURIs []string
	for _, r := range results {
		fileURI, err := deps.Cache.Ensure(r.URL)
		if err != nil {
			p.logger.Printf("schema fetch failed for %s: %v", r.URL, err)
			continue
		}
		fileURIs = append(fileURIs, fileURI)
	}

	if len(fileURIs) == 0 {
		return
	}

	// Build composite if multiple schemas
	schemaURI, err := deps.Composite.Build(fileURIs)
	if err != nil {
		p.logger.Printf("composite schema build failed: %v", err)
		return
	}

	p.state.SetSchemas(uri, []string{schemaURI})
	p.sendDidChangeConfiguration()
}
