package proxy

import (
	"encoding/json"

	"github.com/bl4ko/yaml-ls-k8s/internal/lsp"
)

// editorToServer reads messages from the editor, processes them, and forwards to yamlls.
func (p *Proxy) editorToServer() {
	for {
		data, err := lsp.ReadMessage(p.fromEditor)
		if err != nil {
			p.logger.Printf("editor read error (shutting down): %v", err)
			return
		}

		msg, err := lsp.ParseMessage(data)
		if err != nil {
			p.logger.Printf("failed to parse editor message: %v", err)
			p.sendToServer(data)
			continue
		}

		switch {
		case msg.IsResponse() && p.handleEditorResponse(msg, data):
			continue
		case msg.Method == "textDocument/didOpen":
			p.handleDidOpen(msg, data)
		case msg.Method == "textDocument/didChange":
			p.handleDidChange(msg, data)
		case msg.Method == "textDocument/didClose":
			p.handleDidClose(msg, data)
		default:
			p.sendToServer(data)
		}
	}
}

func (p *Proxy) handleDidOpen(msg *lsp.BaseMessage, data []byte) {
	var params lsp.DidOpenTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		p.logger.Printf("failed to parse didOpen params: %v", err)
		p.sendToServer(data)
		return
	}

	uri := params.TextDocument.URI
	content := params.TextDocument.Text
	p.state.SetContent(uri, content)
	p.logger.Printf("didOpen: %s (%d bytes)", uri, len(content))

	// Forward immediately, detect async
	p.sendToServer(data)
	go p.detectAndNotify(uri)
}

func (p *Proxy) handleDidChange(msg *lsp.BaseMessage, data []byte) {
	var params lsp.DidChangeTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		p.logger.Printf("failed to parse didChange params: %v", err)
		p.sendToServer(data)
		return
	}

	uri := params.TextDocument.URI
	for _, change := range params.ContentChanges {
		if change.Range == nil {
			// Full sync
			p.state.SetContent(uri, change.Text)
		} else {
			// Incremental sync
			p.state.ApplyChange(uri,
				change.Range.Start.Line, change.Range.Start.Character,
				change.Range.End.Line, change.Range.End.Character,
				change.Text,
			)
		}
	}

	// Forward immediately, detect async
	p.sendToServer(data)
	go p.detectAndNotify(uri)
}

func (p *Proxy) handleDidClose(msg *lsp.BaseMessage, data []byte) {
	var params lsp.DidCloseTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		p.logger.Printf("failed to parse didClose params: %v", err)
		p.sendToServer(data)
		return
	}

	p.state.DeleteContent(params.TextDocument.URI)
	p.logger.Printf("didClose: %s", params.TextDocument.URI)
	p.sendToServer(data)
}

// handleEditorResponse checks if a response from the editor matches a tracked request.
// Returns true if the message was intercepted and handled.
func (p *Proxy) handleEditorResponse(msg *lsp.BaseMessage, data []byte) bool {
	if msg.ID == nil {
		return false
	}
	idStr := string(*msg.ID)
	method, ok := p.tracker.Consume(idStr)
	if !ok {
		return false
	}

	if method == "workspace/configuration" {
		p.interceptConfigResponse(msg, data)
		return true
	}

	// Not a method we need to intercept, forward as-is
	p.sendToServer(data)
	return true
}

// interceptConfigResponse injects yaml.schemas into the workspace/configuration response.
func (p *Proxy) interceptConfigResponse(msg *lsp.BaseMessage, data []byte) {
	// The result is an array of settings objects (one per ConfigurationItem).
	var results []json.RawMessage
	if err := json.Unmarshal(msg.Result, &results); err != nil {
		p.logger.Printf("failed to parse config response result: %v", err)
		p.sendToServer(data)
		return
	}

	schemaMap := p.state.BuildSchemaMap()
	if len(schemaMap) == 0 {
		p.sendToServer(data)
		return
	}

	// Convert schema map: schema → list of URIs → schema → single glob or URI string
	yamlSchemas := make(map[string]interface{})
	for schema, uris := range schemaMap {
		if len(uris) == 1 {
			yamlSchemas[schema] = uris[0]
		} else {
			yamlSchemas[schema] = uris
		}
	}

	// Inject into each result object that has a "yaml" section
	for i, raw := range results {
		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err != nil {
			// If it's not an object, try to create one
			obj = make(map[string]interface{})
		}

		// Ensure yaml section exists
		yamlSection, ok := obj["yaml"].(map[string]interface{})
		if !ok {
			yamlSection = make(map[string]interface{})
		}
		yamlSection["schemas"] = yamlSchemas
		obj["yaml"] = yamlSection

		newRaw, err := json.Marshal(obj)
		if err != nil {
			p.logger.Printf("failed to marshal injected config: %v", err)
			continue
		}
		results[i] = newRaw
	}

	newResult, err := json.Marshal(results)
	if err != nil {
		p.logger.Printf("failed to marshal config response: %v", err)
		p.sendToServer(data)
		return
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(*msg.ID),
		"result":  json.RawMessage(newResult),
	}
	modified, err := json.Marshal(response)
	if err != nil {
		p.logger.Printf("failed to marshal modified response: %v", err)
		p.sendToServer(data)
		return
	}

	schemasJSON, _ := json.Marshal(yamlSchemas)
	p.logger.Printf("injected yaml.schemas: %s", schemasJSON)
	p.sendToServer(modified)
}
