package proxy

import "testing"

func TestState_ContentCRUD(t *testing.T) {
	s := NewState()

	// Set and get
	s.SetContent("file:///test.yaml", "hello: world")
	content, ok := s.GetContent("file:///test.yaml")
	if !ok || content != "hello: world" {
		t.Errorf("GetContent = (%q, %v), want (hello: world, true)", content, ok)
	}

	// Get non-existent
	_, ok = s.GetContent("file:///nonexistent.yaml")
	if ok {
		t.Error("GetContent for non-existent should return false")
	}

	// Delete
	s.DeleteContent("file:///test.yaml")
	_, ok = s.GetContent("file:///test.yaml")
	if ok {
		t.Error("GetContent after delete should return false")
	}
}

func TestState_ApplyChange(t *testing.T) {
	s := NewState()
	uri := "file:///test.yaml"

	s.SetContent(uri, "line0\nline1\nline2\n")

	// Replace "line1" with "replaced"
	s.ApplyChange(uri, 1, 0, 1, 5, "replaced")

	content, _ := s.GetContent(uri)
	expected := "line0\nreplaced\nline2\n"
	if content != expected {
		t.Errorf("after ApplyChange:\ngot:  %q\nwant: %q", content, expected)
	}
}

func TestState_ApplyChange_Insert(t *testing.T) {
	s := NewState()
	uri := "file:///test.yaml"

	s.SetContent(uri, "ab")

	// Insert "XY" at position (0,1) to (0,1) — between a and b
	s.ApplyChange(uri, 0, 1, 0, 1, "XY")

	content, _ := s.GetContent(uri)
	if content != "aXYb" {
		t.Errorf("after insert: got %q, want %q", content, "aXYb")
	}
}

func TestState_BuildSchemaMap(t *testing.T) {
	s := NewState()

	s.SetSchemas("file:///a.yaml", []string{"file:///schema/deploy.json"})
	s.SetSchemas("file:///b.yaml", []string{"file:///schema/deploy.json", "file:///schema/svc.json"})

	m := s.BuildSchemaMap()

	deployURIs := m["file:///schema/deploy.json"]
	if len(deployURIs) != 2 {
		t.Errorf("deploy schema should map to 2 URIs, got %d", len(deployURIs))
	}

	svcURIs := m["file:///schema/svc.json"]
	if len(svcURIs) != 1 {
		t.Errorf("svc schema should map to 1 URI, got %d", len(svcURIs))
	}
}
