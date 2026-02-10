# Task Completion Checklist

When a task involves code changes:
1. If *_types.go or kubebuilder markers changed:
   - make manifests
   - make generate
2. Run formatting/linting:
   - make fmt
   - make vet
   - (optional) make lint-fix
3. Run tests:
   - make test

Note: E2E tests require a dedicated Kind cluster (make test-e2e).
