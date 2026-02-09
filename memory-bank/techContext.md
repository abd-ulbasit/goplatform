# Technical Context

## Development Environment

### Required Tools
- **Go** 1.22+ - Primary language
- **kubectl** - Kubernetes CLI
- **kind** or **minikube** - Local Kubernetes cluster
- **kubebuilder** 3.x - Operator scaffolding
- **Terraform** 1.5+ - Infrastructure provisioning
- **AWS CLI** - AWS interaction
- **Docker** - Container builds
- **make** - Build automation

### Optional Tools
- **k9s** - Kubernetes TUI
- **stern** - Multi-pod log tailing
- **localstack** - Local AWS simulation

## Tech Stack

### Core
| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.22+ | Primary language |
| Kubernetes | 1.28+ | Container orchestration |
| Terraform | 1.5+ | Infrastructure as Code |
| AWS | - | Cloud provider |

### Frameworks
| Framework | Purpose |
|-----------|---------|
| controller-runtime | Kubernetes controller SDK |
| kubebuilder | Operator scaffolding |
| cobra | CLI framework |
| chi or gin | HTTP router |
| zap | Structured logging |

### Testing
| Tool | Purpose |
|------|---------|
| envtest | Controller integration tests |
| testify | Test assertions |
| mockery | Mock generation |
| localstack | AWS simulation |

## Project Structure

```
goplatform/
├── cmd/
│   ├── goplatform/          # Operator entry point
│   │   └── main.go
│   └── gpctl/               # CLI entry point
│       └── main.go
├── internal/
│   ├── controller/          # Kubernetes controllers
│   │   ├── application_controller.go
│   │   ├── database_controller.go
│   │   └── suite_test.go
│   ├── api/                 # REST API server
│   │   ├── server.go
│   │   └── handlers/
│   ├── terraform/           # Terraform integration
│   │   ├── runner.go
│   │   ├── state.go
│   │   └── modules/
│   ├── catalog/             # Service catalog
│   │   └── catalog.go
│   ├── observability/       # Prometheus/Grafana
│   │   ├── servicemonitor.go
│   │   └── dashboard.go
│   └── config/              # Configuration
│       └── config.go
├── pkg/
│   └── apis/
│       └── platform/
│           └── v1alpha1/    # CRD types
│               ├── application_types.go
│               ├── groupversion_info.go
│               └── zz_generated.deepcopy.go
├── deploy/
│   ├── helm/
│   │   └── goplatform/      # Helm chart
│   ├── kubernetes/          # Raw manifests
│   │   └── crds/
│   └── terraform/
│       └── modules/         # Reusable TF modules
│           ├── rds/
│           ├── elasticache/
│           └── sqs/
└── docs/
```

## Development Setup

```bash
# Clone repository
git clone https://github.com/abd-ulbasit/goplatform.git
cd goplatform

# Install dependencies
go mod download

# Create local cluster
kind create cluster --name goplatform

# Install CRDs
make install

# Run controller locally
make run

# Run tests
make test
```

## Configuration

### Controller Configuration
```yaml
# config.yaml
controller:
  metricsBindAddress: ":8080"
  healthProbeBindAddress: ":8081"
  leaderElection:
    enabled: true
    resourceName: goplatform-leader-election

terraform:
  binaryPath: "/usr/local/bin/terraform"
  workDir: "/tmp/goplatform-terraform"
  stateBackend:
    type: s3
    bucket: "goplatform-state"
    region: "us-east-1"
    dynamoDBTable: "goplatform-locks"

aws:
  region: "us-east-1"
  # Uses IRSA or IAM role attached to node
```

### Environment Variables
| Variable | Description | Default |
|----------|-------------|---------|
| `KUBECONFIG` | Kubernetes config path | `~/.kube/config` |
| `AWS_REGION` | AWS region | `us-east-1` |
| `TF_STATE_BUCKET` | S3 bucket for TF state | Required |
| `LOG_LEVEL` | Logging level | `info` |

## Dependencies

### Go Modules
```go
require (
    k8s.io/api v0.28.x
    k8s.io/apimachinery v0.28.x
    k8s.io/client-go v0.28.x
    sigs.k8s.io/controller-runtime v0.16.x
    github.com/spf13/cobra v1.8.x
    github.com/go-chi/chi/v5 v5.x
    go.uber.org/zap v1.26.x
)
```

## Technical Constraints

1. **Kubernetes Version** - Must support 1.25+ (CRD structural schemas)
2. **Terraform Version** - Must support 1.0+ (JSON output format)
3. **AWS Regions** - Must support all commercial AWS regions
4. **State Backend** - S3 + DynamoDB for production
5. **RBAC** - Controller needs cluster-admin-like permissions

## Performance Considerations

1. **Controller Concurrency** - Configurable workers per controller
2. **Terraform Parallelism** - Limit concurrent applies
3. **API Rate Limiting** - Prevent API server overload
4. **Caching** - Use informer cache, avoid direct API calls
