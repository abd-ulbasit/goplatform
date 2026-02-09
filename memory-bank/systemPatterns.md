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

### Controller → Terraform Runner
- Controller calls TerraformRunner for AWS resources
- TerraformRunner returns outputs (endpoints, credentials)
- Controller stores outputs in status and secrets

### Controller → Kubernetes Resources
- Controller creates/updates K8s resources
- Uses owner references for garbage collection
- Updates status based on resource state

### Service Catalog → Applications
- Catalog watches all Application CRDs
- Builds dependency graph
- Tracks team ownership

## Design Decisions

_To be filled as decisions are made during development_

| Pattern | Decision | Reasoning |
|---------|----------|-----------|
| | | |

## Anti-Patterns to Avoid

1. **Polling Instead of Watching** - Always use informers
2. **Non-Idempotent Reconciliation** - Every reconcile must be safe to repeat
3. **Blocking Operations** - Use goroutines for long-running tasks
4. **Missing Status Updates** - Always report current state
5. **Orphaned Resources** - Always use finalizers for external resources
