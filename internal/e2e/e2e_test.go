package e2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// lspClient is a minimal LSP client for e2e testing.
type lspClient struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader

	mu          sync.Mutex
	nextID      int
	diagnostics map[string][]diagnostic
	responses   map[int]chan json.RawMessage

	configHandler func(id json.RawMessage) // responds to workspace/configuration
}

type diagnostic struct {
	Severity int    `json:"severity"`
	Message  string `json:"message"`
}

func newLSPClient(t *testing.T, binary string, cacheDir string) *lspClient {
	t.Helper()

	cmd := exec.Command(binary,
		"--cache-dir", cacheDir,
		"--log-file", filepath.Join(cacheDir, "test.log"),
	)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	c := &lspClient{
		t:           t,
		cmd:         cmd,
		stdin:       stdin,
		reader:      bufio.NewReader(stdout),
		diagnostics: make(map[string][]diagnostic),
		responses:   make(map[int]chan json.RawMessage),
	}

	c.configHandler = func(id json.RawMessage) {
		c.sendRaw(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  []interface{}{map[string]interface{}{"schemaStore": map[string]interface{}{"enable": false}, "schemas": map[string]interface{}{}, "validate": true}},
		})
	}

	go c.readLoop()
	return c
}

func (c *lspClient) close() {
	c.stdin.Close()
	c.cmd.Process.Kill()
	c.cmd.Wait()
}

func (c *lspClient) sendRaw(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.t.Fatalf("marshal: %v", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stdin.Write([]byte(header))
	c.stdin.Write(data)
}

func (c *lspClient) send(method string, params interface{}) int {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.responses[id] = make(chan json.RawMessage, 1)
	c.mu.Unlock()

	c.sendRaw(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	return id
}

func (c *lspClient) notify(method string, params interface{}) {
	c.sendRaw(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

func (c *lspClient) waitResponse(id int, timeout time.Duration) json.RawMessage {
	c.mu.Lock()
	ch := c.responses[id]
	c.mu.Unlock()

	select {
	case result := <-ch:
		return result
	case <-time.After(timeout):
		c.t.Fatalf("timeout waiting for response id=%d", id)
		return nil
	}
}

func (c *lspClient) readLoop() {
	for {
		msg, err := readMessage(c.reader)
		if err != nil {
			return
		}

		var base struct {
			ID     *json.RawMessage `json:"id,omitempty"`
			Method string           `json:"method,omitempty"`
			Params json.RawMessage  `json:"params,omitempty"`
			Result json.RawMessage  `json:"result,omitempty"`
		}
		if err := json.Unmarshal(msg, &base); err != nil {
			continue
		}

		// Handle workspace/configuration from yamlls
		if base.Method == "workspace/configuration" && base.ID != nil {
			c.configHandler(*base.ID)
			continue
		}

		// Collect diagnostics
		if base.Method == "textDocument/publishDiagnostics" {
			var params struct {
				URI         string       `json:"uri"`
				Diagnostics []diagnostic `json:"diagnostics"`
			}
			if err := json.Unmarshal(base.Params, &params); err == nil {
				c.mu.Lock()
				c.diagnostics[params.URI] = params.Diagnostics
				c.mu.Unlock()
			}
			continue
		}

		// Dispatch responses
		if base.ID != nil && base.Method == "" {
			var id int
			if err := json.Unmarshal(*base.ID, &id); err == nil {
				c.mu.Lock()
				ch, ok := c.responses[id]
				c.mu.Unlock()
				if ok {
					ch <- base.Result
				}
			}
		}
	}
}

func (c *lspClient) getDiagnostics(uri string) []diagnostic {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.diagnostics[uri]
}

func (c *lspClient) initialize() {
	id := c.send("initialize", map[string]interface{}{
		"processId": nil,
		"rootUri":   "file:///tmp",
		"capabilities": map[string]interface{}{
			"workspace": map[string]interface{}{"configuration": true},
			"textDocument": map[string]interface{}{
				"completion": map[string]interface{}{
					"completionItem": map[string]interface{}{"snippetSupport": true},
				},
			},
		},
	})
	c.waitResponse(id, 5*time.Second)
	c.notify("initialized", map[string]interface{}{})
	time.Sleep(500 * time.Millisecond)
}

func (c *lspClient) openFile(uri, content string) {
	c.notify("textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "yaml", "version": 1, "text": content,
		},
	})
}

func (c *lspClient) completion(uri string, line, char int) []string {
	id := c.send("textDocument/completion", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": line, "character": char},
	})
	result := c.waitResponse(id, 10*time.Second)

	var items struct {
		Items []struct {
			Label string `json:"label"`
		} `json:"items"`
	}
	if err := json.Unmarshal(result, &items); err != nil {
		// Try as array
		var arr []struct {
			Label string `json:"label"`
		}
		if err := json.Unmarshal(result, &arr); err != nil {
			return nil
		}
		var labels []string
		for _, a := range arr {
			labels = append(labels, a.Label)
		}
		return labels
	}

	var labels []string
	for _, item := range items.Items {
		labels = append(labels, item.Label)
	}
	return labels
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			val := strings.TrimPrefix(line, "Content-Length: ")
			contentLength, _ = strconv.Atoi(val)
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

func findBinary(t *testing.T) string {
	t.Helper()
	// Try built binary first, then PATH
	local := filepath.Join("..", "..", "yaml-ls-k8s")
	if _, err := os.Stat(local); err == nil {
		abs, _ := filepath.Abs(local)
		return abs
	}
	path, err := exec.LookPath("yaml-ls-k8s")
	if err != nil {
		t.Skip("yaml-ls-k8s binary not found; run 'go build' or 'make' first")
	}
	return path
}

func requireYamlLS(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("yaml-language-server"); err != nil {
		t.Skip("yaml-language-server not installed")
	}
}

func contains(ss []string, s string) bool {
	for _, item := range ss {
		if item == s {
			return true
		}
	}
	return false
}
