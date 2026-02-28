package proxy

import (
	"encoding/json"

	"github.com/bl4ko/yaml-ls-k8s/internal/lsp"
)

// serverToEditor reads messages from yamlls, processes them, and forwards to the editor.
func (p *Proxy) serverToEditor() {
	for {
		data, err := lsp.ReadMessage(p.fromServer)
		if err != nil {
			p.logger.Printf("server read error (shutting down): %v", err)
			return
		}

		msg, err := lsp.ParseMessage(data)
		if err != nil {
			p.logger.Printf("failed to parse server message: %v", err)
			p.sendToEditor(data)
			continue
		}

		switch {
		case msg.IsResponse() && msg.ID != nil:
			p.handleServerResponse(msg, data)
		case msg.Method == "workspace/configuration":
			p.handleConfigRequest(msg, data)
		default:
			p.sendToEditor(data)
		}
	}
}

// handleServerResponse intercepts the initialize response to force Full sync.
func (p *Proxy) handleServerResponse(msg *lsp.BaseMessage, data []byte) {
	// Try to detect initialize response by checking for capabilities.textDocumentSync
	if msg.Result == nil {
		p.sendToEditor(data)
		return
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(msg.Result, &result); err != nil {
		p.sendToEditor(data)
		return
	}

	capRaw, hasCaps := result["capabilities"]
	if !hasCaps {
		p.sendToEditor(data)
		return
	}

	var caps map[string]json.RawMessage
	if err := json.Unmarshal(capRaw, &caps); err != nil {
		p.sendToEditor(data)
		return
	}

	syncRaw, hasSync := caps["textDocumentSync"]
	if !hasSync {
		p.sendToEditor(data)
		return
	}

	modified := p.forceFullSync(syncRaw, caps, result, msg)
	if modified != nil {
		p.sendToEditor(modified)
		return
	}
	p.sendToEditor(data)
}

// forceFullSync modifies textDocumentSync to use Full (1) instead of Incremental (2).
func (p *Proxy) forceFullSync(syncRaw json.RawMessage, caps, result map[string]json.RawMessage, msg *lsp.BaseMessage) []byte {
	// Try as number first
	var syncKind int
	if err := json.Unmarshal(syncRaw, &syncKind); err == nil {
		if syncKind == 2 {
			p.logger.Printf("forcing textDocumentSync from Incremental(2) to Full(1)")
			caps["textDocumentSync"], _ = json.Marshal(1)
			return p.rebuildResponse(caps, result, msg)
		}
		return nil
	}

	// Try as object
	var syncOpts lsp.TextDocumentSyncOptions
	if err := json.Unmarshal(syncRaw, &syncOpts); err == nil {
		if syncOpts.Change == 2 {
			p.logger.Printf("forcing textDocumentSync.change from Incremental(2) to Full(1)")
			syncOpts.Change = 1
			caps["textDocumentSync"], _ = json.Marshal(syncOpts)
			return p.rebuildResponse(caps, result, msg)
		}
	}
	return nil
}

func (p *Proxy) rebuildResponse(caps, result map[string]json.RawMessage, msg *lsp.BaseMessage) []byte {
	result["capabilities"], _ = json.Marshal(caps)
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(*msg.ID),
		"result":  json.RawMessage(mustMarshal(result)),
	}
	data, _ := json.Marshal(response)
	return data
}

// handleConfigRequest tracks a workspace/configuration request from yamlls.
func (p *Proxy) handleConfigRequest(msg *lsp.BaseMessage, data []byte) {
	if msg.ID != nil {
		idStr := string(*msg.ID)
		p.tracker.Track(idStr, "workspace/configuration")
		p.logger.Printf("tracking workspace/configuration request id=%s params=%s", idStr, msg.Params)
	}
	p.sendToEditor(data)
}

// sendDidChangeConfiguration sends a workspace/didChangeConfiguration notification to yamlls.
func (p *Proxy) sendDidChangeConfiguration() {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "workspace/didChangeConfiguration",
		"params": map[string]interface{}{
			"settings": map[string]interface{}{},
		},
	}
	data, _ := json.Marshal(notification)
	p.sendToServer(data)
	p.logger.Printf("sent didChangeConfiguration to yamlls")
}

func mustMarshal(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
