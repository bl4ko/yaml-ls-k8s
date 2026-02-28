package detector

// SchemaResult represents a detected schema for a document.
type SchemaResult struct {
	URL   string // Remote URL for the schema
	IsCRD bool   // True if this is a CRD schema (needs ObjectMeta wrapping)
}

// Detector detects schemas from YAML document content.
type Detector interface {
	Detect(content string) []SchemaResult
}

// Chain runs multiple detectors and aggregates results.
type Chain struct {
	detectors []Detector
}

func NewChain(detectors ...Detector) *Chain {
	return &Chain{detectors: detectors}
}

func (c *Chain) Detect(content string) []SchemaResult {
	var results []SchemaResult
	seen := make(map[string]bool)
	for _, d := range c.detectors {
		for _, r := range d.Detect(content) {
			if !seen[r.URL] {
				seen[r.URL] = true
				results = append(results, r)
			}
		}
	}
	return results
}
