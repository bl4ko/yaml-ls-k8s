# yaml-ls-k8s

Standalone LSP server (Go) that wraps `yaml-language-server` (yamlls) and adds automatic Kubernetes schema detection by parsing `apiVersion`/`kind` from YAML content.

## Architecture

```
Editor (stdin) → yaml-ls-k8s → yamlls (child process, stdin/stdout)
                  ↕ intercept                    ↕
              Detector Chain              Schema Cache (~/.cache/)
              (K8s + CRD)                (disk + HTTP download)
```

### Message Flow
1. **didOpen/didChange**: Store full content, run detectors async, map URI→schema
2. **Server→Editor `workspace/configuration` request**: Track request ID
3. **Editor→Server config response**: If tracked ID, inject `yaml.schemas` into `results[0]["schemas"]`
4. **Initialize response**: Force `textDocumentSync: Full (1)` to avoid incremental sync bugs

## Project Structure

```
cmd/yaml-ls-k8s/main.go         # Entry point, CLI flags, wiring
internal/
  config/config.go               # Runtime config struct + defaults
  proxy/
    proxy.go                     # Spawn yamlls, bidirectional goroutines, shutdown
    editor.go                    # Editor→Server: didOpen/didChange/didClose + response interception
    server.go                    # Server→Editor: forceFullSync + track config request IDs
    tracker.go                   # Map JSON-RPC request IDs → methods
    state.go                     # Document content + schema state (thread-safe)
    detect.go                    # Schema detection + download + notify (async)
  lsp/
    transport.go                 # Content-Length read/write framing
    types.go                     # LSP types (BaseMessage, didOpen/didChange params, etc.)
  detector/
    detector.go                  # Detector interface + Chain (run all, aggregate)
    kubernetes/
      meta.go                    # extractAllTypeMeta: split ---, parse apiVersion/kind
      k8s.go                     # Core K8s detector (groups WITHOUT dots)
      crd.go                     # CRD detector (groups WITH dots, including *.k8s.io)
  schema/
    cache.go                     # Disk cache, file:// URIs, CRD ObjectMeta wrapping
    download.go                  # HTTP GET with timeout + negative caching (404 TTL)
    composite.go                 # anyOf schema for multi-doc files, SHA256-keyed dedup
  e2e/
    e2e_test.go                  # E2E test harness (spawns full pipeline)
    cases_test.go                # E2E test cases (Deployment, HTTPRoute, ExternalSecrets, modeline)
```

## Key Design Decisions

### K8s vs CRD group routing
- **K8sDetector**: groups WITHOUT dots (`v1`, `apps`, `batch`, `autoscaling`) → `yannh/kubernetes-json-schema`
- **CRDDetector**: ALL groups WITH dots (`cert-manager.io`, `gateway.networking.k8s.io`, `external-secrets.io`) → `datreeio/CRDs-catalog`

### Schema injection format
yamlls sends `workspace/configuration` requesting sections like `["yaml", "http", "[yaml]", "editor", "files"]`. The editor responds with an **array where each element is the section VALUE** (not wrapped). So for the `yaml` section, the response is `{"schemas": {...}, "schemaStore": {...}}`. We inject into `results[0]["schemas"]` directly — NOT into `results[0]["yaml"]["schemas"]`.

### CRD ObjectMeta wrapping
CRD schemas from datreeio have `metadata: {"type": "object"}` with no properties. We download a core K8s schema (pod-v1), extract the full ObjectMeta definition (annotations, labels, name, namespace, etc.), and inject it into CRD schemas. Wrapped schemas cached as `*_wrapped.json`.

### Preventing infinite loops
`didChangeConfiguration` → yamlls sends `workspace/configuration` → we inject → could loop. Fix: `SetSchemas()` returns `bool` indicating whether mapping changed; only send `didChangeConfiguration` when it actually changed.

### Negative caching
404 responses cached in memory with 1-hour TTL to prevent hammering GitHub on every keystroke for nonexistent CRDs.

## CLI Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `--yamlls-path` | `yaml-language-server` | Path to yamlls binary |
| `--log-file` | `~/.config/yaml-ls-k8s/server.log` | Log file (never stdout) |
| `--k8s-version` | `v1.33.0` | K8s schema version |
| `--cache-dir` | `~/.cache/yaml-ls-k8s/schemas/` | Schema cache directory |
| `--stdio` | `true` | Ignored, LSP client compat |

## Development

```bash
make build          # Build binary
make test           # Run unit + e2e tests
make test-unit      # Unit tests only
make test-e2e       # E2E tests only (requires yamlls installed)
make install        # Build + copy to ~/.local/bin/
```

## Dependencies

- Go standard library only (no external deps)
- `yaml-language-server` must be installed (`npm i -g yaml-language-server` or via Mason)

## Pre-commit Hooks

- trailing-whitespace, end-of-file-fixer, check-yaml, check-json
- go build, go vet, go test (unit + e2e)

## Neovim Integration (LazyVim)

Configured in `~/.config/nvim/lua/plugins/lspconfig.lua`:
- `yamlls.cmd = {"yaml-ls-k8s", "--yamlls-path", "yaml-language-server"}` for direct YAML files
- `helm_ls.yamlls.path = "yaml-ls-k8s"` for Helm templates (helm-ls renders templates, passes clean YAML to yaml-ls-k8s)

**Critical lspconfig settings** (LazyVim + native `vim.lsp.config()` API):
- `mason = false` — required. With `mason = true`, LazyVim delegates `vim.lsp.enable()` to mason-lspconfig's `automatic_enable`, which doesn't trigger for custom `cmd` binaries not installed by Mason.
- `root_markers = {".git"}` + `single_file_support = true` — required. The lspconfig-style `root_dir` function is NOT compatible with the native `vim.lsp.config()` API that LazyVim uses. `root_markers` is the native equivalent, and `single_file_support` ensures attachment even outside git repos.

## Schema Sources

- **Core K8s**: `https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/{version}-standalone-strict/{kind}-{group}-{version}.json`
- **CRDs**: `https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{group}/{kind}_{version}.json`
- **ObjectMeta**: Extracted from `pod-v1.json` standalone-strict schema

## Debugging

- **Log file**: `~/.config/yaml-ls-k8s/server.log` — shows all detection, download, injection activity
- **Cache**: `~/.cache/yaml-ls-k8s/schemas/` — inspect cached/wrapped schemas
- **Clear cache**: `rm -rf ~/.cache/yaml-ls-k8s/schemas/` to force re-download
- **Test schema URL**: `curl -s -o /dev/null -w "%{http_code}" "URL"` to check if schema exists on GitHub
