# GoPlatform Project Brief

## Overview
GoPlatform is a Kubernetes operator built with kubebuilder v4 that automates infrastructure provisioning for developers. It transforms declarative Application CRDs into fully provisioned Kubernetes infrastructure using in-cluster operators (CloudNativePG, Redis, RabbitMQ).

## Why This Project Exists

### The Purpose
Learn Kubernetes operator patterns deeply by building a real Internal Developer Platform. AI writes production-grade code; the developer reads every line to learn K8s internals, controller-runtime, CRDs, webhooks, and the CNCF ecosystem.

### What It Does
Developers declare what they need via an Application CRD. The operator provisions:
- Kubernetes resources (Deployment, Service, HPA, PDB)
- In-cluster databases (CloudNativePG PostgreSQL)
- In-cluster caches (Redis via Spotahome operator)
- In-cluster queues (RabbitMQ Cluster Operator)
- Credential Secrets with connection strings
- Monitoring resources (ServiceMonitor, PrometheusRule)

## Core Goals

1. **Learn K8s Operator Patterns** - controller-runtime, CRDs, reconciliation, webhooks, multi-version APIs
2. **Learn CNCF Ecosystem** - Prometheus, Kyverno, cert-manager, operator interoperability
3. **Self-Service Infrastructure** - Developers get what they need via simple YAML
4. **Production Patterns** - Build with the same patterns used by cert-manager, ArgoCD, CNPG

## Technical Approach

- **Kubernetes Operator** - CRD-based, level-triggered reconciliation
- **In-Cluster Provisioning** - Leverage existing operators (CNPG, Redis, RabbitMQ)
- **kubectl Interface** - No CLI or REST API needed
- **Pluggable Providers** - InfrastructureProvider interface for future extensibility

## Learning Objectives

1. Kubernetes operator internals (informers, work queues, leader election)
2. CRD design with OpenAPI validation and kubebuilder markers
3. Admission webhooks (validating, mutating, conversion)
4. Multi-version CRD evolution (hub-and-spoke pattern)
5. Prometheus operator ecosystem (ServiceMonitor, PrometheusRule)
6. Policy engines (Kyverno integration)
7. E2E testing for operators (Kind, CI/CD)
8. Drift detection and self-healing reconciliation

## Success Criteria

1. Operator provisions complete infrastructure from a single Application CR
2. All patterns follow production best practices (finalizers, conditions, events, owner refs)
3. Webhooks validate and default Application resources
4. Multi-version CRD works with conversion webhooks
5. Controller exposes custom Prometheus metrics
6. E2E tests validate full lifecycle on real Kind cluster

## Constraints

- Kubernetes-only (no cloud provider APIs, no Terraform)
- kubectl-only interface (no CLI tool, no REST API)
- Must work with Kind for testing
- Self-hosted (no SaaS dependencies)
- Timeline: 1-2 months for remaining milestones
