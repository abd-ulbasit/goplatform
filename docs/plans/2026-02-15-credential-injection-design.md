# Credential Injection Design

**Date:** 2026-02-15
**Status:** Approved
**Approach:** Controller-side env injection (Approach A)

## Problem

Users must manually wire `secretKeyRef` entries for every infrastructure credential (DB host, port, user, password, database). This is verbose, error-prone, and defeats the purpose of a platform abstraction.

## Solution

The controller auto-injects well-known environment variables into workload containers based on which infrastructure is declared in the Application spec.

## CRD Changes

Two new fields on `WorkloadSpec`:

- `injectCredentials` (`*bool`, default `true`) — opt-out toggle for auto-injection
- `envFrom` (`[]corev1.EnvFromSource`) — bulk-mount external Secrets/ConfigMaps

## Injected Env Vars

### Database (postgres)
`DATABASE_URL`, `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`

### Database (mysql)
`DATABASE_URL`, `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_DATABASE`

### Cache (redis)
`REDIS_URL`, `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`

### Queue (rabbitmq)
`AMQP_URL`, `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USER`, `RABBITMQ_PASSWORD`

All values come from `secretKeyRef` pointing to the provider-created credential Secrets.

## Precedence

User-defined env vars in `spec.workload.env` always win. The controller skips injecting any var the user already defined.

## Secret Naming

Unchanged from existing KubernetesProvider conventions:
- Database: `{app.Name}-db-credentials`
- Cache: `{app.Name}-cache-credentials`
- Queue: `{app.Name}-queue-credentials`
