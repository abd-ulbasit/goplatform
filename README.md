# GoPlatform

**Internal Developer Platform - Kubernetes Operator for Self-Service Infrastructure**

Build a production-grade Internal Developer Platform that transforms declarative Application CRDs into fully provisioned, observable infrastructure. Think: Backstage + Crossplane + Terraform Cloud in a single, self-hosted solution.

---

## The Problem

Platform teams spend endless hours on:
- **Ticket-based provisioning** - "Please create my database" (3-day SLA)
- **Snowflake infrastructure** - Every service configured differently
- **Knowledge silos** - Only platform team knows how to deploy
- **Compliance gaps** - Manual security reviews, inconsistent policies
- **Cost opacity** - No idea what each team spends

## The Solution

Developers declare what they need. The platform provisions everything automatically:

```yaml
apiVersion: platform.goplatform.io/v1alpha1
kind: Application
metadata:
  name: payments-api
spec:
  team: payments
  tier: critical
  
  workload:
    image: ghcr.io/org/payments-api:v1.0.0
    replicas: 3
  
  # Cloud-agnostic infrastructure requests
  database:
    type: postgres
    size: small
    highAvailability: true
  
  cache:
    type: redis
    size: small
  
  queue:
    type: sqs
    deadLetterQueue:
      enabled: true

# GoPlatform automatically provisions:
# ✅ Kubernetes Deployment, Service, HPA, PDB
# ✅ AWS RDS PostgreSQL (via Terraform)
# ✅ AWS ElastiCache Redis (via Terraform)
# ✅ AWS SQS Queue with DLQ (via Terraform)
# ✅ IAM roles with least-privilege (IRSA)
# ✅ Prometheus ServiceMonitor
# ✅ Grafana dashboard (auto-generated)
# ✅ Alerting rules based on SLA tier
# ✅ Service catalog entry with dependencies
```

---

## What Makes GoPlatform Different

| Feature | GoPlatform | Backstage | Crossplane | Terraform Cloud |
|---------|:----------:|:---------:|:----------:|:---------------:|
| **K8s Native (CRDs)** | ✅ | ❌ | ✅ | ❌ |
| **Self-Hosted** | ✅ | ✅ | ✅ | ❌ |
| **K8s + Cloud Resources** | ✅ | ❌ | ✅ | ❌ |
| **Use Existing TF Modules** | ✅ | ❌ | ❌ | ✅ |
| **Service Catalog Built-in** | ✅ | ✅ | ❌ | ❌ |
| **Cost Estimation** | ✅ | ❌ | ❌ | ❌ |
| **Preview Environments** | ✅ | ❌ | ❌ | ❌ |
| **Drift Detection** | ✅ | ❌ | ✅ | ✅ |
| **Complexity** | Medium | High | High | Low |

### Unique Capabilities

#### 🔮 Cost Estimation Before Provisioning
```bash
$ gpctl estimate payments-api.yaml

Estimated Monthly Cost: $73.95
├── RDS db.t3.small (PostgreSQL):  $49.64
├── RDS storage (100GB gp2):       $11.50
├── ElastiCache cache.t3.micro:    $12.41
└── SQS (~1M messages):            $0.40

Proceed with provisioning? [y/N]
```

#### 🚀 Preview Environments for Every PR
Open a PR → Get a fully isolated environment with its own database, cache, and URL.
```
🚀 Preview environment ready!
URL: https://pr-42.payments-api.preview.example.com
Logs: https://grafana.example.com/d/preview-pr-42
```

#### 🔄 Environment Promotion
```bash
$ gpctl promote payments-api --from dev --to staging

Diff:
  replicas: 1 → 2
  database.size: small → medium

Promote to staging? [y/N]
```

#### 🔍 Drift Detection & Self-Healing
Detects when cloud resources drift from desired state:
```yaml
status:
  conditions:
    - type: DriftDetected
      status: "True"
      message: "aws_db_instance.main instance_class drifted: db.t3.small → db.t3.medium"
```

#### 💰 Team Budgets & Cost Controls
```yaml
apiVersion: platform.goplatform.io/v1alpha1
kind: TeamQuota
metadata:
  name: payments-team
spec:
  maxApplications: 10
  maxMonthlyBudget: 5000  # USD
  alerts:
    - threshold: 80
      notify: slack
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Developer Interface                            │
│   kubectl apply  │  gpctl CLI  │  REST API  │  GitOps (ArgoCD)              │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Kubernetes API Server                               │
│   Application CRD  │  ProviderConfig CRD  │  TeamQuota CRD                 │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │ watch
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        GoPlatform Controller                                │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────────┐  │
│  │ App Reconciler  │  │ Infra Provider  │  │ Observability Controller   │  │
│  │ - K8s resources │  │ - AWS (TF)      │  │ - ServiceMonitor           │  │
│  │ - Status mgmt   │  │ - GCP (future)  │  │ - Grafana Dashboard        │  │
│  │ - Conditions    │  │ - Local (dev)   │  │ - PrometheusRule           │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
          │                      │                         │
          ▼                      ▼                         ▼
┌─────────────────┐   ┌─────────────────────┐   ┌─────────────────────────────┐
│ K8s Resources   │   │ Cloud Infrastructure│   │ Observability Stack         │
│ - Deployment    │   │ - RDS PostgreSQL   │   │ - Prometheus scraping      │
│ - Service       │   │ - ElastiCache Redis│   │ - Grafana dashboards       │
│ - HPA, PDB      │   │ - SQS Queues       │   │ - Alert rules              │
│ - ConfigMap     │   │ - S3 Buckets       │   │                             │
│ - Secret        │   │ - IAM Roles (IRSA) │   │                             │
└─────────────────┘   └─────────────────────┘   └─────────────────────────────┘
```

