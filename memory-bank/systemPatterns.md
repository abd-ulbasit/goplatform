# System Patterns

## Architecture

GoPlatform follows a layered architecture with pluggable infrastructure providers:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         LAYER 1: DEVELOPER INTERFACE                        │
│  kubectl apply                                                              │
│  Users interact here. Abstracts away all infrastructure complexity.         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      LAYER 2: APPLICATION CRD                               │
│  Application CRD - cloud-agnostic specifications                           │
│  Defines "I want a database" without saying "CNPG" or "RDS"               │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    LAYER 3: ORCHESTRATION CONTROLLER                        │
│  ApplicationReconciler - watches CRDs, orchestrates provisioning            │
│  Translates Application spec → commands to Layer 4                         │
│  INTERFACE: InfrastructureProvider Go interface                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    LAYER 4: INFRASTRUCTURE PROVIDER                         │
│  ┌───────────────────────┬────────────────────────────────────────────────┐ │
│  │ KubernetesProvider    │ CNPG, Redis Operator, RabbitMQ Operator       │ │
│  │ MockProvider          │ Testing - returns fake endpoints              │ │
│  │ (Future providers)    │ Interface designed for extensibility           │ │
│  └───────────────────────┴────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        LAYER 5: ACTUAL RESOURCES                            │
│  CloudNativePG Cluster | RedisFailover | RabbitmqCluster | PVCs            │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Why This Matters

1. **Testing**: MockProvider returns fake endpoints instantly
2. **Extensibility**: Future cloud providers implement the same interface
3. **Separation of concerns**: Controller doesn't know about CNPG internals

---

## Core Patterns

### 1. Controller Reconciliation Pattern (M2)
Level-triggered, idempotent state management.

```
Watch Event → Work Queue → Reconcile() → Compare State → Act → Update Status
     │                          │                              │
     │                     idempotent                     requeue if error
     │                     operations                          │
     └─────────────────────────────────────────────────────────┘
```

### 2. Finalizer Pattern (M4)
Blocks deletion until infrastructure cleanup is complete.

```
Create Application → Add Finalizer
Delete Application → Detect deletionTimestamp → Destroy() → Remove Finalizer → Actually Delete
```

### 3. Status Conditions (M4)
Multi-dimensional status using K8s conventions.

```yaml
status:
  phase: Ready
  conditions:
    - type: Ready
      status: "True"
      reason: AllResourcesReady
    - type: DatabaseReady
      status: "True"
    - type: CacheReady
      status: "True"
```

### 4. Provider Interface (M5)
Pluggable infrastructure provisioning.

```go
type InfrastructureProvider interface {
    Provision(ctx context.Context, app *Application) (*ResourceState, error)
    GetStatus(ctx context.Context, app *Application) (*ResourceState, error)
    Destroy(ctx context.Context, app *Application) error
    Name() string
    Type() ProviderType
    Healthy(ctx context.Context) bool
}
```

### 5. Drift Detection (M9 - planned)
Watches owned resources and restores them when externally modified.

```
Owned resource modified → Watch triggers reconcile → CreateOrUpdate overwrites → Event recorded
Owned resource deleted → Watch triggers reconcile → CreateOrUpdate recreates → Event recorded
```

### 6. Admission Webhooks (M7 - planned)
Intercept API requests before objects are stored.

```
kubectl apply → API Server → Mutating Webhook (inject defaults) → Validating Webhook (reject invalid) → etcd
```

---

## Component Relationships

### Controller → InfrastructureProvider
- Controller calls provider interface for infrastructure resources
- Provider returns ResourceState with endpoints, credentials, and status
- Controller maps ResourceState to Application conditions

### Controller → Kubernetes Resources
- Controller creates/updates K8s resources with CreateOrUpdate
- Uses owner references for garbage collection
- Updates status based on resource state

### Credential Flow
```
Provider creates operator CRD (e.g., CNPG Cluster)
    ↓
Provider creates K8s Secret with connection strings
    ↓
Deployment can mount Secret as environment variables
    ↓
Application reads DATABASE_URL from env
```

---

## Design Decisions

| Pattern | Decision | Reasoning |
|---------|----------|-----------|
| Cloud Abstraction | InfrastructureProvider interface | Simpler than Crossplane XRDs, extensible |
| Infrastructure | K8s-native operators (CNPG, Redis, RabbitMQ) | Aligned with K8s learning goal |
| Developer Interface | kubectl only | Focus on operator patterns, not CLI/API |
| Credential Passing | K8s Secrets | Simple, works everywhere |
| Policy Engine | Kyverno integration | Don't reinvent, learn the ecosystem |
| Local K8s | Kind (testing) + Colima (dev) | Kind for CI, Colima for daily dev |

---

## Anti-Patterns to Avoid

1. **Polling Instead of Watching** - Always use informers
2. **Non-Idempotent Reconciliation** - Every reconcile must be safe to repeat
3. **Blocking Operations** - Use goroutines for long-running tasks
4. **Missing Status Updates** - Always report current state
5. **Orphaned Resources** - Always use finalizers for external resources
6. **Hardcoded Provider Logic** - Use provider interface for abstraction
7. **Secrets in Status** - Use Secret references, not values
