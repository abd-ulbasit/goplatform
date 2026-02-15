# Tech Context

## Development Environment

### Required Tools
- Go 1.22+ (primary language)
- kubectl (cluster interaction)
- kind (testing clusters)
- kubebuilder v4 (operator scaffolding)
- Docker (container builds)
- make (build automation)

### Optional Tools
- Colima (local K8s for daily dev on macOS)
- k9s (Kubernetes TUI)
- stern (multi-pod log tailing)

## Tech Stack

| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.22+ | Primary language |
| Kubernetes | 1.28+ | Container orchestration |
| controller-runtime | v0.23.x | Kubernetes controller SDK |
| kubebuilder | v4 | Operator scaffolding |

### Infrastructure Operators (provisioned by GoPlatform)
| Operator | CRD | Purpose |
|----------|-----|---------|
| CloudNativePG | Cluster | PostgreSQL databases |
| Spotahome Redis Operator | RedisFailover | Redis caches |
| RabbitMQ Cluster Operator | RabbitmqCluster | Message queues |

### Monitoring (planned M8)
| Technology | Purpose |
|------------|---------|
| Prometheus Operator | ServiceMonitor, PrometheusRule CRDs |
| prometheus client_golang | Custom controller metrics |

### Policy (planned M11)
| Technology | Purpose |
|------------|---------|
| Kyverno | ClusterPolicy for organizational rules |

### Testing
| Tool | Purpose |
|------|---------|
| Ginkgo/Gomega | BDD-style testing |
| envtest | Controller integration tests (real kube-apiserver + etcd) |
| Kind | E2E testing clusters |

## Project Structure

```
goplatform/
├── cmd/main.go                    # Operator entry point
├── api/v1alpha1/                  # CRD type definitions
├── internal/
│   ├── controller/                # ApplicationReconciler
│   └── provider/                  # InfrastructureProvider implementations
├── config/
│   ├── crd/bases/                 # Generated CRD YAML (make manifests)
│   ├── rbac/                      # Generated RBAC (make manifests)
│   ├── samples/                   # Example Application CRs
│   └── manager/                   # Controller deployment manifest
├── test/e2e/                      # E2E tests
├── deploy/helm/                   # Helm chart
├── memory-bank/                   # AI development notes
├── PROGRESS.md                    # Milestone tracking (source of truth)
├── CLAUDE.md                      # AI coding instructions
└── .github/copilot-instructions.md # Copilot instructions
```

## Development Setup

```bash
git clone https://github.com/abd-ulbasit/goplatform.git
cd goplatform
go mod download
make install    # Install CRDs
make run        # Run controller locally
make test       # Run tests
```

## Go Module

**Module:** `github.com/abd-ulbasit/goplatform`
**Go Version:** 1.25.3

**Key Dependencies:**
- `sigs.k8s.io/controller-runtime v0.23.1`
- `k8s.io/apimachinery v0.35.0`
- `k8s.io/client-go v0.35.0`
- `k8s.io/api v0.35.0`
- `github.com/onsi/ginkgo/v2 v2.27.2`
- `github.com/onsi/gomega v1.38.2`

## Technical Constraints

1. Kubernetes-only (no cloud provider APIs, no Terraform)
2. kubectl-only interface (no CLI tool, no REST API)
3. Must work with Kind for testing
4. Self-hosted (no SaaS dependencies)
