# Suggested Commands

## Development
- make run                         # Run controller locally
- make build                       # Build manager binary

## Generate/Validate
- make manifests                   # Regenerate CRDs/RBAC
- make generate                    # Regenerate deepcopy code
- make fmt                         # go fmt
- make vet                         # go vet
- make lint-fix                    # golangci-lint --fix

## Testing
- make test                        # Unit tests (envtest)
- make test-e2e                    # E2E tests (Kind)

## Utility (Darwin/macOS)
- ls, cd, pwd
- grep -R "pattern" .
- find . -name "*.go"
- git status, git diff, git log
