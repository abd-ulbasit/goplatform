# GitHub Copilot Instructions

## Role & Context
I am learning advanced Kubernetes, Terraform, and AWS patterns by building goplatform - an Internal Developer Platform. AI writes production code; I focus on architectural decisions and learning through reading.

## Learning Philosophy: Agentic AI-Driven with Deep Understanding
- **AI implements, I architect** - AI writes complete, production-ready code
- **I read everything** - Every line of code should teach me something
- **Comments are documentation** - Rich inline comments explain WHY, HOW, and ALTERNATIVES
- **Learn by building real systems** - goplatform teaches platform engineering through implementation
- **Compare with real platforms** - Every pattern should reference how Backstage, Crossplane, Terraform Cloud do it

## Critical Learning Requirement

**I have working knowledge of K8s, Terraform, AWS - but NOT deep expertise.**

For EVERY significant piece of code, explain:
1. **WHY** - Why are we doing it this way?
2. **HOW** - How does it work under the hood?
3. **ALTERNATIVES** - What other approaches exist? Why didn't we choose them?
4. **TRADEOFFS** - What are the pros/cons?
5. **REAL-WORLD** - How do Spotify, Netflix, AWS, Google do this?
6. **FAILURE MODES** - What can go wrong?

## Learning Mode: Full Agentic Implementation

### How Sessions Should Flow
1. **I describe** what I want to build or learn
2. **Copilot presents** architectural options with tradeoffs (table format)
3. **Copilot explains** concepts I might not deeply understand
4. **I choose** the direction based on tradeoffs
5. **Copilot implements** complete, working code with rich comments
6. **Copilot explains** platform concepts inline via comprehensive comments
7. **Copilot writes tests** with explanatory comments
8. **I review** every line, ask questions about anything unclear
9. **We iterate** - I request changes, Copilot implements

### Implementation Standards

**Every significant function should have:**
```go
// ============================================================================
// FINALIZER PATTERN FOR SAFE DELETION
// ============================================================================
//
// WHY: Kubernetes deletes objects immediately when you run kubectl delete.
// If our operator has created external AWS resources (RDS, ElastiCache),
// they would be orphaned. Finalizers prevent deletion until cleanup is done.
//
// HOW IT WORKS:
//   1. When Application is created, we add finalizer to metadata.finalizers[]
//   2. When user deletes Application, Kubernetes sets deletionTimestamp but
//      doesn't actually delete because finalizer exists
//   3. Our reconciler detects deletionTimestamp != nil
//   4. We run Terraform destroy to cleanup AWS resources
//   5. After cleanup, we remove the finalizer
//   6. Kubernetes sees no finalizers, actually deletes the object
//
// ALTERNATIVES CONSIDERED:
//   ┌────────────────────────────────────────────────────────────────────────┐
//   │ Approach              │ Pros                 │ Cons                    │
//   ├───────────────────────┼──────────────────────┼─────────────────────────┤
//   │ No finalizer          │ Simple               │ Orphaned AWS resources  │
//   │                       │                      │ (costs money forever)   │
//   ├───────────────────────┼──────────────────────┼─────────────────────────┤
//   │ Pre-delete webhook    │ Synchronous cleanup  │ Timeouts, blocks API,   │
//   │                       │                      │ single point of failure │
//   ├───────────────────────┼──────────────────────┼─────────────────────────┤
//   │ ✅ Finalizer          │ Async, reliable,     │ Object lingers until    │
//   │                       │ controller handles   │ cleanup done            │
//   └────────────────────────────────────────────────────────────────────────┘
//
// HOW CROSSPLANE/ACK DO IT:
//   - Crossplane: Finalizers on every managed resource
//   - AWS Controllers for K8s (ACK): Same pattern
//   - Operators universally use this for external resources
//
// FAILURE MODES:
//   1. Terraform destroy fails → Reconcile retried, object stuck deleting
//   2. Finalizer removed before cleanup → Resources orphaned (prevented by code)
//   3. Controller crashed mid-cleanup → On restart, sees deletionTimestamp, retries
//
// FLOW:
//   ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
//   │ User deletes │────►│ deletionTS   │────►│ Reconciler   │
//   │ Application  │     │ set by K8s   │     │ sees delete  │
//   └──────────────┘     └──────────────┘     └──────┬───────┘
//                                                     │
//                        ┌────────────────────────────┘
//                        ▼
//   ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
//   │ Remove       │◄────│ TF destroy   │◄────│ Cleanup AWS  │
//   │ finalizer    │     │ succeeds     │     │ resources    │
//   └──────┬───────┘     └──────────────┘     └──────────────┘
//          │
//          ▼
//   ┌──────────────┐
//   │ Object gone  │
//   │ from etcd    │
//   └──────────────┘
//
func (r *ApplicationReconciler) handleDeletion(ctx context.Context, app *v1alpha1.Application) error {
```

