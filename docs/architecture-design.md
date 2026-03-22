# OpenClaw AutoDeploy Architecture Design

## 1. Overview

This project provides a backend-only control plane that automatically provisions dedicated OpenClaw containers for users based on PostgreSQL records. The system exposes all capabilities through APIs and runs as Linux background services on Ubuntu 24.04.

The target workflow is:

1. The operator system or upstream form writes user base data and preset selections into PostgreSQL.
2. The control plane validates the tenant configuration and creates a deployment job.
3. `ultraworker` resolves the tenant's base image, runtime template, secrets, and seed files.
4. The worker creates the tenant workspace, renders configuration, starts the container, and waits for health confirmation.
5. The control plane records lifecycle status and exposes query, restart, stop, redeploy, and destroy APIs.

## 2. Goals

- Automatically create one OpenClaw runtime per tenant/user.
- Derive runtime configuration from PostgreSQL by `user_id`.
- Support tenant-specific seed data such as model API keys, channel IDs, skill lists, `SOUL.md`, `memory.md`, and other startup assets.
- Standardize image selection, config rendering, container startup, health checking, and lifecycle recovery.
- Run without a GUI; provide a clean API and CLI surface for external systems and operators.
- Keep the first release simple enough for a single Ubuntu host while leaving room for later horizontal expansion.

## 3. Non-Goals for Phase 1

- No web UI.
- No Kubernetes.
- No per-tenant custom image build pipeline unless runtime templating cannot satisfy a requirement.
- No self-service billing or customer-facing portal.
- No multi-host cluster scheduling in the first release.

## 4. Recommended Technical Baseline

Because the repository is greenfield, the recommended baseline is:

- Language: Go 1.24+
- HTTP framework: `chi` or `gin` (prefer `chi` for lighter control plane APIs)
- Database: PostgreSQL 16+
- DB access: `sqlc` + `pgx`
- Migrations: `golang-migrate`
- Docker control: Docker Engine API via Go Docker SDK
- Reverse proxy: Traefik v3 with Docker provider
- Service runtime: `systemd`
- Config format: YAML + environment variables
- Secrets at rest: PostgreSQL encrypted columns (`pgcrypto`) plus an application master key from environment or file-based secret

This stack fits Ubuntu background services well, allows a single static binary for the control plane, and keeps deployment and observability straightforward.

## 5. High-Level Architecture

Core components:

1. `control-plane-api`
   - Exposes REST APIs.
   - Validates tenant data.
   - Creates deployment and lifecycle jobs.
   - Provides instance, job, and audit query endpoints.

2. `openclawctl`
   - Operator-facing CLI binary.
   - Wraps the full API surface for scripts, CI, and manual operations.
   - Supports declarative apply mode and imperative lifecycle commands.
   - Outputs table or JSON for both humans and automation.

3. `ultraworker`
   - Pulls pending jobs from PostgreSQL.
   - Locks jobs with `FOR UPDATE SKIP LOCKED`.
   - Resolves image, template set, secrets, and runtime parameters.
   - Creates local workspace and Docker resources.
   - Starts/stops/restarts/redeploys/destroys tenant containers.
   - Performs health verification and reconciliation.

4. PostgreSQL
   - Source of truth for tenants, templates, deployment jobs, instances, secret metadata, and audit logs.

5. Docker Engine
   - Runs OpenClaw tenant containers.
   - Hosts shared networks, named volumes, and per-tenant mounts.

6. Traefik
   - Routes inbound traffic to tenant containers using labels.
   - Avoids per-container host port management.

7. Tenant Workspace Store
   - Local filesystem path such as `/srv/openclaw/tenants/<tenant_id>/`.
   - Stores rendered config, `SOUL.md`, `memory.md`, mounted working files, and generated env files.

## 5.1 CLI Design Principle

The system must not rely on a GUI for any operational path. Every operator-facing capability exposed by the API must also be reachable from a first-party CLI.

CLI requirements:

- full lifecycle parity with API endpoints
- safe secret input from environment, file, or stdin only
- JSON output mode for automation
- non-interactive flags for CI/CD and shell scripts
- optional declarative `apply` workflow for tenant onboarding

## 6. Deployment Model

Each tenant gets:

- one logical tenant record in PostgreSQL
- one runtime configuration snapshot
- one dedicated OpenClaw container
- one workspace directory on disk
- one named Docker volume for persistent runtime data if needed
- one route label set in Traefik

Recommended host layout:

- `control-plane-api` as systemd service
- `ultraworker` as systemd service
- PostgreSQL on local host or managed service
- Docker Engine on same host for Phase 1
- Traefik on same host attached to `traefik-public` network

