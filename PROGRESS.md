# GoPlatform Development Progress

## Status: Phase 1 - Operator Foundation

**Target Milestones**: 35  
**Completed**: 2  
**Current**: Milestone 2 Complete - Ready for Milestone 3

---

## Phase Overview

| Phase | Description | Milestones | Status |
|-------|-------------|------------|--------|
| Phase 1 | Operator Foundation | M1-M5 | � In Progress |
| Phase 2 | Infrastructure Providers | M6-M12 | 📋 Planned |
| Phase 3 | Credential Management | M13-M15 | 📋 Planned |
| Phase 4 | Platform API & CLI | M16-M19 | 📋 Planned |
| Phase 5 | Observability | M20-M23 | 📋 Planned |
| Phase 6 | Service Catalog | M24-M27 | 📋 Planned |
| Phase 7 | Developer Experience | M28-M31 | 📋 Planned |
| Phase 8 | Production Hardening | M32-M35 | 📋 Planned |

---

## Unique Platform Features (Competitive Differentiators)

These features distinguish GoPlatform from Backstage, Crossplane, and Terraform Cloud:

| Feature | Phase | Description | Why It Matters |
|---------|-------|-------------|----------------|
| **Cost Estimation** | M17 | Show monthly cost before provisioning | No billing surprises |
| **Preview Environments** | M29 | Auto-create full stack for each PR | DX game-changer |
| **Environment Promotion** | M28 | Promote config dev→staging→prod | Golden path |
| **Drift Detection** | M31 | Detect when infra diverges from CRD | Self-healing |
| **Dependency Graph** | M25 | Visual service dependency graph | Impact awareness |
| **Team Budgets** | M33 | Cost controls per team | FinOps built-in |
| **Secrets Rotation** | M15 | Auto-rotate database passwords | Security by default |
| **Local Development** | M30 | Run same stack locally | True parity |
| **Audit Trail** | M34 | Who provisioned what, when | Compliance |
| **Resource Templates** | M27 | Pre-built blueprints | Faster onboarding |

---

## Phase 1: Operator Foundation

### Milestone 1: Project Setup & CRD Design - ✅ COMPLETED

**Goal:** Set up the operator project structure and design the core Application CRD.

**Learning Focus:**
- How kubebuilder scaffolds operators
- CRD schema design with OpenAPI validation
- Why structural schemas matter for Kubernetes
- Admission webhooks for complex validation

**Concepts to Understand:**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         KUBEBUILDER PROJECT STRUCTURE                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  api/                       # CRD type definitions (Go structs)             │
│  └── v1alpha1/              # Version directory                             │
│      ├── application_types.go    # Application struct + markers             │
│      ├── groupversion_info.go    # API group registration                   │
│      └── zz_generated.deepcopy.go  # Auto-generated (make generate)         │
│                                                                             │
│  internal/controller/       # Reconciliation logic                          │
│  └── application_controller.go   # Reconciler implementation                │
│                                                                             │
│  config/                    # Generated Kubernetes manifests                │
│  ├── crd/                   # CRD YAML (make manifests)                     │
│  ├── rbac/                  # RBAC for controller                           │
│  └── webhook/               # Webhook configuration                         │
│                                                                             │
│  WHY THIS STRUCTURE:                                                        │
│  - Follows controller-runtime conventions                                   │
│  - Code generation expects specific paths                                   │
│  - Same pattern as Kubernetes itself                                        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Deliverables:**
- [ ] kubebuilder project initialization with domain `platform.goplatform.io`
- [ ] Application CRD v1alpha1 type definitions
- [ ] OpenAPI structural schema with validations
- [ ] Validation webhook for complex cross-field validation
- [ ] Defaulting webhook for sensible defaults
- [ ] Generated CRD YAML and RBAC manifests
- [ ] Basic unit tests for type conversions
- [ ] Helm chart for CRD installation

