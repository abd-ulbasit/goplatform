# GoPlatform Project Brief

## Overview
GoPlatform is an Internal Developer Platform (IDP) - a Kubernetes operator that automates infrastructure provisioning for developers. It transforms declarative Application CRDs into fully provisioned, observable infrastructure.

## Why This Project Exists

### The Problem
Platform teams spend endless hours on:
1. **Ticket-based provisioning** - "Please create my database" (3-day SLA)
2. **Snowflake infrastructure** - Every service configured differently
3. **Knowledge silos** - Only platform team knows how to deploy
4. **Compliance gaps** - Manual security reviews, inconsistent policies
5. **Cost opacity** - No idea what each team spends

### The Solution
GoPlatform lets developers declare what they need, and the platform provisions everything automatically:
- Kubernetes resources (Deployment, Service, HPA)
- AWS infrastructure (RDS, ElastiCache, SQS)
- Observability (Prometheus, Grafana, Alerting)
- Service catalog entry

## Core Goals

1. **Self-Service Infrastructure** - Developers get what they need without tickets
2. **Golden Paths** - Standardized, compliant infrastructure by default
3. **Observability by Default** - Every app gets monitoring automatically
4. **Cost Visibility** - Track costs per team and application
5. **Developer Experience** - Simple YAML, instant infrastructure

## Technical Approach

- **Kubernetes Operator** - CRD-based, reconciliation pattern
- **Terraform Integration** - Reuse existing modules, proper state management
- **Platform API** - REST + gRPC for programmatic access
- **Service Catalog** - Track all applications and dependencies

## Learning Objectives

This project is built to deeply learn:
1. Kubernetes Operator patterns (controller-runtime, CRDs, webhooks)
2. Terraform programmatic usage (state management, module generation)
3. AWS service provisioning patterns
4. Platform engineering best practices

## Success Criteria

1. Developer can deploy a complete application with database, cache, and queue in < 5 minutes
2. All infrastructure follows security best practices by default
3. Monitoring and alerting is automatic
4. Service catalog tracks all applications
5. Cost allocation is automatic

## Constraints

- Must work with existing AWS accounts
- Must integrate with existing Terraform modules
- Must support GitOps workflows (ArgoCD)
- Must be self-hosted (no SaaS dependencies)
