# Code Style and Conventions

## Go Style
- Follow idiomatic Go (Effective Go + Go Code Review Comments).
- Prefer clarity, early returns, and nil safety.
- Document exported symbols.
- Wrap errors with context using fmt.Errorf("...: %w", err).
- Avoid emoji in code/comments.

## Kubernetes Operator Conventions
- Use controller-runtime patterns (Reconcile, SetupWithManager).
- Status updates use the /status subresource.
- Use metav1.Condition for status conditions.
- Use controllerutil.CreateOrUpdate for child resources.
- Set owner references for GC.
- Emit Events for key lifecycle operations.

## Generated Files
- Do NOT edit generated files:
  - config/crd/bases/*.yaml
  - config/rbac/role.yaml
  - **/zz_generated.*.go
  - PROJECT

## Testing Style
- Ginkgo/Gomega (Describe/Context/It)
- Envtest for controller tests
