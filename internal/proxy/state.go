package proxy

import "sync"

// State stores document content and per-URI schema mappings (thread-safe).
type State struct {
	mu       sync.RWMutex
	contents map[string]string   // uri → full document text
	schemas  map[string][]string // uri → list of schema file:// URIs
}

func NewState() *State {
	return &State{
		contents: make(map[string]string),
		schemas:  make(map[string][]string),
	}
}

func (s *State) SetContent(uri, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contents[uri] = content
}

func (s *State) GetContent(uri string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.contents[uri]
	return c, ok
}

func (s *State) DeleteContent(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.contents, uri)
	delete(s.schemas, uri)
}

func (s *State) SetSchemas(uri string, schemas []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schemas[uri] = schemas
}

// BuildSchemaMap returns the aggregated yaml.schemas map: schema → [uri, ...].
func (s *State) BuildSchemaMap() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string][]string)
	for uri, schemas := range s.schemas {
		for _, schema := range schemas {
			result[schema] = append(result[schema], uri)
		}
	}
	return result
}

// ApplyChange applies an incremental text edit to the stored content.
func (s *State) ApplyChange(uri string, startLine, startChar, endLine, endChar int, newText string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	content, ok := s.contents[uri]
	if !ok {
		return
	}

	lines := splitLines(content)
	startOffset := lineCharToOffset(lines, startLine, startChar)
	endOffset := lineCharToOffset(lines, endLine, endChar)

	if startOffset > len(content) {
		startOffset = len(content)
	}
	if endOffset > len(content) {
		endOffset = len(content)
	}
	if startOffset > endOffset {
		startOffset = endOffset
	}

	s.contents[uri] = content[:startOffset] + newText + content[endOffset:]
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start <= len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func lineCharToOffset(lines []string, line, char int) int {
	offset := 0
	for i := 0; i < line && i < len(lines); i++ {
		offset += len(lines[i])
	}
	if line < len(lines) {
		if char > len(lines[line]) {
			char = len(lines[line])
		}
		offset += char
	}
	return offset
}
