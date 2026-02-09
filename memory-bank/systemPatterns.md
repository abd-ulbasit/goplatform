# System Patterns

## Architecture Overview

GoPlatform follows a layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Developer Interface                          │
│  kubectl apply | gpctl CLI | REST API | GitOps                     │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Kubernetes API Server                          │
│  Application CRD | Database CRD | Cache CRD                        │
└─────────────────────────────────────────────────────────────────────┘
                                │ watch
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    GoPlatform Controller                            │
│  Reconcilers | Terraform Runner | Status Manager                    │
└─────────────────────────────────────────────────────────────────────┘
                                │
                ┌───────────────┼───────────────┐
                ▼               ▼               ▼
┌─────────────────────┐ ┌─────────────┐ ┌──────────────────┐
│ Kubernetes Resources│ │ AWS Infra   │ │ Observability    │
│ Deployment, Service │ │ via Terraform│ │ Prometheus, etc  │
└─────────────────────┘ └─────────────┘ └──────────────────┘
```

## Core Patterns

### 1. Controller Reconciliation Pattern
**Status:** To be implemented in M2

The heart of Kubernetes operators - continuously reconcile actual state to desired state.

```
Watch Event → Work Queue → Reconcile() → Compare State → Act → Update Status
     │                          │                              │
     │                          │                              │
     │                     idempotent                          │
     │                     operations                     requeue if error
     │                                                         │
     └─────────────────────────────────────────────────────────┘
```

### 2. Finalizer Pattern
**Status:** To be implemented in M5

Prevent deletion until cleanup is complete.

```
Create Application → Add Finalizer
Delete Application → Detect deletionTimestamp → Cleanup → Remove Finalizer → Actually Delete
```

### 3. Terraform State Isolation
**Status:** To be implemented in M7

Each application gets isolated Terraform state.

```
State Key: s3://bucket/apps/{namespace}/{name}/terraform.tfstate
Lock: DynamoDB item per state key
```

### 4. Status Conditions
**Status:** To be implemented in M4

Report status using Kubernetes conventions.

```yaml
status:
  conditions:
    - type: Ready
      status: "True"
      reason: AllResourcesReady
      message: "Application is fully provisioned"
    - type: DatabaseReady
      status: "True"
      reason: RDSProvisioned
      message: "RDS instance is available"
```

## Component Relationships

### Controller → InfrastructureProvider
- Controller calls InfrastructureProvider interface for cloud resources
- Provider returns endpoints, credentials, and status
- Controller stores outputs in status and K8s Secrets

### Controller → Terraform Runner (AWS Provider)
- TerraformRunner executes terraform CLI in subprocess
- Manages per-application state in S3
- Uses DynamoDB for locking

### Controller → Kubernetes Resources
- Controller creates/updates K8s resources
- Uses owner references for garbage collection
- Updates status based on resource state

### Credential Flow
```
Terraform provisions RDS → Outputs to TerraformRunner
    ↓
TerraformRunner parses outputs → Returns to Controller
    ↓
Controller creates K8s Secret (or ExternalSecret for ESO)
    ↓
Deployment mounts Secret as environment variables
    ↓
Application reads DATABASE_URL from env
```

### Service Catalog → Applications
- Catalog watches all Application CRDs
- Builds dependency graph
- Tracks team ownership

## Design Decisions (Session 2)

| Pattern | Decision | Reasoning |
|---------|----------|-----------|
| Cloud Abstraction | InfrastructureProvider interface | Simpler than Crossplane XRDs, extensible |
| Credential Passing | K8s Secrets + ESO support | Simple default, production-ready option |
| RBAC | K8s RBAC + policies → platform-level later | Don't reinvent, add when needed |
| Terraform State | S3 + DynamoDB per-app isolation | Standard pattern, proven |
| Local K8s | Colima with K8s 1.33+ | Good macOS support, avoid extended support costs |

## Infrastructure Provider Pattern

```go
// Adapter pattern - controller doesn't know which cloud
type InfrastructureProvider interface {
    ProvisionDatabase(ctx, app, spec) → (DatabaseStatus, error)
    ProvisionCache(ctx, app, spec) → (CacheStatus, error)
    ProvisionQueue(ctx, app, spec) → (QueueStatus, error)
    Destroy(ctx, app) → error
    GetStatus(ctx, app) → (InfrastructureStatus, error)
    EstimateCost(ctx, app) → (CostEstimate, error)
}

// Implementations
AWSProvider    → Terraform + RDS/ElastiCache/SQS
GCPProvider    → Terraform + CloudSQL/Memorystore (future)
LocalProvider  → CloudNativePG/Redis operators (for dev/preview)
MockProvider   → For testing
```

## Anti-Patterns to Avoid

1. **Polling Instead of Watching** - Always use informers
2. **Non-Idempotent Reconciliation** - Every reconcile must be safe to repeat
3. **Blocking Operations** - Use goroutines for long-running tasks
4. **Missing Status Updates** - Always report current state
5. **Orphaned Resources** - Always use finalizers for external resources
6. **Hardcoded Cloud Logic** - Use provider interface for abstraction
7. **Secrets in Status** - Use Secret references, not values