## 7. Provisioning Strategy

### 7.1 Base Principle

Prefer a shared OpenClaw base image plus runtime injection over per-tenant image builds.

Why:

- faster tenant provisioning
- fewer images to manage
- easier rollbacks
- easier security patching
- supports tenant-specific content through mounted files and env injection

### 7.2 Provisioning Inputs

For each tenant, the worker resolves:

- `tenant_id`
- selected deployment template
- selected base image tag
- LLM provider config and API key
- channels and channel credentials
- skill list
- `SOUL.md`
- `memory.md`
- optional extra startup files
- CPU/memory tier
- domain/subdomain or route key

### 7.3 Rendered Outputs

The worker writes a tenant runtime bundle into the workspace, for example:

```text
/srv/openclaw/tenants/<tenant_id>/
  config/
    app.env
    channels.json
    skills.json
    SOUL.md
    memory.md
    metadata.json
  logs/
  data/
```

### 7.4 Container Creation Flow

1. Validate tenant record completeness.
2. Create deployment job in status `pending`.
3. Worker locks the job and marks it `running`.
4. Resolve base image and deployment template.
5. Materialize tenant workspace and config snapshot.
6. Create or reuse Docker network and volume.
7. Create container with:
   - tenant labels
   - mounted workspace
   - resource limits
   - restart policy
   - health check
8. Start container.
9. Wait for health check success.
10. Update instance status to `running` and store snapshot/version.

### 7.5 Runtime Labels

Recommended labels:

- `app=openclaw`
- `service=tenant-runtime`
- `tenant.id=<tenant_id>`
- `tenant.slug=<tenant_slug>`
- `template.id=<template_id>`
- `image.tag=<image_tag>`
- `managed.by=openclaw-autodeploy`

Traefik labels should follow the same tenant identity.

## 8. Data Model

### 8.1 Main Tables

#### `tenants`
- tenant identity and business status
- creation source and owner metadata

Key fields:

- `id` UUID PK
- `external_user_id` text unique
- `slug` text unique
- `display_name` text
- `status` text
- `created_at`, `updated_at`

#### `tenant_profiles`
- resolved business input for deployment

Key fields:

- `tenant_id` UUID PK/FK
- `model_provider` text
- `model_name` text
- `channels` jsonb
- `skills` jsonb
- `soul_markdown` text
- `memory_markdown` text
- `extra_files` jsonb
- `resource_tier` text
- `route_key` text
- `template_id` UUID
- `is_valid` boolean
- `validation_errors` jsonb

#### `image_catalog`
- available base images

Key fields:

- `id` UUID PK
- `image_ref` text
- `version` text
- `runtime_family` text
- `status` text
- `default_template_id` UUID

#### `deployment_templates`
- preset combinations for tenant types

Key fields:

- `id` UUID PK
- `code` text unique
- `name` text
- `description` text
- `base_image_policy` jsonb
- `default_channels` jsonb
- `default_skills` jsonb
- `default_files` jsonb
- `resource_policy` jsonb
- `enabled` boolean

#### `tenant_secrets`
- encrypted sensitive values

Key fields:

- `id` UUID PK
- `tenant_id` UUID FK
- `secret_type` text
- `secret_key` text
- `encrypted_value` bytea
- `value_fingerprint` text
- `version` int
- `status` text

#### `deployment_jobs`
- async job queue and execution state

Key fields:

- `id` UUID PK
- `tenant_id` UUID FK
- `job_type` text
- `requested_by` text
- `idempotency_key` text
- `status` text
- `payload` jsonb
- `attempt_count` int
- `last_error` text
- `scheduled_at` timestamptz
- `started_at` timestamptz
- `finished_at` timestamptz

#### `tenant_instances`
- current and historical runtime instances

Key fields:

- `id` UUID PK
- `tenant_id` UUID FK
- `deployment_job_id` UUID FK
- `container_id` text
- `container_name` text
- `image_ref` text
- `status` text
- `host_node` text
- `workspace_path` text
- `volume_name` text
- `route_url` text
- `health_status` text
- `config_version` int
- `started_at` timestamptz
- `stopped_at` timestamptz

#### `audit_logs`
- immutable operation traces

Key fields:

- `id` UUID PK
- `tenant_id` UUID null
- `actor` text
- `action` text
- `target_type` text
- `target_id` text
- `request_id` text
- `details` jsonb
- `created_at` timestamptz

### 8.2 Suggested Status Enums

Tenant status:

- `draft`
- `ready`
- `deploying`
- `running`
- `stopped`
- `failed`
- `archived`

Job status:

- `pending`
- `running`
- `succeeded`
- `failed`
- `cancelled`

