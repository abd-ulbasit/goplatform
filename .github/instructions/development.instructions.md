---
applyTo: '**'
description: 'Development environment setup and Serena MCP tools usage'
---

# Development Environment Instructions

## Local Kubernetes Setup

**Hardware:** macOS with Colima (Docker runtime)

### Kubernetes Version Requirements
- **Target:** Kubernetes 1.33+ 
- **Reason:** Avoid extended support which costs 6x more on cloud providers
- **Local:** Match the target production version for parity

### Colima Setup
```bash
# Install Colima if not present
brew install colima

# Start Colima with Kubernetes enabled (K8s 1.33+)
colima start --kubernetes --cpu 4 --memory 8 --kubernetes-version v1.33.0

# Verify
kubectl cluster-info
kubectl version --short
```

### Local Development vs Cloud
When testing infrastructure provisioning:
1. **LocalStack** for AWS service emulation (RDS, ElastiCache, SQS)
2. **Kind/Colima** for Kubernetes resources
3. **CloudNativePG** operator for local PostgreSQL testing
4. **Redis operator** for local Redis testing

### Testing Strategy
- Unit tests: `envtest` with fake K8s API
- Integration tests: Local Colima cluster + LocalStack
- E2E tests: Real AWS (use separate test account)

## Serena MCP Tools Usage

When working with this codebase, prefer Serena's semantic tools over raw file operations:

### DO Use (Token Efficient)
- `find_symbol` - Find specific functions, structs, interfaces by name
- `get_symbols_overview` - Get package structure without reading entire files
- `find_referencing_symbols` - Trace dependencies and usages
- `replace_symbol_body` - Edit specific functions/methods
- `insert_before_symbol` / `insert_after_symbol` - Add new code precisely

### DON'T Do
- Read entire files when seeking specific symbols
- Re-read files already analyzed in the same session
- Use file-level operations when symbol-level works

### Example Workflow
```
1. Use get_symbols_overview to understand package structure
2. Use find_symbol with name path pattern to locate specific code
3. Use find_referencing_symbols to understand dependencies
4. Use replace_symbol_body to make targeted changes
```

## Go Development Standards

### Project Structure
```
cmd/           - Main applications (goplatform, gpctl)
internal/      - Private application code
pkg/           - Public libraries (APIs)
deploy/        - Deployment configurations
```

### Testing Commands
```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific package tests
go test ./internal/controller/...

# Run with envtest (Kubernetes API)
go test ./internal/controller/... -tags=integration
```

### Code Generation
```bash
# Generate CRD manifests and deep copy
make generate

# Generate CRD YAML
make manifests

# Update go.mod
go mod tidy
```

## Kubernetes Development

### Apply CRDs to Local Cluster
```bash
kubectl apply -f deploy/kubernetes/crds/

# Verify CRD installation
kubectl get crd applications.platform.goplatform.io
```

### Run Controller Locally
```bash
# Run outside cluster (for development)
make run

# Or with specific kubeconfig
KUBECONFIG=~/.kube/config go run ./cmd/goplatform
```

### Debug Controller
```bash
# Enable verbose logging
go run ./cmd/goplatform --zap-log-level=debug

# With delve debugger
dlv debug ./cmd/goplatform -- --zap-log-level=debug
```
