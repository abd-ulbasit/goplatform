# Active Context

## Current Focus
Pre-Milestone 1 planning complete. Ready to begin kubebuilder project initialization.

## Next Steps
1. Initialize kubebuilder project with domain `platform.goplatform.io`
2. Scaffold Application CRD v1alpha1
3. Design cloud-agnostic CRD schema
4. Set up local Kubernetes with Colima
5. Implement validation webhooks

## Recent Changes (Session 2)
- Designed credential flow (Terraform → K8s Secrets → Application)
- Designed InfrastructureProvider adapter pattern for cloud abstraction
- Evaluated RBAC (K8s RBAC + policies first, platform-level later)
- Added unique features: Cost Estimation, Preview Environments, Drift Detection
- Expanded PROGRESS.md with all 35 milestones across 8 phases
- Added development.instructions.md for Serena and local K8s
- Cleaned up README.md with competitive differentiators

## Active Decisions

### Made Decisions (Session 2)
1. **Cloud Abstraction** - Use InfrastructureProvider interface pattern (simpler than Crossplane XRDs)
2. **Credential Passing** - K8s Secrets (simple) + External Secrets Operator (production)
3. **RBAC Approach** - K8s RBAC + Kyverno policies first, add platform-level RBAC in Phase 8
4. **State Backend** - S3 + DynamoDB locking
5. **Local Kubernetes** - Colima with K8s 1.33+ (avoid extended support costs)
6. **Terraform Execution** - Subprocess (exec.Cmd) for isolation

### Pending Decisions
1. **kubebuilder vs operator-sdk** - Leaning kubebuilder (simpler, widely used)
2. **CLI Framework** - cobra + viper (standard Go CLI stack)

## Current Session Goals
- Begin Milestone 1: kubebuilder project setup
- Scaffold Application CRD
- Set up local development environment

## Blockers
None currently.

## Notes
- Focus on learning Kubernetes operator patterns deeply
- Every implementation should have comprehensive comments explaining WHY and HOW
- Compare approaches with real platforms (Backstage, Crossplane)
- Use Serena MCP tools for efficient code navigation