Instance status:

- `creating`
- `starting`
- `running`
- `degraded`
- `stopping`
- `stopped`
- `destroyed`
- `failed`

## 9. API/Worker Interaction Pattern

- APIs never directly block on long Docker operations.
- APIs write jobs into `deployment_jobs`.
- `ultraworker` executes jobs asynchronously.
- Short operations can optionally support `wait=true` with a small timeout for internal operators, but the canonical mode remains async.

This keeps API latency stable and makes retries, reconciliation, and auditing easier.

## 10. Reconciliation and Recovery

`ultraworker` must run a reconciliation loop every 15 to 30 seconds:

- find jobs stuck in `running`
- compare DB instance state with Docker actual state
- detect missing containers
- detect unhealthy containers
- update heartbeat and health status
- optionally auto-restart based on policy

Recovery rules:

- if container create failed before start, mark job failed and keep workspace for inspection
- if container exists but DB says absent, reconcile into `degraded`
- if DB says running but Docker says exited, record exit code and apply restart policy

## 11. Security Design

### 11.1 Secrets

- Never store tenant API keys in plaintext.
- Encrypt at rest using `pgcrypto` or application-level envelope encryption.
- Decrypt only inside worker execution path.
- Prefer mounted env files or files in a root-owned path over command-line arguments.
- Mask secrets in logs and API responses.

### 11.2 API Access

Phase 1 recommendation:

- internal API only
- token or mTLS between caller and control plane
- RBAC roles: `admin`, `operator`, `viewer`

### 11.3 Container Hardening

Default container constraints:

- `CapDrop=ALL`
- `no-new-privileges=true`
- read-only root filesystem when OpenClaw allows it
- dedicated tmpfs for `/tmp`
- CPU and memory limits per tenant tier
- PID limits
- bounded log rotation

### 11.4 File Safety

- sanitize tenant-provided filenames
- only allow writes under tenant workspace root
- version rendered config snapshots
- record checksum of generated files for supportability

## 12. Resource Tiers

Suggested starting tiers:

| Tier | CPU | RAM | PIDs | Notes |
| --- | --- | --- | --- | --- |
| `starter` | 0.5 core | 512 MB | 128 | test/demo |
| `standard` | 1 core | 2 GB | 256 | default production |
| `pro` | 2 cores | 4 GB | 512 | heavier skills/channels |
| `enterprise` | 4 cores | 8 GB | 1024 | reserved |

These are policy values in the control plane, not hardcoded in callers.

## 13. Suggested Project Layout

```text
cmd/
  control-plane-api/
  openclawctl/
  ultraworker/
internal/
  api/
  cli/
  auth/
  config/
  db/
  docker/
  jobs/
  renderer/
  reconcile/
  service/
  template/
  audit/
migrations/
deploy/
  systemd/
  traefik/
docs/
```

## 14. Linux Service Plan

Recommended systemd services:

- `openclaw-control-plane.service`
- `openclaw-ultraworker.service`

Both depend on:

- network-online
- Docker
- PostgreSQL availability

The worker should be restartable independently of the API.

## 15. Observability

Minimum required telemetry:

- structured logs with `request_id`, `job_id`, `tenant_id`, `container_id`
- metrics:
  - job success/failure count
  - deployment duration
  - running container count
  - unhealthy container count
  - reconciliation repairs
- health endpoints:
  - API liveness
  - DB connectivity
  - Docker connectivity
  - worker queue lag

## 16. Capacity Notes for Phase 1

This first design assumes a single Docker host. Capacity depends on tenant tier mix, but the control plane must explicitly reject provisioning when host memory or CPU thresholds are exceeded.

For a test host, start with:

- max 5 to 10 concurrent tenant containers
- alert threshold at 70% memory and 70% CPU
- no automatic overcommit in first release

## 17. Open Questions to Confirm Before Implementation

These do not block documentation approval, but should be frozen before coding:

1. Whether one tenant always maps to one end user or can represent a team.
2. Whether each tenant needs a dedicated domain/subdomain.
3. Whether `SOUL.md` and `memory.md` are full replacements or partial templates.
4. Whether secrets are supplied once or can be rotated through API after deployment.
5. Whether OpenClaw itself exposes a native health endpoint; otherwise a custom readiness strategy is needed.

## 18. Phase 1 Recommendation

Approve a control plane made of two services:

- synchronous REST API service
- asynchronous `ultraworker`

Back it with PostgreSQL as the source of truth, use one shared OpenClaw base image family, inject tenant runtime data through workspace rendering and mounted files, and manage lifecycle through Docker Engine plus a reconciliation loop.
