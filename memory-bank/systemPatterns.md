# System Patterns

## Layered Architecture with Pluggable Interfaces

GoPlatform is designed as a **layered platform** where each layer can be replaced independently. This enables users to swap implementations without changing the layers above.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         LAYER 1: DEVELOPER INTERFACE                        │
│  kubectl apply | gpctl CLI | REST API | GitOps | Developer Portal           │
│                                                                             │
│  Users interact here. Abstracts away all infrastructure complexity.         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      LAYER 2: APPLICATION CRD (GoPlatform)                  │
│  Application, Database, Cache, Queue - cloud-agnostic specifications        │
│                                                                             │
│  WHAT IT DOES: Defines "I want a database" without saying "RDS" or "GCP"    │
│  INTERFACE: Kubernetes CRD (Application, etc.)                              │
│  PLUGGABLE: No - this is the GoPlatform core abstraction                    │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    LAYER 3: ORCHESTRATION CONTROLLER                        │
│  Watches CRDs, orchestrates provisioning, manages lifecycle                 │
│                                                                             │
│  WHAT IT DOES: Translates Application spec → commands to Layer 4            │
│  INTERFACE: InfrastructureProvider Go interface                             │
│  PLUGGABLE: Yes - could be our controller or a different orchestrator       │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    LAYER 4: INFRASTRUCTURE PROVIDER                         │
│  Actually provisions cloud resources                                        │
│                                                                             │
│  OPTIONS (plug-and-play):                                                   │
│  ┌───────────────────┬─────────────────────────────────────────────────────┐│
│  │ TerraformProvider │ Default. Uses Terraform + AWS/GCP/Azure modules     ││
│  │ CrossplaneProvider│ Alternative. Delegate to Crossplane XRDs            ││
│  │ PulumiProvider    │ Alternative. Uses Pulumi for provisioning           ││
│  │ LocalProvider     │ Dev/test. CloudNativePG, Redis operator             ││
│  │ MockProvider      │ Testing. Returns fake endpoints                     ││
│  └───────────────────┴─────────────────────────────────────────────────────┘│
│                                                                             │
│  INTERFACE: InfrastructureProvider Go interface defined below               │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        LAYER 5: CLOUD RESOURCES                             │
│  AWS RDS | GCP Cloud SQL | Azure DB | LocalStack | CloudNativePG            │
│                                                                             │
│  Actual cloud-managed or local resources                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Why This Matters

1. **Crossplane Integration**: User wants Crossplane? Implement `CrossplaneProvider` that creates Crossplane `Claim` CRDs instead of running Terraform.

2. **Multi-Cloud**: Each cloud gets its own provider implementation. Controller doesn't change.

3. **Testing**: Use `MockProvider` in tests without needing real cloud resources.

4. **Gradual Migration**: Start with TerraformProvider, migrate to CrossplaneProvider later without changing Applications.

5. **Vendor Lock-In Avoidance**: Application CRDs are portable. Only providers are cloud-specific.

---

## Architecture Overview

GoPlatform follows a layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Developer Interface                          │
│  kubectl apply | gpctl CLI | REST API | GitOps                      │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Kubernetes API Server                          │
│  Application CRD | Database CRD | Cache CRD                         │
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
│ Deployment, Service │ │ via Terraform│ │ Prometheus, etc │
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

## Infrastructure Provider Pattern (Pluggable Layer 4)

```go
// =============================================================================
// INFRASTRUCTURE PROVIDER INTERFACE
// =============================================================================
// This is the pluggable abstraction for Layer 4.
// The controller calls these methods without knowing the implementation.

type InfrastructureProvider interface {
    ProvisionDatabase(ctx context.Context, app *Application, spec *DatabaseSpec) (*DatabaseStatus, error)
    ProvisionCache(ctx context.Context, app *Application, spec *CacheSpec) (*CacheStatus, error)
    ProvisionQueue(ctx context.Context, app *Application, spec *QueueSpec) (*QueueStatus, error)
    ProvisionStorage(ctx context.Context, app *Application, spec *StorageSpec) (*StorageStatus, error)
    Destroy(ctx context.Context, app *Application) error
    GetStatus(ctx context.Context, app *Application) (*InfrastructureStatus, error)
    EstimateCost(ctx context.Context, app *Application) (*CostEstimate, error)
}
```

### Provider Implementations

| Provider | Backend | Use Case |
|----------|---------|----------|
| `TerraformProvider` | Terraform + AWS/GCP modules | Production default |
| `CrossplaneProvider` | Crossplane XRDs | Alternative - use existing Crossplane |
| `PulumiProvider` | Pulumi | Alternative - code-based IaC |
| `LocalProvider` | CloudNativePG, Redis operator | Local dev, preview envs |
| `MockProvider` | In-memory fake | Unit/integration testing |

### Why Pluggable

- **Crossplane users**: Implement `CrossplaneProvider` that creates Crossplane Claims
- **Multi-cloud**: Each cloud can have its own provider
- **Testing**: MockProvider returns fake endpoints instantly
- **Migration**: Switch from Terraform to Crossplane without changing Applications

## Anti-Patterns to Avoid

1. **Polling Instead of Watching** - Always use informers
2. **Non-Idempotent Reconciliation** - Every reconcile must be safe to repeat
3. **Blocking Operations** - Use goroutines for long-running tasks
4. **Missing Status Updates** - Always report current state
5. **Orphaned Resources** - Always use finalizers for external resources
6. **Hardcoded Cloud Logic** - Use provider interface for abstraction
7. **Secrets in Status** - Use Secret references, not values
