# GoPlatform

**Internal Developer Platform - Kubernetes Operator for Self-Service Infrastructure**

Build a production-grade Internal Developer Platform that transforms declarative Application CRDs into fully provisioned, observable infrastructure. Think: Backstage + Crossplane + Terraform Cloud in a single, self-hosted solution.

---

## The Problem

Platform teams spend endless hours on:
1. **Ticket-based provisioning** - "Please create my database" (3-day SLA)
2. **Snowflake infrastructure** - Every service configured differently
3. **Knowledge silos** - Only platform team knows how to deploy
4. **Compliance gaps** - Manual security reviews, inconsistent policies
5. **Cost opacity** - No idea what each team spends

## The Solution

GoPlatform lets developers declare what they need, and the platform provisions everything automatically:

```yaml
# Developer applies this YAML
apiVersion: platform.goplatform.io/v1alpha1
kind: Application
metadata:
  name: payments-api
  namespace: default
spec:
  team: payments
  language: go

  # What the developer needs
  database:
    type: postgres
    size: small
  cache:
    type: redis
  queue:
    type: sqs

# GoPlatform automatically provisions:
# ✅ Kubernetes Deployment, Service, HPA
# ✅ AWS RDS PostgreSQL (via Terraform)
# ✅ AWS ElastiCache Redis (via Terraform)
# ✅ AWS SQS Queue (via Terraform)
# ✅ IAM roles with least-privilege
# ✅ Prometheus ServiceMonitor
# ✅ Grafana dashboard
# ✅ Service catalog entry
```

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              GoPlatform                                          │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                        Developer Experience Layer                           │ │
│  │                                                                             │ │
│  │  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐   ┌─────────────────┐  │ │
│  │  │ kubectl     │   │ gpctl CLI   │   │ REST API    │   │ GitOps          │  │ │
│  │  │ apply -f    │   │ apply/status│   │ /api/v1/    │   │ (ArgoCD sync)   │  │ │
│  │  └──────┬──────┘   └──────┬──────┘   └──────┬──────┘   └────────┬────────┘  │ │
│  │         │                 │                 │                    │          │ │
│  └─────────┼─────────────────┼─────────────────┼────────────────────┼──────────┘ │
│            │                 │                 │                    │            │
│            ▼                 ▼                 ▼                    ▼            │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                         Kubernetes API Server                               │ │
│  │                                                                             │ │
│  │  Application CRD    Database CRD    Cache CRD    Queue CRD                 │ │
│  │  (desired state stored in etcd)                                            │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│                                        │                                         │
│                                        │ watch                                   │
│                                        ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                    GoPlatform Controller (Operator)                         │ │
│  │                                                                             │ │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────────┐  │ │
│  │  │ App Reconciler  │  │ DB Reconciler   │  │ Observability Reconciler    │  │ │
│  │  │                 │  │                 │  │                             │  │ │
│  │  │ - Watch App CRD │  │ - Watch DB CRD  │  │ - Generate ServiceMonitor   │  │ │
│  │  │ - Create K8s    │  │ - Call Terraform│  │ - Generate Grafana dash     │  │ │
│  │  │   resources     │  │ - Manage state  │  │ - Create AlertRules         │  │ │
│  │  │ - Update status │  │ - Handle errors │  │ - Configure tracing         │  │ │
│  │  └────────┬────────┘  └────────┬────────┘  └─────────────┬───────────────┘  │ │
│  │           │                    │                         │                  │ │
│  │           │              ┌─────┴─────┐                   │                  │ │
│  │           │              ▼           ▼                   │                  │ │
│  │           │     ┌────────────┐  ┌────────────┐           │                  │ │
│  │           │     │ TF Runner  │  │ TF State   │           │                  │ │
│  │           │     │            │  │ Manager    │           │                  │ │
│  │           │     │ - Generate │  │            │           │                  │ │
│  │           │     │   modules  │  │ - S3 state │           │                  │ │
│  │           │     │ - Apply    │  │ - DDB lock │           │                  │ │
│  │           │     │ - Destroy  │  │ - Isolate  │           │                  │ │
│  │           │     └────────────┘  └────────────┘           │                  │ │
│  │           │                                              │                  │ │
│  └───────────┼──────────────────────────────────────────────┼──────────────────┘ │
│              │                                              │                    │
│              ▼                                              ▼                    │
│  ┌─────────────────────────────────┐  ┌─────────────────────────────────────────┐│
│  │     Kubernetes Resources        │  │         Observability Stack             ││
│  │                                 │  │                                         ││
│  │  ┌───────────┐  ┌───────────┐   │  │  ┌─────────────┐  ┌─────────────────┐   ││
│  │  │Deployment │  │  Service  │   │  │  │ Prometheus  │  │ Grafana         │   ││
│  │  │+ replicas │  │+ ClusterIP│   │  │  │ scrapes     │  │ dashboards      │   ││
│  │  └───────────┘  └───────────┘   │  │  │ metrics     │  │ auto-generated  │   ││
│  │  ┌───────────┐  ┌───────────┐   │  │  └─────────────┘  └─────────────────┘   ││
│  │  │    HPA    │  │    PDB    │   │  │  ┌─────────────┐  ┌─────────────────┐   ││
│  │  │autoscale  │  │disruption │   │  │  │ AlertRules  │  │ ServiceMonitor  │   ││
│  │  │           │  │ budget    │   │  │  │ SLA-based   │  │ per-app config  │   ││
│  │  └───────────┘  └───────────┘   │  │  └─────────────┘  └─────────────────┘   ││
│  │  ┌───────────┐  ┌───────────┐   │  │                                         ││
│  │  │ConfigMap  │  │  Secret   │   │  └─────────────────────────────────────────┘│
│  │  │app config │  │credentials│   │                                             │
│  │  └───────────┘  └───────────┘   │                                             │
│  └─────────────────────────────────┘                                             │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                         AWS Infrastructure                                  │ │
│  │                         (Provisioned via Terraform)                         │ │
│  │                                                                             │ │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐  │ │
│  │  │   RDS     │  │ElastiCache│  │    SQS    │  │    S3     │  │    IAM    │  │ │
│  │  │ PostgreSQL│  │   Redis   │  │  Queues   │  │  Buckets  │  │   Roles   │  │ │
│  │  │ + replicas│  │ + cluster │  │ + DLQ     │  │ + policy  │  │ + IRSA    │  │ │
│  │  └───────────┘  └───────────┘  └───────────┘  └───────────┘  └───────────┘  │ │
│  │                                                                             │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                         Service Catalog                                     │ │
│  │                                                                             │ │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────────┐  │ │
│  │  │ Dependency Graph│  │ Team Ownership  │  │ Compliance Status           │  │ │
│  │  │ A → B → C       │  │ payments: [app] │  │ ✓ resource limits          │  │ │
│  │  │                 │  │ orders: [app2]  │  │ ✓ security policies         │  │ │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────────────┘  │ │
│  │                                                                             │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

