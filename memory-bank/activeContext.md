# Active Context

## Current Focus
Scope revision complete. Ready to begin Milestone 6 (End-to-End Integration).

## Next Steps
1. Wire KubernetesProvider into controller reconciliation flow
2. Implement status mapping from provider → ApplicationStatus conditions
3. Set up Kind cluster with CNPG/Redis/RabbitMQ operators for real testing
4. Validate full create → status → delete lifecycle on real cluster

## Recent Changes (Session 4)
- Revised project scope from 36 milestones to 12
- Dropped Terraform, REST API, CLI, Service Catalog, Preview Environments
- Focused roadmap on K8s operator depth: webhooks, observability, drift detection, multi-version CRDs, policy, E2E
- Fixed milestone tracking inconsistencies
- Updated all documentation to reflect new scope

## Active Decisions

### Made Decisions
1. **Cloud Abstraction** - InfrastructureProvider interface pattern
2. **Infrastructure** - Kubernetes-only (CNPG, Redis, RabbitMQ operators). No Terraform.
3. **Developer Interface** - kubectl only. No CLI or REST API.
4. **Credential Passing** - K8s Secrets (simple, works everywhere)
5. **Policy Engine** - Kyverno integration (don't reinvent)
6. **Local Kubernetes** - Kind (testing) + Colima (dev)

### No Pending Decisions
All major architectural decisions resolved during scope revision.

## Blockers
None currently.

## Notes
- Primary goal: Learn Kubernetes deeply by building operators
- Timeline: 1-2 months for remaining 7 milestones (M6-M12)
- Each milestone chosen for maximum K8s learning value
- Validate on real clusters, not just envtest