**CRD Design (Cloud-Agnostic):**
```yaml
apiVersion: platform.goplatform.io/v1alpha1
kind: Application
metadata:
  name: payments-api
  namespace: default
spec:
  # ============================================================
  # OWNERSHIP - who owns this service
  # ============================================================
  team: payments
  owner: alice@company.com
  tier: critical  # critical/standard/development → affects SLAs

  # ============================================================
  # WORKLOAD - what to deploy
  # ============================================================
  workload:
    image: ghcr.io/company/payments-api:v1.0.0
    replicas: 3
    resources:
      requests:
        cpu: 500m
        memory: 512Mi
      limits:
        cpu: 1
        memory: 1Gi
    ports:
      - name: http
        containerPort: 8080
      - name: metrics
        containerPort: 9090
    healthCheck:
      path: /health
      port: 8080
    
  # ============================================================
  # SCALING - how to scale
  # ============================================================
  scaling:
    minReplicas: 2
    maxReplicas: 10
    metrics:
      - type: cpu
        target: 70
      - type: memory
        target: 80
  
  # ============================================================
  # INFRASTRUCTURE - cloud-agnostic resource requests
  # ============================================================
  # The platform maps these to provider-specific resources
  # (AWS RDS, GCP Cloud SQL, local CloudNativePG, etc.)
  
  database:
    type: postgres               # postgres, mysql
    size: small                  # small/medium/large → platform interprets
    version: "15"                # Major version only
    highAvailability: true       # Multi-AZ/replicas
    backup:
      enabled: true
      retentionDays: 7
      window: "03:00-04:00"
    
  cache:
    type: redis                  # redis, memcached
    size: small
    highAvailability: true
    
  queue:
    type: sqs                    # sqs, rabbitmq, kafka
    fifo: false
    deadLetterQueue:
      enabled: true
      maxReceiveCount: 5
  
  storage:
    type: s3                     # s3, gcs
    versioning: true
    encryption: true
  
  # ============================================================
  # OBSERVABILITY - monitoring configuration
  # ============================================================
  observability:
    metrics:
      enabled: true
      path: /metrics
      port: 9090
    tracing:
      enabled: true
      sampleRate: 0.1
    logging:
      format: json               # json, logfmt
  
  # ============================================================
  # DEPENDENCIES - what this service depends on
  # ============================================================
  dependencies:
    - name: orders-api           # Service name
      namespace: default         # Optional, defaults to same namespace
      required: true             # Block startup if unavailable
    - name: notification-svc
      required: false

status:
  phase: Ready  # Pending/Provisioning/Ready/Failed/Deleting
  observedGeneration: 1
  
  conditions:
    - type: Ready
      status: "True"
      reason: AllResourcesProvisioned
      message: "All resources are ready"
      lastTransitionTime: "2025-02-09T10:00:00Z"
    - type: WorkloadReady
      status: "True"
    - type: DatabaseReady  
      status: "True"
    - type: CacheReady
      status: "True"
  
  # Infrastructure endpoints (populated after provisioning)
  infrastructure:
    database:
      endpoint: payments-api-db.xxx.us-east-1.rds.amazonaws.com
      port: 5432
      secretRef: 
        name: payments-api-database-credentials
    cache:
      endpoint: payments-api-cache.xxx.cache.amazonaws.com
      port: 6379
    queue:
      url: https://sqs.us-east-1.amazonaws.com/123456789/payments-api-queue
      arn: arn:aws:sqs:us-east-1:123456789:payments-api-queue
  
  # Cost estimation
  estimatedMonthlyCost:
    amount: "245.50"
    currency: USD
    breakdown:
      database: "180.00"
      cache: "45.50"
      queue: "20.00"
```

**Key Patterns:**
- Cloud-agnostic spec (no AWS-specific fields)
- Size abstraction (small/medium/large → provider maps to instance types)
- Status reports infrastructure endpoints
- Owner references for garbage collection
- Conditions for fine-grained status

---

### Milestone 2: Basic Controller Reconciliation - ✅ COMPLETED

**Goal:** Implement the core reconciliation loop that watches Applications and creates Kubernetes resources.

**Learning Focus:**
- Controller-runtime architecture (informers, work queues)
- Reconciliation pattern (level-triggered vs edge-triggered)
- Idempotent operations
- Error handling and requeueing

**Concepts to Understand:**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      KUBERNETES CONTROLLER ARCHITECTURE                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                             INFORMER                                    ││
│  │  ┌───────────────┐    ┌───────────────┐    ┌───────────────┐            ││
│  │  │   Reflector   │───►│   DeltaFIFO   │───►│   Indexer     │            ││
│  │  │               │    │   (queue)     │    │   (cache)     │            ││
│  │  │ - List+Watch  │    │ - Add/Update  │    │ - Local copy  │            ││
│  │  │ - From API    │    │   /Delete     │    │   of objects  │            ││
│  │  │   server      │    │ - Sync        │    │               │            ││
│  │  └───────────────┘    └───────┬───────┘    └───────────────┘            ││
│  │                                │                     ▲                  ││
│  │                                │  events             │ read cache       ││
│  │                                ▼                     │                  ││
│  │                       ┌───────────────┐              │                  ││
│  │                       │   Handler     │──────────────┘                  ││
│  │                       │ (filter/map)  │                                 ││
│  │                       └───────┬───────┘                                 ││
│  │                                │                                        ││
│  └────────────────────────────────┼────────────────────────────────────────┘│
│                                   ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                           WORK QUEUE                                    ││
│  │                                                                         ││
│  │  Features:                                                              ││
│  │  - Rate limiting (prevents hammering API)                               ││
│  │  - Exponential backoff (on errors)                                      ││
│  │  - Deduplication (multiple events → one reconcile)                      ││
│  │  - Fair queuing (no object starves)                                     ││
│  │                                                                         ││
│  │  ┌─────┬─────┬─────┬─────┬─────┐                                        ││
│  │  │ A/1 │ B/2 │ C/1 │ A/1 │ D/5 │ ──► deduped to [A/1, B/2, C/1, D/5]    ││
│  │  └─────┴─────┴─────┴─────┴─────┘                                        ││
│  │                                                                         ││
│  └─────────────────────────────────┬───────────────────────────────────────┘│
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                           RECONCILER                                    ││
│  │                                                                         ││
│  │  Reconcile(ctx, Request{Name, Namespace})                               ││
│  │    │                                                                    ││
│  │    ├─► Get current state (from cache)                                   ││
│  │    │                                                                    ││
│  │    ├─► Compare to desired state (spec)                                  ││
│  │    │                                                                    ││
│  │    ├─► Take action (create/update/delete resources)                     ││
│  │    │                                                                    ││
│  │    ├─► Update status                                                    ││
│  │    │                                                                    ││
│  │    └─► Return Result                                                    ││
│  │          - Result{} = done, don't requeue                               ││
│  │          - Result{Requeue: true} = requeue immediately                  ││
│  │          - Result{RequeueAfter: 5m} = requeue in 5 minutes              ││
│  │          - error = requeue with backoff                                 ││
│  │                                                                         ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  WHY LEVEL-TRIGGERED (not edge-triggered):                                  │
│  - Edge: "Something changed" → might miss events, need complex logic        │
│  - Level: "Make actual = desired" → idempotent, self-healing                │
│  - If reconcile fails, just retry - eventual consistency                    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Deliverables:**
- [x] ApplicationReconciler struct with controller-runtime setup
- [x] SetupWithManager() with watches and predicates
- [x] Basic reconcile loop structure
- [x] Create Deployment from Application spec
- [x] Apply labels (app, team, managed-by)
- [x] Handle create/update/delete events (with finalizers)
- [x] Proper structured logging (controller-runtime log)
- [x] Error handling with requeue strategies
- [x] Unit tests with envtest (66.9% coverage)
- [ ] Integration tests with local cluster (deferred to later)