---

## Component Breakdown

### 1. Kubernetes Operator (Core)

The heart of GoPlatform - a controller that watches CRDs and reconciles desired state.

```
┌─────────────────────────────────────────────────────────────────┐
│                    Controller Runtime                           │
│                                                                 │
│  ┌───────────────┐     ┌───────────────┐     ┌───────────────┐  │
│  │  Informer     │────►│  Work Queue   │────►│  Reconciler   │  │
│  │  (watch CRDs) │     │  (rate limit) │     │  (reconcile)  │  │
│  └───────────────┘     └───────────────┘     └───────────────┘  │
│                                                     │           │
│                                              ┌──────┴──────┐    │
│                                              ▼             ▼    │
│                                       ┌───────────┐ ┌──────────┐│
│                                       │Create K8s │ │Call      ││
│                                       │Resources  │ │Terraform ││
│                                       └───────────┘ └──────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### 2. Terraform Integration

Programmatic Terraform for AWS resource provisioning.

```
┌─────────────────────────────────────────────────────────────────┐
│                    Terraform Runner                             │
│                                                                 │
│  ┌───────────────┐     ┌───────────────┐     ┌───────────────┐  │
│  │ Module Gen    │────►│ TF Init/Plan  │────►│ TF Apply      │  │
│  │ (Go → HCL)    │     │ (validate)    │     │ (provision)   │  │
│  └───────────────┘     └───────────────┘     └───────────────┘  │
│         │                                           │           │
│         ▼                                           ▼           │
│  ┌───────────────┐                          ┌───────────────┐   │
│  │ Per-App State │                          │ Output Parser │   │
│  │ s3://bucket/  │                          │ (endpoints,   │   │
│  │ apps/{ns}/{n} │                          │  credentials) │   │
│  └───────────────┘                          └───────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 3. Service Catalog

