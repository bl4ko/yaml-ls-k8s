package proxy

import "sync"

// Tracker maps JSON-RPC request IDs to their methods.
// Used to identify responses to requests we initiated or need to intercept.
type Tracker struct {
	mu      sync.Mutex
	pending map[string]string // id (as raw JSON string) → method
}

func NewTracker() *Tracker {
	return &Tracker{
		pending: make(map[string]string),
	}
}

// Track records a request ID and its method.
func (t *Tracker) Track(id string, method string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending[id] = method
}

// Consume checks if an ID was tracked, returns the method and removes it.
func (t *Tracker) Consume(id string) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	method, ok := t.pending[id]
	if ok {
		delete(t.pending, id)
	}
	return method, ok
}