**Key Patterns Implemented:**
- Reconciler is idempotent (running twice = same result)
- Level-triggered reconciliation (compare and sync, not event-driven)
- Finalizer pattern for cleanup before deletion
- Owner references for garbage collection
- Status conditions for granular readiness reporting

---

### Milestone 3: Kubernetes Resource Generation - NOT STARTED

**Goal:** Generate all necessary Kubernetes resources from Application spec.

**Learning Focus:**
- Building K8s resources programmatically with Go client types
- Owner references for garbage collection
- ConfigMap and Secret generation patterns
- HPA and PDB for production readiness

**Concepts to Understand:**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         OWNER REFERENCES & GARBAGE COLLECTION               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  WHY: When Application is deleted, we want all created resources to be      │
│  automatically cleaned up. Kubernetes has built-in support for this via     │
│  owner references + garbage collector.                                      │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                        Application: payments-api                        ││
│  │                      (owner, controller: true)                          ││
│  │                                │                                        ││
│  │           ┌────────────────────┼────────────────────┐                   ││
│  │           ▼                    ▼                    ▼                   ││
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐          ││
│  │  │   Deployment    │  │    Service      │  │   ConfigMap     │          ││
│  │  │  payments-api   │  │  payments-api   │  │  payments-api   │          ││
│  │  │                 │  │                 │  │                 │          ││
│  │  │ ownerReferences:│  │ ownerReferences:│  │ ownerReferences:│          ││
│  │  │ - kind: App     │  │ - kind: App     │  │ - kind: App     │          ││
│  │  │   name: pay-api │  │   name: pay-api │  │   name: pay-api │          ││
│  │  │   controller: ✓ │  │                 │  │                 │          ││
│  │  └─────────────────┘  └─────────────────┘  └─────────────────┘          ││
│  │                                                                         ││
│  │  DELETION MODES:                                                        ││
│  │  - Foreground: Children deleted first, then owner (blocks)              ││
│  │  - Background: Owner deleted, children cleaned async (default)          ││
│  │  - Orphan: Owner deleted, children remain                               ││
│  │                                                                         ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  CONTROLLER FLAG:                                                           │
│  - Only ONE owner can have controller: true                                 │
│  - Controller gets precedence in conflict resolution                        │
│  - Used for determining "primary" owner                                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Deliverables:**
- [ ] Deployment generation with pod spec from Application
- [ ] Service generation (ClusterIP, port mapping)
- [ ] ConfigMap generation for application configuration
- [ ] Secret placeholder (for credential references)
- [ ] HPA generation from scaling spec
- [ ] PodDisruptionBudget for availability guarantees
- [ ] Owner references on all created resources
- [ ] Resource update logic (handle spec changes)
- [ ] Unit tests for resource generation
- [ ] Test garbage collection on Application delete

---

### Milestone 4: Status Management & Conditions - NOT STARTED

**Goal:** Implement proper status reporting with conditions following Kubernetes conventions.

**Learning Focus:**
- Status subresource pattern (spec vs status)
- Kubernetes conditions convention
- Observability through status
- Status update patterns (avoid conflicts)