Track all applications, dependencies, and ownership.

```
┌─────────────────────────────────────────────────────────────────┐
│                    Service Catalog                              │
│                                                                 │
│  Applications:                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ payments-api      │ team: payments │ deps: [orders-db]    │ │
│  │ orders-service    │ team: orders   │ deps: [payments-api] │ │
│  │ notification-svc  │ team: platform │ deps: [sqs-queue]    │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  Dependency Graph:                                              │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │     orders-service                                         │ │
│  │           │                                                │ │
│  │           ▼                                                │ │
│  │     payments-api ─────► orders-db (RDS)                    │ │
│  │           │                                                │ │
│  │           ▼                                                │ │
│  │     notification-svc ─────► sqs-queue                      │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## Development Phases

### Phase 1: Operator Foundation
**Goal:** Build core Kubernetes operator with basic CRD and K8s resource generation.

| Milestone | Description | Key Deliverables |
|-----------|-------------|------------------|
| M1 | Project Setup & CRD Design | kubebuilder scaffolding, Application CRD schema, validation webhooks |
| M2 | Basic Controller Reconciliation | Watch Applications, create Deployments, handle create/update/delete |
| M3 | Kubernetes Resource Generation | Service, ConfigMap, HPA, PDB generation from Application spec |
| M4 | Status Management & Conditions | Status subresource, conditions (Ready, Progressing, Degraded) |
| M5 | Finalizers & Cleanup | Safe deletion with finalizers, cascading cleanup |

### Phase 2: Terraform Integration
**Goal:** Provision AWS infrastructure via Terraform from within the operator.

| Milestone | Description | Key Deliverables |
|-----------|-------------|------------------|
| M6 | Terraform Runner Basics | Call Terraform CLI from Go, init/plan/apply workflow |
| M7 | State Management | Per-app state isolation, S3 backend, DynamoDB locking |
| M8 | RDS Module | Generate RDS Terraform module, provision PostgreSQL |
| M9 | ElastiCache Module | Generate ElastiCache module, provision Redis |
| M10 | SQS Module | Generate SQS module with DLQ configuration |
| M11 | IAM & IRSA | Generate IAM roles, service account annotations |

### Phase 3: Platform API & CLI
**Goal:** REST API and CLI tool for platform interaction beyond kubectl.

| Milestone | Description | Key Deliverables |
|-----------|-------------|------------------|
| M12 | Platform API Server | REST API for app listing, status, provisioning |
| M13 | CLI Tool (gpctl) | gpctl apply, status, logs, delete commands |
| M14 | Authentication | API keys, JWT tokens, RBAC integration |
| M15 | Webhook Events | Notify external systems on app lifecycle events |

### Phase 4: Observability
**Goal:** Auto-generate monitoring, alerting, and dashboards for every application.

| Milestone | Description | Key Deliverables |
|-----------|-------------|------------------|
| M16 | ServiceMonitor Generation | Prometheus scrape configs per app |
| M17 | Grafana Dashboard Generation | Auto-generated dashboards based on app type |
| M18 | AlertRule Generation | SLA-based alerts (latency, error rate, availability) |
| M19 | Distributed Tracing | OpenTelemetry configuration injection |

### Phase 5: Service Catalog
**Goal:** Track all applications, dependencies, and provide a software catalog.

| Milestone | Description | Key Deliverables |
|-----------|-------------|------------------|
| M20 | Catalog CRD & Controller | ServiceCatalog CRD, track all apps |
| M21 | Dependency Tracking | Infer and store dependencies between services |
| M22 | Team Ownership | Track team ownership, enable team-based views |
| M23 | Compliance Checks | Validate apps meet platform policies |

### Phase 6: Advanced Features
**Goal:** Production hardening and advanced platform capabilities.

| Milestone | Description | Key Deliverables |
|-----------|-------------|------------------|
| M24 | Multi-Environment | Dev/staging/prod environment support |
| M25 | GitOps Integration | ArgoCD ApplicationSet integration |
| M26 | Cost Tracking | Tag AWS resources, aggregate costs per team |
| M27 | Secrets Management | Integration with AWS Secrets Manager or external-secrets |
| M28 | Production Hardening | Rate limiting, audit logging, metrics |

---

## Milestone Details

### Milestone 1: Project Setup & CRD Design

**Goal:** Set up the operator project structure and design the core Application CRD.

**Learning Focus:**
- How kubebuilder scaffolds operators
- CRD schema design with OpenAPI validation
- Why structural schemas matter for Kubernetes
- Admission webhooks for complex validation

**Deliverables:**
- [ ] kubebuilder project initialization
- [ ] Application CRD with comprehensive spec
- [ ] Validation webhook for Application
- [ ] Default values webhook
- [ ] CRD installation via Helm/Kustomize
- [ ] Basic unit tests for CRD

**CRD Design:**
```yaml
apiVersion: platform.goplatform.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  # Team ownership
  team: payments
  
  # Workload configuration
  replicas: 3
  image: ghcr.io/org/my-app:v1.0.0
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 1
      memory: 1Gi
  
  # Infrastructure dependencies
  database:
    type: postgres
    version: "15"
    size: small  # small/medium/large → maps to RDS instance types
    backup:
      enabled: true
      retentionDays: 7
  
  cache:
    type: redis
    size: small
  
  queue:
    type: sqs
    fifo: false
  
  # Observability
  observability:
    metrics:
      enabled: true
      port: 9090
      path: /metrics
    tracing:
      enabled: true
      sampleRate: 0.1