### What AI Must Do

- **Write complete, production-ready code** (no TODOs, no scaffolding)
- **Add rich inline comments** explaining:
  - WHY this pattern/approach (not just what it does)
  - HOW it works under the hood (mechanisms, not magic)
  - ALTERNATIVES and why we didn't choose them
  - COMPARISON to Backstage, Crossplane, Terraform Cloud, AWS ACK
  - TRADEOFFS and implications
  - FAILURE MODES and edge cases
  - ASCII DIAGRAMS for complex flows
- **Write comprehensive tests** with comments explaining what each tests
- **Handle all error cases** properly
- **Use idiomatic Go patterns**

### What I Do

- Make architectural decisions from presented options
- Read and understand every line of code
- Ask "why" and "how" questions when unclear
- Request deeper explanations for platform concepts
- Approve or redirect implementation direction

## Concepts to Teach (via code comments)

### Kubernetes Deep Concepts

**Controller Internals:**
- Informers, SharedIndexInformers, cache synchronization
- Work queues with rate limiting and exponential backoff
- Predicate filters for efficient event handling
- Leader election for high-availability controllers

**CRD Design:**
- Structural schemas with OpenAPI validation
- Status subresource vs spec (read-only status)
- Conversion webhooks for version migration
- Admission webhooks (validating, mutating)

**Operator Patterns:**
- Reconciliation loop (level-triggered)
- Finalizers for cleanup
- Owner references for garbage collection
- Conditions for status reporting
- Metrics and observability

### Terraform Integration

**State Management:**
- Per-application state isolation
- S3 backend with DynamoDB locking
- State import and taint
- Partial applies and recovery

**Programmatic Usage:**
- Calling Terraform from Go
- Module generation
- Output extraction
- Error parsing

### Platform Engineering

**IDP Patterns:**
- Golden paths
- Self-service with guardrails
- Developer portal design
- Cost allocation

**Service Catalog:**
- Dependency tracking
- Ownership metadata
- API documentation
- Compliance checks

## How I Want Feedback

### On Architecture Decisions:
- ✅ Present options with clear tradeoffs (table format)
- ✅ Explain how real platforms (Backstage, Crossplane) solve this
- ✅ Explain WHY each option matters
- ✅ Recommend a choice with reasoning
- ✅ Wait for my decision before implementing

### On Code I'm Reading:
- ✅ Answer "why" questions in depth
- ✅ Draw ASCII diagrams for complex flows
- ✅ Compare with how other platforms do it
- ✅ Explain failure modes and edge cases
- ✅ Explain K8s/Terraform internals when relevant

### Documentation:
- ❌ DON'T create summary markdown files unless I ask
- ❌ DON'T create extra documentation files
- ✅ DO add comprehensive inline comments explaining everything
- ✅ DO include ASCII diagrams in comments for complex concepts
- ✅ DO compare with other platforms in comments

## Tech Stack

**Core:**
- Go 1.22+ (controller, API, CLI)
- Kubernetes (CRDs, operators)
- Terraform (AWS infrastructure)

**Frameworks:**
- controller-runtime (Kubernetes operator)
- kubebuilder (scaffolding)
- cobra (CLI)
- chi or gin (REST API)

**Infrastructure:**
- AWS (RDS, ElastiCache, S3, IAM)
- Prometheus (metrics)
- Grafana (dashboards)

## Reminders for AI

1. **Implement fully** - No TODOs, no scaffolding, complete code
2. **Teach via comments** - Every function explains WHY, HOW, ALTERNATIVES
3. **Compare with real platforms** - How does Backstage/Crossplane do this?
4. **ASCII diagrams** - Visualize flows, state machines, data paths
5. **Present tradeoffs** - Let me choose after understanding
6. **Explain concepts first** - Explain the concept before showing code
7. **Memory bank integration** - Suggest updates for architectural decisions
8. **No unnecessary files** - No docs/examples unless requested
9. **Production quality** - Write code as if shipping to production
10. **Deep explanations** - I need to understand K8s/TF deeply, not just use them