**Concepts to Understand:**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         KUBERNETES CONDITIONS PATTERN                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  WHY CONDITIONS:                                                            │
│  - Single `phase` field is too limited                                      │
│  - Can't express "DB provisioning but cache failed"                         │
│  - Conditions allow independent status for each concern                     │
│                                                                             │
│  CONDITION STRUCTURE:                                                       │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │  conditions:                                                            ││
│  │    - type: Ready           # The overall readiness                      ││
│  │      status: "False"       # True, False, Unknown                       ││
│  │      reason: DatabaseFailed # CamelCase, machine-readable               ││
│  │      message: "RDS instance failed to provision: QUOTA_EXCEEDED"        ││
│  │      lastTransitionTime: "2025-02-09T10:00:00Z"                         ││
│  │      observedGeneration: 5  # Which spec generation this reflects       ││
│  │                                                                         ││
│  │    - type: WorkloadReady                                                ││
│  │      status: "True"                                                     ││
│  │      reason: DeploymentAvailable                                        ││
│  │                                                                         ││
│  │    - type: DatabaseReady                                                ││
│  │      status: "False"                                                    ││
│  │      reason: Provisioning                                               ││
│  │      message: "RDS instance is starting up..."                          ││
│  │                                                                         ││
│  │    - type: CacheReady                                                   ││
│  │      status: "True"                                                     ││
│  │      reason: ElastiCacheAvailable                                       ││
│  │                                                                         ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  CONVENTIONS:                                                               │
│  - "Ready" = overall status (True only if all components ready)             │
│  - Use positive polarity (Ready, not NotReady)                              │
│  - Reason = why this status (short, CamelCase)                              │
│  - Message = human-readable details                                         │
│  - Update lastTransitionTime only on status change                          │
│  - observedGeneration = which spec version status reflects                  │
│                                                                             │
│  ANTI-PATTERNS:                                                             │
│  ✗ Updating status on every reconcile (causes watch storms)                 │
│  ✗ Losing lastTransitionTime (resets every reconcile)                       │
│  ✗ Using phase field alone (can't express partial states)                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Deliverables:**
- [ ] Status subresource in CRD definition (marker: `+kubebuilder:subresource:status`)
- [ ] Condition types: Ready, WorkloadReady, DatabaseReady, CacheReady, QueueReady
- [ ] Condition helper functions (SetCondition, GetCondition, IsReady)
- [ ] Phase field: Pending/Provisioning/Ready/Failed/Deleting
- [ ] Infrastructure status (endpoints, ports, secrets)
- [ ] ObservedGeneration tracking
- [ ] Event recording for key state transitions
- [ ] Status update conflict handling (retry on conflict)
- [ ] Tests for condition transitions

---

### Milestone 5: Finalizers & Cleanup - NOT STARTED

**Goal:** Implement safe deletion with finalizers to prevent orphaned resources.

**Learning Focus:**
- Finalizer pattern and mechanics
- Deletion workflow in Kubernetes
- Preventing orphaned cloud resources
- Graceful cleanup order

**Concepts to Understand:**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              FINALIZER PATTERN                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  WHY FINALIZERS:                                                            │
│  When user runs `kubectl delete application payments-api`:                  │
│  - K8s wants to delete the object immediately                               │
│  - But we have AWS resources (RDS, ElastiCache) to clean up                 │
│  - Finalizers BLOCK deletion until we're done with cleanup                  │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                          DELETION FLOW                                  ││
│  │                                                                         ││
│  │  1. Application Created                                                 ││
│  │     ┌─────────────────────────────────────────────────────────────┐     ││
│  │     │ metadata:                                                   │     ││
│  │     │   finalizers:                                               │     ││
│  │     │   - platform.goplatform.io/cleanup  ◄── Added by controller │     ││
│  │     │   deletionTimestamp: null                                   │     ││
│  │     └─────────────────────────────────────────────────────────────┘     ││
│  │                                │                                        ││
│  │                                ▼                                        ││
│  │  2. User Deletes: kubectl delete application payments-api               ││
│  │     ┌─────────────────────────────────────────────────────────────┐     ││
│  │     │ metadata:                                                   │     ││
│  │     │   finalizers:                                               │     ││
│  │     │   - platform.goplatform.io/cleanup  ◄── Still present       │     ││
│  │     │   deletionTimestamp: 2025-02-09T10:00:00Z ◄── K8s marks     │     ││
│  │     └─────────────────────────────────────────────────────────────┘     ││
│  │     Object is NOT deleted yet! User sees "Terminating"                  ││
│  │                                │                                        ││
│  │                                ▼                                        ││
│  │  3. Controller Reconciles (sees deletionTimestamp != nil)               ││
│  │     - Run `terraform destroy` for AWS resources                         ││
│  │     - Wait for destruction to complete                                  ││
│  │     - Remove finalizer from metadata.finalizers                         ││
│  │                                │                                        ││
│  │                                ▼                                        ││
│  │  4. Finalizer Removed                                                   ││
│  │     ┌─────────────────────────────────────────────────────────────┐     ││
│  │     │ metadata:                                                   │     ││
│  │     │   finalizers: []  ◄── Empty now                             │     ││
│  │     │   deletionTimestamp: 2025-02-09T10:00:00Z                   │     ││
│  │     └─────────────────────────────────────────────────────────────┘     ││
│  │                                │                                        ││
│  │                                ▼                                        ││
│  │  5. K8s Garbage Collector sees no finalizers → Object deleted           ││
│  │                                                                         ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  EDGE CASES:                                                                │
│  - Terraform destroy fails → Keep retrying, object stuck in Terminating     │
│  - Controller crashes mid-cleanup → On restart, sees deletionTimestamp,     │
│    continues cleanup from where it left off                                 │
│  - Force delete with --force --grace-period=0 → Still waits for finalizer!  │
│  - To truly force: kubectl patch -p '{"metadata":{"finalizers":null}}'      │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Deliverables:**
- [ ] Add finalizer on Application create/update
- [ ] Detect deletion via `DeletionTimestamp != nil`
- [ ] Cleanup infrastructure (prepare for Terraform destroy in M6)
- [ ] Remove finalizer after successful cleanup
- [ ] Update status to Deleting phase during cleanup
- [ ] Handle cleanup failures (retry, don't remove finalizer)
- [ ] Event recording for cleanup stages
- [ ] Tests for normal deletion flow
- [ ] Tests for cleanup failure scenarios
- [ ] Document manual intervention for stuck resources

---

## Phase 2: Infrastructure Providers

### Milestone 6: Provider Interface & Factory - NOT STARTED

**Goal:** Design the adapter pattern for infrastructure provisioning.

**Learning Focus:**
- Go interfaces for abstraction
- Factory pattern for provider selection
- Configuration-driven provider instantiation
- Strategy pattern for different clouds

**Deliverables:**
- [ ] InfrastructureProvider interface definition
- [ ] ProviderConfig CRD for platform configuration
- [ ] ProviderFactory to instantiate correct provider
- [ ] Provider selection from config (aws/gcp/local)
- [ ] Mock provider for testing
- [ ] Provider lifecycle management
- [ ] Error types for infrastructure failures
- [ ] Unit tests with mock provider

---

### Milestone 7: Terraform Runner - NOT STARTED

**Goal:** Execute Terraform from within the controller for AWS resource provisioning.

**Learning Focus:**
- Calling external processes from Go (os/exec)
- Terraform CLI workflow (init/plan/apply/destroy)
- Parsing Terraform JSON output
- Working directory and file management
- Process timeouts and cancellation

**Deliverables:**
- [ ] TerraformRunner struct with CLI wrapper
- [ ] Working directory management per application
- [ ] HCL template generation from Go
- [ ] terraform init with backend configuration
- [ ] terraform plan with JSON output
- [ ] terraform apply with plan file
- [ ] terraform destroy for cleanup
- [ ] terraform output parsing
- [ ] Timeout and cancellation via context
- [ ] Error parsing and classification
- [ ] Structured logging of TF output
- [ ] Unit tests with mocked terraform binary
- [ ] Integration tests with LocalStack

---

### Milestone 8: State Management - NOT STARTED

**Goal:** Implement per-application Terraform state isolation with S3 backend and DynamoDB locking.

**Learning Focus:**
- Terraform state mechanics and why it matters
- S3 backend configuration
- DynamoDB locking for concurrent access
- State isolation strategies for multi-tenancy

**Deliverables:**
- [ ] S3 backend configuration generation
- [ ] DynamoDB lock table setup (one-time platform setup)
- [ ] State key pattern: `apps/{namespace}/{name}/terraform.tfstate`
- [ ] Backend config injection into generated HCL
- [ ] Lock acquisition timeout handling
- [ ] State file cleanup on Application delete
- [ ] State import capability (for existing resources)
- [ ] Tests for concurrent access scenarios

---

### Milestone 9: AWS RDS Module - NOT STARTED

**Goal:** Generate Terraform module for RDS PostgreSQL/MySQL provisioning.

**Learning Focus:**
- RDS configuration options (instance types, storage, networking)
- Security groups and subnet groups
- Parameter groups for database tuning
- Backup and maintenance windows
- Multi-AZ for high availability

**Deliverables:**
- [ ] RDS module HCL generation
- [ ] Size mapping (small→db.t3.micro, medium→db.t3.small, large→db.m5.large)
- [ ] Version support (PostgreSQL 13-16, MySQL 8.0)
- [ ] Multi-AZ configuration from HA spec
- [ ] Subnet group configuration
- [ ] Security group with least-privilege rules
- [ ] Parameter group for common tuning
- [ ] Backup configuration (window, retention)
- [ ] Maintenance window configuration
- [ ] Master password generation (store in Secrets Manager)
- [ ] Output extraction (endpoint, port, ARN)
- [ ] IAM policy for application access
- [ ] Integration tests with LocalStack

---

### Milestone 10: AWS ElastiCache Module - NOT STARTED

**Goal:** Generate Terraform module for ElastiCache Redis provisioning.

**Learning Focus:**
- ElastiCache Redis cluster modes
- Replication groups for HA
- Security groups and subnet groups
- Parameter groups for Redis tuning
- Encryption options

**Deliverables:**
- [ ] ElastiCache module HCL generation
- [ ] Size mapping (small→cache.t3.micro, etc.)
- [ ] Single-node vs replication group
- [ ] Automatic failover configuration
- [ ] Subnet group configuration
- [ ] Security group with least-privilege
- [ ] Parameter group for common tuning
- [ ] Encryption at rest and in transit
- [ ] Auth token for Redis 6+
- [ ] Output extraction (endpoint, port)
- [ ] Integration tests with LocalStack

---

### Milestone 11: AWS SQS Module - NOT STARTED

**Goal:** Generate Terraform module for SQS queue provisioning.

**Learning Focus:**
- SQS queue types (Standard vs FIFO)
- Dead Letter Queues and redrive policies
- Visibility timeout and retention
- Access policies

**Deliverables:**
- [ ] SQS module HCL generation
- [ ] Standard queue configuration
- [ ] FIFO queue configuration (with deduplication)
- [ ] Dead Letter Queue with redrive policy
- [ ] Visibility timeout configuration
- [ ] Message retention configuration
- [ ] Server-side encryption
- [ ] Access policy for application
- [ ] Output extraction (URL, ARN)
- [ ] Integration tests with LocalStack

---

### Milestone 12: IAM & IRSA - NOT STARTED

**Goal:** Generate IAM roles for applications with IRSA (IAM Roles for Service Accounts).

**Learning Focus:**
- IAM role trust policies
- IRSA mechanics (OIDC provider)
- Least-privilege IAM policies
- Service account annotation

**Deliverables:**
- [ ] IAM role module HCL generation
- [ ] OIDC trust policy for EKS
- [ ] Per-resource IAM policies (RDS connect, S3 access, SQS send/receive)
- [ ] Least-privilege policy generation
- [ ] ServiceAccount creation with annotation
- [ ] Role ARN output for pod configuration
- [ ] Deployment updates to use ServiceAccount
- [ ] Tests for IAM policy correctness

---

## Phase 3: Credential Management

### Milestone 13: Secrets Generation - NOT STARTED

**Goal:** Create Kubernetes Secrets from Terraform outputs for application consumption.

**Deliverables:**
- [ ] Secret generation from Terraform outputs
- [ ] Standard secret format (DATABASE_URL, etc.)
- [ ] Owner reference to Application
- [ ] Secret update on infrastructure change
- [ ] Secret deletion on Application delete
- [ ] Support for both envFrom and volumeMount patterns
- [ ] Tests for secret content

---

### Milestone 14: External Secrets Integration - NOT STARTED

**Goal:** Integrate with External Secrets Operator for production-grade secrets management.

**Deliverables:**
- [ ] Detect if ESO is installed in cluster
- [ ] Generate ExternalSecret instead of Secret when ESO available
- [ ] AWS Secrets Manager path convention
- [ ] SecretStore reference configuration
- [ ] Fallback to K8s Secret if ESO not available
- [ ] Documentation for ESO setup

---

### Milestone 15: Secrets Rotation - NOT STARTED

**Goal:** Implement automatic database password rotation.

**Deliverables:**
- [ ] Detect rotation-enabled databases
- [ ] Configure AWS SM rotation schedule
- [ ] Lambda rotation function deployment
- [ ] Secret rotation trigger configuration
- [ ] Application restart strategy on rotation
- [ ] Rotation status in Application status
- [ ] Manual rotation trigger via annotation
- [ ] Tests for rotation flow

---

## Phase 4: Platform API & CLI

### Milestone 16: REST API Server - NOT STARTED

**Goal:** Build a REST API for platform operations beyond kubectl.

**Deliverables:**
- [ ] HTTP server with chi or gin router
- [ ] OpenAPI 3.0 specification
- [ ] List applications endpoint (with filters)
- [ ] Get application status endpoint
- [ ] Create/update application endpoint
- [ ] Delete application endpoint
- [ ] Health and readiness endpoints
- [ ] Request logging middleware
- [ ] Error response standardization
- [ ] Swagger UI for API documentation
- [ ] Integration tests

---

### Milestone 17: Cost Estimation API - NOT STARTED

**Goal:** Provide cost estimation before provisioning.

**Deliverables:**
- [ ] AWS Pricing API integration
- [ ] Price caching (refresh daily)
- [ ] Cost calculation per resource type
- [ ] Size → instance type → price mapping
- [ ] Estimate endpoint in API
- [ ] Cost in Application status
- [ ] Cost breakdown by resource
- [ ] Historical cost tracking (future)

---

### Milestone 18: CLI Tool (gpctl) - NOT STARTED

**Goal:** Build a CLI tool for platform interaction.

**Deliverables:**
- [ ] gpctl binary with cobra CLI framework
- [ ] `gpctl apply -f app.yaml` - Create/update application
- [ ] `gpctl get apps` - List applications
- [ ] `gpctl describe app NAME` - Show details
- [ ] `gpctl status NAME` - Show provisioning status
- [ ] `gpctl estimate -f app.yaml` - Cost estimation
- [ ] `gpctl logs NAME` - Show application logs
- [ ] `gpctl delete NAME` - Delete application
- [ ] Multiple output formats (table, json, yaml)
- [ ] Kubeconfig context support
- [ ] Auto-completion for bash/zsh/fish
- [ ] Configuration file (~/.gpctl/config)

---

### Milestone 19: Webhook Events - NOT STARTED

**Goal:** Emit webhook events for application lifecycle changes.

**Deliverables:**
- [ ] WebhookConfig CRD for registering endpoints
- [ ] Event types: ApplicationCreated, Provisioned, Failed, Deleted
- [ ] Webhook delivery with retries
- [ ] HMAC signature for verification
- [ ] Delivery status tracking
- [ ] Failed delivery alerting
- [ ] Integration with Slack, PagerDuty, etc.

---

## Phase 5: Observability

### Milestone 20: ServiceMonitor Generation - NOT STARTED

**Goal:** Auto-generate Prometheus ServiceMonitors for every application.

**Deliverables:**
- [ ] ServiceMonitor generation from Application spec
- [ ] Metrics path and port from observability spec
- [ ] Labels for Prometheus discovery
- [ ] Scrape interval configuration
- [ ] Metrics relabeling for team/app labels
- [ ] Owner reference to Application
- [ ] Tests for ServiceMonitor generation

---

### Milestone 21: Grafana Dashboard Generation - NOT STARTED

**Goal:** Auto-generate Grafana dashboards based on application type.

**Deliverables:**
- [ ] Dashboard JSON template system
- [ ] Base HTTP dashboard (rate, errors, latency)
- [ ] Language-specific panels (Go, Python, Node, Java)
- [ ] Infrastructure panels (RDS, Redis, SQS)
- [ ] GrafanaDashboard CRD generation
- [ ] Dashboard links in Application status
- [ ] Dashboard cleanup on Application delete

---

### Milestone 22: AlertRule Generation - NOT STARTED

**Goal:** Auto-generate PrometheusRules for SLA-based alerting.

**Deliverables:**
- [ ] PrometheusRule generation from Application spec
- [ ] SLA-based alerts (based on spec.tier):
  - Critical: <99.9% availability, P99 >100ms
  - Standard: <99.5% availability, P99 >500ms
  - Development: <99% availability, P99 >1s
- [ ] High error rate alerts
- [ ] Pod crash alerts
- [ ] Infrastructure alerts (RDS CPU, Redis memory)
- [ ] Alert labels (team, app, tier)
- [ ] Owner reference to Application
- [ ] Tests for alert generation

---

### Milestone 23: OpenTelemetry Configuration - NOT STARTED

**Goal:** Configure distributed tracing via OpenTelemetry.

**Deliverables:**
- [ ] OpenTelemetry Instrumentation CRD generation
- [ ] Auto-instrumentation configuration per language
- [ ] Trace collector endpoint configuration
- [ ] Sample rate from observability spec
- [ ] Jaeger/Tempo integration
- [ ] Trace correlation with logs
- [ ] Documentation for manual instrumentation

---

## Phase 6: Service Catalog

### Milestone 24: Catalog Data Model - NOT STARTED

**Goal:** Design and implement the service catalog data model.

**Deliverables:**
- [ ] Service entity model
- [ ] Team entity model
- [ ] Relationship types (depends-on, owned-by)
- [ ] Metadata extensibility
- [ ] Catalog storage (in-cluster CRD or database)
- [ ] Catalog sync from Application CRDs
- [ ] API for catalog queries

---

### Milestone 25: Dependency Tracking - NOT STARTED

**Goal:** Track and visualize service dependencies.

**Deliverables:**
- [ ] Dependency extraction from Application spec
- [ ] Dependency validation (target exists)
- [ ] Dependency graph building
- [ ] Impact analysis API ("what depends on X?")
- [ ] Circular dependency detection
- [ ] Dependency visualization endpoint (for UI)
- [ ] Startup ordering from dependency graph
- [ ] Tests for graph operations

---

### Milestone 26: Team Ownership - NOT STARTED

**Goal:** Track team ownership and enable team-based views.

**Deliverables:**
- [ ] Team CRD (or annotation-based)
- [ ] Team → Applications mapping
- [ ] Team list endpoint
- [ ] Applications by team endpoint
- [ ] Team contact information
- [ ] On-call integration metadata
- [ ] Team dashboards in Grafana

---

### Milestone 27: Resource Templates - NOT STARTED

**Goal:** Provide pre-built templates for common application patterns.

**Deliverables:**
- [ ] Template CRD or embedded templates
- [ ] Template for Go REST API
- [ ] Template for background worker
- [ ] Template for frontend BFF
- [ ] gpctl command to list templates
- [ ] gpctl command to scaffold from template
- [ ] Template validation
- [ ] Custom template support

---

## Phase 7: Developer Experience

### Milestone 28: Environment Promotion - NOT STARTED

**Goal:** Enable configuration promotion from dev → staging → prod.

**Deliverables:**
- [ ] Environment concept (dev/staging/prod)
- [ ] EnvironmentConfig CRD for overrides
- [ ] Base + overlay configuration merge
- [ ] Promote command in gpctl
- [ ] Promotion diff preview
- [ ] Promotion history tracking
- [ ] Approval workflow for production (annotation-based)

---

### Milestone 29: Preview Environments - NOT STARTED

**Goal:** Auto-create full stack preview environments for pull requests.

**Deliverables:**
- [ ] PreviewEnvironment CRD
- [ ] Preview namespace provisioning
- [ ] Local provider for preview (no cloud costs)
- [ ] Ingress/URL generation for preview
- [ ] TTL-based auto-cleanup
- [ ] GitHub/GitLab webhook integration
- [ ] PR comment with preview URL
- [ ] Preview status in CI checks
- [ ] gpctl preview commands

---

### Milestone 30: Local Development Mode - NOT STARTED

**Goal:** Enable developers to run the same stack locally.

**Deliverables:**
- [ ] LocalProvider implementation
- [ ] CloudNativePG for local PostgreSQL
- [ ] Redis operator for local Redis
- [ ] Local SQS alternative (ElasticMQ or fake SQS)
- [ ] Docker Compose generation from Application
- [ ] Tilt or Skaffold integration
- [ ] gpctl local commands
- [ ] Documentation for local development

---

### Milestone 31: Drift Detection - NOT STARTED

**Goal:** Detect when cloud infrastructure drifts from desired state.

**Deliverables:**
- [ ] Periodic drift detection job
- [ ] Terraform plan for drift detection
- [ ] Drift status condition
- [ ] Drift alert generation
- [ ] Auto-heal option (configurable)
- [ ] Drift report in Application status
- [ ] gpctl drift command
- [ ] Metrics for drift events

---

## Phase 8: Production Hardening

### Milestone 32: Policy Enforcement - NOT STARTED

**Goal:** Implement policy-as-code for infrastructure compliance.

**Deliverables:**
- [ ] Built-in policies:
  - Resource limits required
  - Team label required
  - Database backup required for production
  - HA required for critical tier
- [ ] Policy CRD for custom policies
- [ ] Compliance status in Application
- [ ] Policy violation alerts
- [ ] Exception workflow (with approvals)
- [ ] Compliance dashboard

---

### Milestone 33: Team Quotas & Budgets - NOT STARTED

**Goal:** Implement cost controls per team.

**Deliverables:**
- [ ] TeamQuota CRD (max applications, max resources)
- [ ] Budget tracking per team
- [ ] Budget alert thresholds
- [ ] Quota enforcement on provisioning
- [ ] Cost dashboard per team
- [ ] Monthly cost reports
- [ ] Chargeback integration (Kubecost, etc.)

---

### Milestone 34: Audit Logging - NOT STARTED

**Goal:** Implement comprehensive audit logging.

**Deliverables:**
- [ ] Audit log for all mutations (create, update, delete)
- [ ] Who/what/when/where captured
- [ ] Audit log storage (CloudWatch, Loki)
- [ ] Audit log retention policies
- [ ] Audit log search API
- [ ] Integration with SIEM tools
- [ ] Compliance reports

---

### Milestone 35: High Availability & Scaling - NOT STARTED

**Goal:** Production-harden the platform controller.

**Deliverables:**
- [ ] Leader election for controller HA
- [ ] Multiple controller replicas
- [ ] Work queue rate limiting
- [ ] Terraform concurrency limits
- [ ] Circuit breaker for AWS API
- [ ] Graceful shutdown handling
- [ ] Controller metrics (queue depth, reconcile time)
- [ ] Health and readiness probes
- [ ] PodDisruptionBudget for controller

---

## Architectural Decisions

Track key architectural decisions as they are made:

| Decision | Options Considered | Choice | Reasoning |
|----------|-------------------|--------|-----------|
| Cloud abstraction | Direct AWS calls, XRD-style, Interface pattern | Interface pattern | Simpler than XRDs, more flexible than direct calls |
| Credential passing | K8s Secrets, ESO, CSI driver | K8s Secrets + ESO support | Simple by default, ESO for production |
| RBAC approach | Platform-level, K8s RBAC only | K8s RBAC + policies, then platform-level | Don't reinvent, add when needed |
| Local Kubernetes | minikube, kind, k3s, Colima | Colima | Docker runtime, good macOS support |
| _More to fill during development_ | | | |

---

## Concepts Learned

Track platform engineering concepts learned during development:

| Concept | Description | Where Applied |
|---------|-------------|---------------|
| Reconciliation Pattern | Level-triggered, idempotent state management | M2 |
| Finalizer Pattern | Block deletion until cleanup complete | M5 |
| Owner References | Automatic garbage collection of child resources | M3 |
| Conditions | Multi-dimensional status reporting | M4 |
| IRSA | Per-pod AWS credentials without shared secrets | M12 |
| External Secrets | Sync cloud secrets to K8s | M14 |
| _More to fill during development_ | | |

---

## Known Issues

Track issues discovered during development:

| Issue | Severity | Status | Notes |
|-------|----------|--------|-------|
| _None yet_ | | | |

