package proxy

import "testing"

func TestTracker(t *testing.T) {
	tr := NewTracker()

	tr.Track("1", "workspace/configuration")
	tr.Track(`"abc"`, "window/showMessageRequest")

	// Consume existing
	method, ok := tr.Consume("1")
	if !ok || method != "workspace/configuration" {
		t.Errorf("Consume(1) = (%q, %v), want (workspace/configuration, true)", method, ok)
	}

	// Consume again should fail (already consumed)
	_, ok = tr.Consume("1")
	if ok {
		t.Error("Consume(1) second time should return false")
	}

	// Consume other
	method, ok = tr.Consume(`"abc"`)
	if !ok || method != "window/showMessageRequest" {
		t.Errorf("Consume(abc) = (%q, %v), want (window/showMessageRequest, true)", method, ok)
	}

	// Unknown ID
	_, ok = tr.Consume("999")
	if ok {
		t.Error("Consume(999) should return false for unknown ID")
	}
}