status:
  phase: Ready  # Pending/Provisioning/Ready/Failed
  conditions:
    - type: KubernetesReady
      status: "True"
    - type: DatabaseReady
      status: "True"
    - type: CacheReady
      status: "True"
  database:
    endpoint: my-app-db.xxx.us-east-1.rds.amazonaws.com
    port: 5432
  cache:
    endpoint: my-app-cache.xxx.cache.amazonaws.com
    port: 6379
```

---

### Milestone 2: Basic Controller Reconciliation

**Goal:** Implement the core reconciliation loop that watches Applications and creates Kubernetes resources.

**Learning Focus:**
- Controller-runtime architecture (informers, work queues)
- Reconciliation pattern (level-triggered)
- Idempotent operations
- Error handling and requeueing

**Deliverables:**
- [ ] ApplicationReconciler implementation
- [ ] Create Deployment from Application spec
- [ ] Handle create/update/delete events
- [ ] Proper logging and error handling
- [ ] Requeue on transient failures
- [ ] Unit tests with envtest

**Key Concepts:**
```
Reconciliation Flow:
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ User applies │────►│ Informer     │────►│ Work Queue   │
│ Application  │     │ sees change  │     │ (rate        │
│              │     │              │     │  limited)    │
└──────────────┘     └──────────────┘     └──────┬───────┘
                                                  │
                                                  ▼
                     ┌──────────────┐     ┌──────────────┐
                     │ Update       │◄────│ Reconcile()  │
                     │ status       │     │ - Get App    │
                     └──────────────┘     │ - Compare    │
                                          │ - Create/Upd │
                                          └──────────────┘
