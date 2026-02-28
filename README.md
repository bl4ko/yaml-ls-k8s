# yaml-ls-k8s

An LSP proxy that wraps [yaml-language-server](https://github.com/redhat-developer/yaml-language-server) and automatically provides Kubernetes schema validation and completion — no manual schema configuration needed.

It detects `apiVersion`/`kind` in your YAML files and fetches the matching JSON schema from GitHub, giving you validation, hover docs, and autocompletion for both core Kubernetes resources and CRDs out of the box.

## Features

- **Automatic schema detection** — parses `apiVersion` and `kind` from YAML content and maps them to the correct JSON schema
- **Core Kubernetes resources** — Deployments, Services, ConfigMaps, etc. via [yannh/kubernetes-json-schema](https://github.com/yannh/kubernetes-json-schema)
- **CRD support** — Gateway API, cert-manager, external-secrets, Argo, Istio, and 600+ more via [datreeio/CRDs-catalog](https://github.com/datreeio/CRDs-catalog)
- **Multi-document YAML** — handles `---` separated files with mixed resource types
- **Full ObjectMeta completion** — CRD schemas are enriched with the complete `metadata` definition (annotations, labels, name, namespace, etc.)
- **Local schema caching** — schemas are downloaded once and cached on disk
- **Modeline passthrough** — respects `# yaml-language-server: $schema=...` modelines when present
- **Zero dependencies** — built with Go standard library only

## How it works

```
Editor (stdin/stdout) → yaml-ls-k8s → yaml-language-server (child process)
                          ↕                        ↕
                    detect apiVersion/kind    schema validation
                    fetch & cache schemas     completions, hover
```

yaml-ls-k8s sits between your editor and yaml-language-server, intercepting LSP messages to inject the correct `yaml.schemas` configuration based on the content of each file.

## Installation

### Prerequisites

- Go 1.21+
- [yaml-language-server](https://github.com/redhat-developer/yaml-language-server):
  ```bash
  npm install -g yaml-language-server
  ```

### From source

```bash
git clone https://github.com/bl4ko/yaml-ls-k8s.git
cd yaml-ls-k8s
make install   # builds and copies to ~/.local/bin/
```

## Editor setup

### Neovim (with lspconfig)

```lua
require('lspconfig').yamlls.setup({
  cmd = { "yaml-ls-k8s", "--yamlls-path", "yaml-language-server" },
})
```

### Neovim (LazyVim)

```lua
-- lua/plugins/lspconfig.lua
return {
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        yamlls = {
          mason = false,
          cmd = { "yaml-ls-k8s", "--yamlls-path", "yaml-language-server" },
        },
      },
    },
  },
}
```

### Neovim (native `vim.lsp.config()`)

```lua
vim.lsp.config("yamlls", {
  cmd = { "yaml-ls-k8s", "--yamlls-path", "yaml-language-server" },
  root_markers = { ".git" },
  single_file_support = true,
})
vim.lsp.enable("yamlls")
```

### Helm files (via helm-ls)

If you use [helm-ls](https://github.com/mrjosh/helm-ls) for Helm templates, point its yamlls path to yaml-ls-k8s:

```lua
require('lspconfig').helm_ls.setup({
  settings = {
    ['helm-ls'] = {
      yamlls = {
        path = "yaml-ls-k8s",
      },
    },
  },
})
```

### Other editors

yaml-ls-k8s communicates over stdio and implements the standard LSP protocol. Any editor that supports configuring a custom LSP command can use it — just set the command to `yaml-ls-k8s --yamlls-path yaml-language-server`.

## CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `--yamlls-path` | `yaml-language-server` | Path to yaml-language-server binary |
| `--log-file` | `~/.config/yaml-ls-k8s/server.log` | Log file path |
| `--k8s-version` | `v1.33.0` | Kubernetes schema version |
| `--cache-dir` | `~/.cache/yaml-ls-k8s/schemas/` | Schema cache directory |

## Troubleshooting

**Check logs:**
```bash
tail -f ~/.config/yaml-ls-k8s/server.log
```

**Clear schema cache:**
```bash
rm -rf ~/.cache/yaml-ls-k8s/schemas/
```

**Test if a schema exists:**
```bash
# Core K8s resource
curl -s -o /dev/null -w "%{http_code}" \
  "https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/v1.33.0-standalone-strict/deployment-apps-v1.json"

# CRD
curl -s -o /dev/null -w "%{http_code}" \
  "https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/gateway.networking.k8s.io/httproute_v1.json"
```

## Development

```bash
make build       # build binary
make test        # run all tests (unit + e2e)
make test-unit   # unit tests only
make test-e2e    # e2e tests (requires yaml-language-server)
make install     # build + install to ~/.local/bin/
```

## License

MIT
