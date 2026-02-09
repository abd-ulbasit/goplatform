# Product Context

## Problem Space

### Developer Pain Points
1. **Infrastructure Request Friction**
   - Need to file tickets for databases, caches, queues
   - Wait 2-5 days for provisioning
   - Multiple back-and-forth clarifications
   - Different processes for different resources

2. **Inconsistent Environments**
   - Dev, staging, prod configured differently
   - "Works in dev, breaks in prod" scenarios
   - No standardization across teams

3. **Operational Burden on Platform Team**
   - Repetitive provisioning tasks
   - Custom configurations per request
   - Firefighting vs building

4. **Compliance and Security Gaps**
   - Manual security reviews
   - Missing encryption, backups, or monitoring
   - No audit trail

### Platform Team Pain Points
1. **Toil Over Innovation**
   - Spend 70% time on tickets, 30% on platform
   - No time for automation or improvement

2. **Inconsistent Implementations**
   - Each engineer provisions slightly differently
   - Documentation gets stale
   - Tribal knowledge problem

3. **Visibility Gaps**
   - Don't know what's running
   - Don't know who owns what
   - Can't track costs

## Solution: Internal Developer Platform

### Core Concept: Golden Paths
- Opinionated, well-lit paths for common use cases
- Developers follow the path, get best practices for free
- Escape hatches for advanced use cases

### Self-Service Model
```
Developer: "I need a Go service with PostgreSQL and Redis"
Platform:  "Here's your Application CRD - fill in these 10 fields"
Developer: "kubectl apply -f app.yaml"
Platform:  "Done. Here are your endpoints and credentials."
```

### What Gets Provisioned

For each Application, GoPlatform creates:

**Kubernetes Layer:**
- Deployment with health probes
- Service (ClusterIP by default)
- ConfigMap for app configuration
- HPA for autoscaling
- PDB for availability

**AWS Layer (via Terraform):**
- RDS PostgreSQL/MySQL (if requested)
- ElastiCache Redis/Memcached (if requested)
- SQS queues with DLQ (if requested)
- IAM roles with IRSA
- S3 buckets (if requested)

**Observability Layer:**
- Prometheus ServiceMonitor
- Grafana dashboard (auto-generated)
- AlertManager rules
- OpenTelemetry configuration

**Catalog Layer:**
- Service catalog entry
- Team ownership
- Dependency tracking
- Compliance status

## User Experience Goals

### For Developers
1. **Simple to start** - One YAML file, instant infrastructure
2. **Observable by default** - Metrics, logs, traces without config
3. **Self-service** - No tickets, no waiting
4. **Flexible escape hatches** - Can customize when needed

### For Platform Team
1. **Reduced toil** - Automation handles provisioning
2. **Consistent infrastructure** - Same patterns everywhere
3. **Visibility** - Know what's running, who owns it
4. **Control** - Enforce policies automatically

### For Security/Compliance
1. **Guardrails** - Policies enforced by default
2. **Audit trail** - All changes tracked
3. **Encryption** - Data encrypted at rest and in transit
4. **Least privilege** - IAM roles scoped appropriately

## Comparison with Alternatives

### vs. Backstage
- Backstage: Software catalog + scaffolding
- GoPlatform: Catalog + actual infrastructure provisioning
- We do more than Backstage (provision infrastructure)

### vs. Crossplane
- Crossplane: Generic K8s-based cloud provisioning
- GoPlatform: Opinionated, focuses on application patterns
- We're simpler for application teams

### vs. Terraform Cloud
- TFC: Terraform execution and state management
- GoPlatform: K8s-native, integrates with Terraform for AWS
- We're K8s-native, they're Terraform-native

### vs. Do-It-Yourself
- DIY: Custom scripts, manual provisioning
- GoPlatform: Standardized, automated
- We provide consistency and speed