```

---

### Milestone 6: Terraform Runner Basics

**Goal:** Execute Terraform from within the controller to provision AWS resources.

**Learning Focus:**
- Calling external processes from Go
- Terraform CLI workflow (init/plan/apply)
- Parsing Terraform output
- Error handling for infrastructure failures

**Deliverables:**
- [ ] TerraformRunner struct with CLI wrapper
- [ ] HCL module generation from Go
- [ ] Init/Plan/Apply workflow
- [ ] Output parsing (endpoints, credentials)
- [ ] Destroy for cleanup
- [ ] Integration tests with localstack

**Key Concepts:**
```
Terraform Execution Flow:
┌───────────────────────────────────────────────────────────────┐
│                    TerraformRunner                            │
│                                                               │
│  1. Generate Module          2. Init           3. Plan        │
│  ┌─────────────────┐     ┌─────────────┐    ┌─────────────┐   │
│  │ app.Spec.DB     │────►│ terraform   │───►│ terraform   │   │
│  │ → main.tf       │     │ init        │    │ plan        │   │
│  │ → variables.tf  │     │             │    │ -out=plan   │   │
│  └─────────────────┘     └─────────────┘    └──────┬──────┘   │
│                                                     │         │
│  4. Apply                 5. Parse Output           │         │
│  ┌─────────────────┐     ┌─────────────┐    ┌──────▼──────┐   │
│  │ terraform       │────►│ Extract     │◄───│ Read        │   │
│  │ apply plan      │     │ - endpoint  │    │ terraform   │   │
│  │                 │     │ - port      │    │ output      │   │
│  └─────────────────┘     │ - creds     │    └─────────────┘   │
│                          └─────────────┘                      │
└───────────────────────────────────────────────────────────────┘
```

---

## Quick Start (After Phase 1)

```bash
# Install CRDs
kubectl apply -f deploy/kubernetes/crds/

# Install operator
helm install goplatform deploy/helm/goplatform

# Create an application
kubectl apply -f - <<EOF
apiVersion: platform.goplatform.io/v1alpha1
kind: Application
metadata:
  name: demo-app
spec:
  team: demo
  replicas: 2
  image: nginx:latest
  database:
    type: postgres
    size: small
EOF

# Check status
kubectl get applications
kubectl describe application demo-app

# Use CLI
gpctl status demo-app
gpctl logs demo-app
```

---

## Project Structure

```
goplatform/
├── .github/
│   ├── instructions/           # AI instruction files
│   ├── workflows/              # CI/CD workflows
│   └── copilot-instructions.md
├── memory-bank/                # Project memory for AI sessions
│   ├── projectbrief.md
│   ├── productContext.md
│   ├── activeContext.md
│   ├── systemPatterns.md
│   ├── techContext.md
│   ├── progress.md
│   └── tasks/
├── cmd/
│   ├── goplatform/             # Operator binary
│   └── gpctl/                  # CLI tool
├── internal/
│   ├── controller/             # Kubernetes controllers
│   ├── api/                    # REST API server
│   ├── terraform/              # Terraform runner
│   ├── catalog/                # Service catalog
│   ├── observability/          # Prometheus/Grafana integration
│   └── config/                 # Configuration management
├── pkg/
│   └── apis/
│       └── platform/
│           └── v1alpha1/       # CRD types
├── deploy/
│   ├── helm/
│   │   └── goplatform/         # Helm chart
│   ├── kubernetes/             # Raw K8s manifests
│   └── terraform/
│       └── modules/            # Terraform modules for AWS
├── docs/
│   └── architecture/           # Architecture docs
├── PROGRESS.md                 # Development progress
├── ROADMAP.md                  # Feature roadmap
└── README.md
```

---

## Comparison with Existing Solutions

| Feature | GoPlatform | Backstage | Crossplane | Terraform Cloud |
|---------|------------|-----------|------------|-----------------|
| K8s Native | ✅ CRD-based | ❌ Separate app | ✅ CRD-based | ❌ SaaS |
| Self-Hosted | ✅ | ✅ | ✅ | ❌ (or TFE) |
| K8s + Cloud | ✅ Both | ❌ Catalog only | ✅ Both | ❌ Cloud only |
| Terraform Modules | ✅ Reuse existing | ❌ N/A | ❌ Own providers | ✅ Native |
| Service Catalog | ✅ Built-in | ✅ Core feature | ❌ No | ❌ No |
| Complexity | Medium | High | High | Low |

---

## Tech Stack

- **Language:** Go 1.22+
- **Operator Framework:** controller-runtime, kubebuilder
- **CLI:** cobra, viper
- **API:** chi or gin
- **Infrastructure:** Terraform, AWS SDK
- **Observability:** Prometheus, Grafana, OpenTelemetry

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

**Building platforms to empower developers.**
