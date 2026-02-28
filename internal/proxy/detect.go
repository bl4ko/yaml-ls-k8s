package proxy

import (
	"errors"

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
		if changed := p.state.SetSchemas(uri, nil); changed {
			p.logger.Printf("cleared schemas for %s", uri)
			p.sendDidChangeConfiguration()
		}
		return
	}

	// Download/cache each schema (CRDs get ObjectMeta wrapping)
	multiDoc := len(results) > 1
	var fileURIs []string
	realSchemaCount := 0
	for _, r := range results {
		var fileURI string
		var err error
		if r.IsCRD {
			fileURI, err = deps.Cache.EnsureCRD(r.URL)
		} else {
			fileURI, err = deps.Cache.Ensure(r.URL)
		}
		if err != nil {
			p.logger.Printf("schema fetch failed for %s: %v", r.URL, err)
			if multiDoc && errors.Is(err, schema.ErrNotFound) {
				// Use permissive schema so yamlls doesn't apply a real schema to this doc
				permURI, permErr := deps.Cache.PermissiveSchemaURI()
				if permErr != nil {
					p.logger.Printf("permissive schema failed: %v", permErr)
					continue
				}
				fileURIs = append(fileURIs, permURI)
			}
			continue
		}
		realSchemaCount++
		fileURIs = append(fileURIs, fileURI)
	}

	// No real schemas found — don't assign a composite of only permissive schemas
	if realSchemaCount == 0 {
		return
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

	if changed := p.state.SetSchemas(uri, []string{schemaURI}); changed {
		p.logger.Printf("schema for %s → %s", uri, schemaURI)
		p.sendDidChangeConfiguration()
	}
}
