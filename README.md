# 🚀 KUBEJAX - Kubernetes Jump Across conteXts

A lightning-fast tool that extends kubectx functionality by automatically detecting and managing multiple kubeconfig files with advanced search and production safety features.

## ⚡ Why KUBEJAX?

- **3-letter command: `kjx`** - super fast to type!
- **🔍 Advanced Search** - fuzzy search in contexts and namespaces
- **⚠️ Enhanced Production Safety** - dual-layer production environment detection
- **📁 Multi-file support** - manage contexts from multiple kubeconfig files
- **⚡ Shell integration** - automatic KUBECONFIG environment variable management
- **🔧 Config Preservation** - maintains original kubeconfig structure (no data loss)

## Installation

### Prerequisites
- Go 1.21+, Git, kubectl
- Linux or macOS (Windows support coming soon)

### Quick Start
```bash
# Clone and build
git clone <repository-url>
cd kubejax
go mod tidy
go build -o kjx .

# Install (REQUIRED for KUBECONFIG export)
sudo cp kjx /usr/local/bin/kjx
kjx install
source ~/.zshrc  # or ~/.bashrc

# Verify
kjx -l
```

## Setup

1. **Create config directory:**
```bash
mkdir -p ~/.kube/configs
```

2. **Place your kubeconfig files:**
```bash
~/.kube/configs/
├── dev-cluster.conf
├── staging-cluster.conf
├── prod-cluster.conf
└── prod-main-k8s.yaml
```

## Usage

### Context Management
```bash
kjx -l                    # List all contexts
kjx -c                    # Show current context info
kjx -i                    # Interactive selection with search
kjx -s [term]            # Search contexts
kjx context-name         # Direct switch
kjx -                    # Previous context
```

### Namespace Management
```bash
kjx ns -l                # List namespaces
kjx ns -c                # Show current namespace info
kjx ns -i                # Interactive selection with search
kjx ns -s [term]         # Search namespaces
kjx ns namespace-name    # Direct switch
```

### Search Examples
```bash
# Interactive search with real-time filtering
kjx -i                   # Type to filter contexts
kjx ns -i                # Type to filter namespaces

# Direct search
kjx -s prod              # Find contexts containing "prod"
kjx ns -s app            # Find namespaces containing "app"

# Auto-switch if single match
kjx -s staging           # Switches automatically if only one match
```

## Production Safety

### Dual-Layer Detection
KUBEJAX detects production environments by checking:
1. **Context name** for keywords: `prd`, `prod`, `production`
2. **Config file name** for same keywords

### Examples
```bash
# Warning triggers for:
prod-east                 # Context name
dev-context in prod-cluster.yaml  # File name
```

### Enhanced Warnings
```bash
⚠️  WARNING: PRODUCTION ENVIRONMENT DETECTED!
🔴 You are selecting context: 'prod-east'
🔴 Context name contains production keywords
🔴 Config file 'prod-cluster.yaml' contains production keywords
🔴 Please be extra careful with any changes!
```

## Interactive Examples

```bash
# Context selection with search
$ kjx -i
? Select context (type to search/filter): prod
  prod-east (prod-cluster.conf) 🔴
  prod-west (prod-cluster.conf) 🔴
# Type filters results in real-time

# Current context info
$ kjx -c
📍 Current Kubernetes Context Information:
🔹 Context: prod-east
📁 Config File: prod-cluster.conf
🏗️ Cluster: prod-cluster
📦 Namespace: default
⚠️  PRODUCTION ENVIRONMENT DETECTED!
💾 KUBECONFIG: /path/to/prod-cluster.conf
```

## Configuration

### Environment Variables
- `KUBECONFIG`: Automatically set by KUBEJAX
- `HOME`: Used for default config directory

### Custom Config Directory
```bash
kjx -d /custom/path/to/configs -l
```

## Troubleshooting

### KUBECONFIG Not Exported
```bash
# Verify installation
which kjx               # Should show: /usr/local/bin/kjx
type kjx                # Should show: kjx is a shell function

# Reinstall if needed
kjx install
source ~/.zshrc

# Manual export (temporary fix)
export KUBECONFIG=/path/to/your/config.yaml
```

### Shell Integration Issues
```bash
# Manual shell function (add to ~/.zshrc)
kjx() {
    local temp_file=$(mktemp)
    if /usr/local/bin/kjx --output-config "$temp_file" "$@"; then
        [ -s "$temp_file" ] && export KUBECONFIG="$(cat "$temp_file")" && echo "✅ KUBECONFIG: $KUBECONFIG"
    fi
    rm -f "$temp_file"
}
```

## All Commands Reference

```bash
# Context Operations
kjx -l                    # List contexts
kjx -c                    # Current context info
kjx -i                    # Interactive selection
kjx -s [term]            # Search contexts
kjx context-name         # Direct switch
kjx -                    # Previous context

# Namespace Operations
kjx ns -l                # List namespaces
kjx ns -c                # Current namespace info
kjx ns -i                # Interactive selection
kjx ns -s [term]         # Search namespaces
kjx ns namespace-name    # Direct switch

# Configuration
kjx -d /path -l          # Custom config directory
kjx install              # Install shell integration
kjx shell-init           # Show shell function code
```

## Development

### Build
```bash
go build -o kjx .
```

### Cross-platform builds
```bash
GOOS=linux GOARCH=amd64 go build -o kjx-linux .
GOOS=darwin GOARCH=amd64 go build -o kjx-macos .
GOOS=darwin GOARCH=arm64 go build -o kjx-macos-arm64 .
```

## Platform Support

### Currently Supported
- **Linux** (Ubuntu, CentOS, Debian)
- **macOS** (Intel and Apple Silicon)

### Coming Soon
- **Windows** with PowerShell integration

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

### Recent Features
- ✅ Enhanced production environment detection (dual-layer)
- ✅ Advanced search with fuzzy matching
- ✅ Real-time filtering in interactive mode
- ✅ Complete kubeconfig structure preservation

### Upcoming Features
- Windows support with PowerShell integration
- Previous namespace switching (`kjx ns -`)
- Bash/Zsh/Fish completion
- Plugin system and cluster health checks
- Context aliasing and history

---

**🎯 TL;DR**: 
1. Install: `sudo cp kjx /usr/local/bin/kjx && kjx install && source ~/.zshrc`
2. Search: `kjx -s` and `kjx ns -s` for interactive search
3. Safety: Automatic production environment warnings
4. Export: KUBECONFIG automatically exported to shell

**🔍 Search anywhere, switch safely!** ⚡