package proxy

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/bl4ko/yaml-ls-k8s/internal/config"
	"github.com/bl4ko/yaml-ls-k8s/internal/lsp"
)

// Proxy manages the lifecycle of yamlls and bidirectional message forwarding.
type Proxy struct {
	cfg     *config.Config
	logger  *log.Logger
	state   *State
	tracker *Tracker
	cmd     *exec.Cmd

	// yamlls stdio
	toServer   *bufio.Writer
	fromServer *bufio.Reader

	// editor stdio
	fromEditor *bufio.Reader
	toEditor   *bufio.Writer

	mu sync.Mutex // protects writes to toServer
}

func New(cfg *config.Config, logger *log.Logger) *Proxy {
	return &Proxy{
		cfg:     cfg,
		logger:  logger,
		state:   NewState(),
		tracker: NewTracker(),
	}
}

// Run spawns yamlls and starts bidirectional forwarding. Blocks until done.
func (p *Proxy) Run() error {
	p.cmd = exec.Command(p.cfg.YamlLSPath, "--stdio")
	p.cmd.Stderr = os.Stderr

	serverIn, err := p.cmd.StdinPipe()
	if err != nil {
		return err
	}
	serverOut, err := p.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := p.cmd.Start(); err != nil {
		return err
	}
	p.logger.Printf("spawned yamlls: %s (pid %d)", p.cfg.YamlLSPath, p.cmd.Process.Pid)

	p.toServer = bufio.NewWriter(serverIn)
	p.fromServer = bufio.NewReader(serverOut)
	p.fromEditor = bufio.NewReader(os.Stdin)
	p.toEditor = bufio.NewWriter(os.Stdout)

	var wg sync.WaitGroup
	wg.Add(2)

	// Editor → Server
	go func() {
		defer wg.Done()
		p.editorToServer()
	}()

	// Server → Editor
	go func() {
		defer wg.Done()
		p.serverToEditor()
	}()

	wg.Wait()
	return p.cmd.Wait()
}

// sendToServer writes a raw message to yamlls (thread-safe).
func (p *Proxy) sendToServer(data []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := lsp.WriteMessage(p.toServer, data); err != nil {
		p.logger.Printf("error writing to server: %v", err)
	}
	p.toServer.Flush()
}

// sendToEditor writes a raw message to the editor.
func (p *Proxy) sendToEditor(data []byte) {
	if err := lsp.WriteMessage(p.toEditor, data); err != nil {
		p.logger.Printf("error writing to editor: %v", err)
	}
	p.toEditor.Flush()
}