---

## Cloud-Agnostic Design

The Application CRD is cloud-agnostic. The platform maps abstract specs to provider-specific resources:

```
┌────────────────────────────────────────────────────────────────────────────┐
│                        Application Spec (Cloud-Agnostic)                   │
│                                                                            │
│    database:               cache:                 queue:                   │
│      type: postgres          type: redis            type: sqs              │
│      size: small             size: small            fifo: false            │
│      highAvailability: true                                                │
└────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
              ┌──────────────────────┼──────────────────────┐
              ▼                      ▼                      ▼
┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐
│    AWS Provider     │  │    GCP Provider     │  │   Local Provider    │
│                     │  │     (future)        │  │                     │
│  postgres+small →   │  │  postgres+small →   │  │  postgres+small →   │
│  RDS db.t3.micro    │  │  Cloud SQL db-f1    │  │  CloudNativePG      │
│                     │  │                     │  │                     │
│  redis+small →      │  │  redis+small →      │  │  redis+small →      │
│  ElastiCache        │  │  Memorystore Redis  │  │  Redis Operator     │
│  cache.t3.micro     │  │                     │  │                     │
└─────────────────────┘  └─────────────────────┘  └─────────────────────┘
```

---

## Quick Start

### Prerequisites
- Kubernetes 1.33+ cluster (Colima, kind, or EKS)
- kubectl configured
- AWS credentials (for cloud resources)

### Installation
```bash
# Install CRDs and controller
helm install goplatform deploy/helm/goplatform -n goplatform-system --create-namespace

# Configure AWS provider
kubectl apply -f - <<EOF
apiVersion: platform.goplatform.io/v1alpha1
kind: ProviderConfig
metadata:
  name: aws-default
spec:
  provider: aws
  aws:
    region: us-east-1
    terraformStateBackend:
      bucket: my-platform-tf-state
      dynamodbTable: my-platform-tf-locks
EOF
```

### Deploy an Application
```bash
# Create an application
kubectl apply -f - <<EOF
apiVersion: platform.goplatform.io/v1alpha1
kind: Application
metadata:
  name: demo-app
spec:
  team: demo-team
  tier: standard
  workload:
    image: nginx:latest
    replicas: 2
  database:
    type: postgres
    size: small
EOF

# Watch provisioning status
kubectl get applications -w

# Check detailed status
kubectl describe application demo-app

# Or use the CLI
gpctl status demo-app
```

---

## Development Phases

| Phase | Description | Status |
|-------|-------------|--------|
| **Phase 1** | Operator Foundation (CRDs, Controllers, Status, Finalizers) | 🔜 Not Started |
| **Phase 2** | Infrastructure Providers (Provider interface, Terraform, RDS, Redis, SQS, IAM) | 📋 Planned |
| **Phase 3** | Credential Management (Secrets, External Secrets, Rotation) | 📋 Planned |
| **Phase 4** | Platform API & CLI (REST API, Cost Estimation, gpctl) | 📋 Planned |
| **Phase 5** | Observability (ServiceMonitor, Dashboards, Alerts, Tracing) | 📋 Planned |
| **Phase 6** | Service Catalog (Dependencies, Team Ownership, Templates) | 📋 Planned |
| **Phase 7** | Developer Experience (Env Promotion, Preview Envs, Local Dev, Drift) | 📋 Planned |
| **Phase 8** | Production Hardening (Policies, Quotas, Audit, HA) | 📋 Planned |

See [PROGRESS.md](PROGRESS.md) for detailed milestone tracking.

---

## Project Structure

```
goplatform/
├── cmd/
│   ├── goplatform/             # Operator binary
│   └── gpctl/                  # CLI tool
├── internal/
│   ├── controller/             # Kubernetes controllers
│   ├── api/                    # REST API server
│   ├── terraform/              # Terraform runner
│   ├── catalog/                # Service catalog
│   └── observability/          # Prometheus/Grafana integration
├── pkg/
│   └── apis/
│       └── platform/
│           └── v1alpha1/       # CRD types
├── deploy/
│   ├── helm/goplatform/        # Helm chart
│   ├── kubernetes/             # Raw K8s manifests
│   └── terraform/modules/      # Terraform modules for AWS
├── PROGRESS.md                 # Milestone tracking
└── README.md
```

---

## Tech Stack

- **Language:** Go 1.22+
- **Operator Framework:** controller-runtime, kubebuilder
- **CLI:** cobra, viper
- **API:** chi (or gin)
- **Infrastructure:** Terraform, AWS SDK v2
- **Observability:** Prometheus, Grafana, OpenTelemetry

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

**Building platforms to empower developers.**
