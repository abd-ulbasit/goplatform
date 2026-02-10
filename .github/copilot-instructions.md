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

## Production Quality Standards

### Nil Safety & Defensive Programming
- **Always check for nil** before dereferencing pointers
- Provide sensible defaults when optional fields are nil
- Document expected nil behavior in comments
- Add nil guards in all functions that receive pointer arguments

### Error Handling & Resilience
- **Use retry.RetryOnConflict** for status updates to handle concurrent modifications
- Emit meaningful error messages with context (wrap errors with `fmt.Errorf("failed to X: %w", err)`)
- Add timeout handling for long-running operations (especially external calls)
- Track operation start times to detect stuck processes

### CreateOrUpdate Pattern for Kubernetes Resources
```go
// PREFER: Atomic CreateOrUpdate
opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
    obj.Spec = desired.Spec
    return controllerutil.SetControllerReference(owner, obj, r.Scheme)
})

// AVOID: Separate Get/Create/Update (race conditions possible)
if err := r.Get(ctx, key, obj); err != nil { ... }
```

### Event Recording for Operational Visibility
- Emit **Normal** events for successful operations (Created, Updated, Deleted)
- Emit **Warning** events for errors and degraded states
- Include relevant details in event messages (e.g., "Created HPA (min: 2, max: 10)")
- Events visible via `kubectl describe` - essential for debugging

### Predicates to Reduce Unnecessary Reconciles
```go
For(&Application{},
    builder.WithPredicates(predicate.GenerationChangedPredicate{}))
```
- Only reconcile when spec changes, not status-only updates
- Reduces API server load and controller CPU usage
- Consider custom predicates for specific filtering needs

### Consistent Resource Cleanup
- When a spec field is removed (e.g., `spec.Scaling`), **delete** the associated resource
- Don't rely only on owner references for spec-change cleanup
- Log and emit events when cleaning up resources

### Owner References and Garbage Collection
- Set `controllerutil.SetControllerReference` on all created resources
- Enables automatic cleanup when parent is deleted
- Enables watch propagation (child changes → parent reconciled)

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
//                                                    │
//                        ┌───────────────────────────┘
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

## Testing Best Practices

### Ginkgo/Gomega with Envtest
This project uses **Ginkgo/Gomega** with **envtest** - the standard for Kubernetes operators:
- Runs real kube-apiserver and etcd (from test binaries)
- Catches schema bugs and API behavior issues
- Controllers are tested step-by-step (deterministic)

### Test Coverage Targets
- **Controllers**: 70%+ coverage with envtest
- **Edge cases**: Deletion, updates, error handling
- **Async assertions**: Use `Eventually()` for reconciliation checks

### Test Patterns
```go
// Use Eventually for async operations
Eventually(func() bool {
    return k8sClient.Get(ctx, key, obj) == nil
}, timeout, interval).Should(BeTrue())

// Test cleanup by reconciling multiple times if needed
for i := 0; i < 3; i++ {
    result, err := reconciler.Reconcile(ctx, req)
    if !result.Requeue { break }
}
```

### What to Test
- **Happy path**: Resource creation, updates
- **Cleanup**: Deletion, finalizer removal, spec field removal
- **Error handling**: Invalid input, API failures
- **Status updates**: Conditions, phase transitions

## Code Review Checklist

When reviewing or writing code, verify:

### Safety
- [ ] Nil checks before pointer dereference
- [ ] Error wrapping with context
- [ ] Retry logic for conflict-prone operations

### Kubernetes Patterns
- [ ] Owner references set on child resources
- [ ] CreateOrUpdate for atomic operations
- [ ] Events emitted for important operations
- [ ] Predicates to reduce reconcile load

### Cleanup
- [ ] Resources deleted when spec field removed
- [ ] Finalizer handles external resource cleanup
- [ ] Graceful handling of stuck deletions

### Observability
- [ ] Structured logging with context
- [ ] Events for kubectl describe visibility
- [ ] Status conditions for progress tracking

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
11. **Nil safety** - Always check pointers before dereference
12. **Events** - Emit Kubernetes events for operational visibility
13. **CreateOrUpdate** - Use atomic patterns, avoid race conditions
14. **Retry on conflict** - Status updates should handle concurrent modifications
15. **Predicates** - Filter events to reduce unnecessary reconciles
16. **Consistent cleanup** - Delete resources when spec fields are removed
