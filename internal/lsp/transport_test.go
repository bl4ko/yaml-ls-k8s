package lsp

import (
	"bufio"
	"bytes"
	"testing"
)

func TestReadWriteMessage(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"test"}`)

	// Write
	var buf bytes.Buffer
	if err := WriteMessage(&buf, body); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	// Read back
	reader := bufio.NewReader(&buf)
	got, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	if !bytes.Equal(got, body) {
		t.Errorf("got %q, want %q", got, body)
	}
}

func TestReadMessage_MissingContentLength(t *testing.T) {
	input := "Content-Type: application/json\r\n\r\n{}"
	reader := bufio.NewReader(bytes.NewReader([]byte(input)))
	_, err := ReadMessage(reader)
	if err == nil {
		t.Error("expected error for missing Content-Length")
	}
}

func TestParseMessage_Request(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	if !msg.IsRequest() {
		t.Error("expected IsRequest() == true")
	}
	if msg.IsNotification() {
		t.Error("expected IsNotification() == false")
	}
	if msg.IsResponse() {
		t.Error("expected IsResponse() == false")
	}
}

func TestParseMessage_Notification(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{}}`)
	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	if !msg.IsNotification() {
		t.Error("expected IsNotification() == true")
	}
}

func TestParseMessage_Response(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)
	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	if !msg.IsResponse() {
		t.Error("expected IsResponse() == true")
	}
}
