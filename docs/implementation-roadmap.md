# OpenClaw AutoDeploy Implementation Roadmap

## 1. Delivery Strategy

The project should be implemented in four milestones. The first goal is to prove the end-to-end path on the existing Ubuntu 24.04 test VM with Docker already installed, while keeping production architecture decisions compatible with later expansion.

## 2. Milestone Plan

### Milestone 0 - Project Bootstrap

Objective:

- create the repository skeleton
- initialize Go module
- add configuration loading
- add PostgreSQL migrations framework
- add local development scripts
- add CLI skeleton and shared API client package
- add systemd and Traefik deployment templates

Exit criteria:

- API binary starts
- worker binary starts
- DB migrations run successfully
- health endpoint returns healthy with DB and Docker checks stubbed or real

### Milestone 1 - Tenant Registry and Validation

Objective:

- implement tenant CRUD
- implement tenant profile CRUD
- implement encrypted secret storage
- implement profile validation rules
- implement template catalog and image catalog read APIs
- implement tenant/profile/secret/template/image CLI commands

Exit criteria:

- a caller can create a tenant, store secrets, write profile, and receive a pass/fail validation result
- audit logs exist for all mutating actions

### Milestone 2 - Async Deploy and Lifecycle Control

Objective:

- implement job table and worker loop
- implement deploy/start/stop/restart/destroy lifecycle jobs
- implement workspace rendering
- implement Docker container creation
- implement health check wait and instance persistence
- implement deployment/job/instance/audit CLI commands

Exit criteria:

- one tenant can be deployed from DB data to a running OpenClaw container
- worker updates job and instance state correctly
- stop/start/redeploy paths work on the test VM

### Milestone 3 - Reconciliation and Operations Hardening

Objective:

- implement reconciliation loop
- implement capacity guardrails
- add metrics and structured logs
- add retry and dead-letter style failure handling
- add backup/export for tenant workspace metadata

Exit criteria:

- the system detects drift between DB and Docker state
- degraded containers can be surfaced and repaired
- operational telemetry is sufficient for support

## 3. Recommended Build Order by Module

1. `internal/config`
2. `internal/db`
3. `internal/api`
4. `internal/cli`
5. `internal/service`
6. `internal/template`
7. `internal/renderer`
8. `internal/docker`
9. `internal/jobs`
10. `internal/reconcile`
11. `internal/audit`

This order keeps the domain and persistence layer stable before container orchestration code begins.

## 4. Database Delivery Plan

Initial migrations should create:

- `tenants`
- `tenant_profiles`
- `tenant_secrets`
- `deployment_templates`
- `image_catalog`
- `deployment_jobs`
- `tenant_instances`
- `audit_logs`

Recommended migration policy:

- forward-only migrations
- never modify old migration files after merge
- treat enum-like statuses as checked text fields in early phase unless a strict enum is clearly needed

## 5. Validation Rules to Implement Early

The validator should fail deployment when any of the following is missing:

- tenant exists
- selected template exists and is enabled
- selected image exists and is active
- required model provider and model name exist
- required secrets exist
- route key is unique if routing is enabled
- file payload sizes are within configured limits
- resource tier is valid

Suggested guardrails:

- `SOUL.md` max 256 KB
- `memory.md` max 256 KB
- one extra file max 256 KB
- combined rendered payload max 2 MB

## 6. Test Strategy

### Unit Tests

- profile validation
- secret masking
- template rendering
- state transitions
- idempotency handling

### Integration Tests

- PostgreSQL repository tests
- Docker adapter tests against local Docker
- full deploy flow on a disposable tenant

### End-to-End Tests

- create tenant -> validate -> deploy -> running -> stop -> start -> destroy
- redeploy after secret rotation
- recovery after container crash

## 7. Environment Plan

### Local Development

- `docker compose` for PostgreSQL + Traefik + sample OpenClaw base image
- API and worker can run from host machine

### Test VM

- systemd services for API and worker
- Docker Engine local
- PostgreSQL local or separate instance
- tenant workspace root on attached disk path if available

### Later Production

- split DB and Docker host if scale requires it
- add remote image registry
- add external secret manager if compliance requires it

## 8. Runtime File Policy

The worker should generate a versioned runtime snapshot for every deployment attempt:

```text
/srv/openclaw/tenants/<tenant_id>/releases/<config_version>/
```

And expose a stable symlink or pointer for current runtime:

```text
/srv/openclaw/tenants/<tenant_id>/current -> releases/3
```

This simplifies rollback and debugging.

## 9. Suggested Rollback Strategy

For Phase 1, prefer `replace` deployment semantics:

1. render new release
2. create new container
3. wait for health
4. switch route label target if needed
5. stop old container

If OpenClaw routing model makes blue/green too heavy for the first release, use `recreate` with a short maintenance window and keep the last successful release directory for fast recovery.

## 10. Risks and Controls

### Risk: Tenant config is incomplete
- Control: strict validate endpoint before deploy

### Risk: Secret leakage in logs
- Control: centralized masking helper and log review tests

### Risk: Docker and DB state drift
- Control: reconciliation loop plus audit trail

### Risk: Host capacity exhaustion
- Control: hard admission checks before scheduling deploy job

### Risk: OpenClaw startup is slow or non-deterministic
- Control: health wait timeout, startup retries, explicit failed state

## 11. Acceptance Checklist for Document Approval

Implementation should begin only after confirming:

1. Go-based control plane is accepted.
2. One shared base image plus runtime injection is accepted.
3. Async API + `ultraworker` job model is accepted.
4. PostgreSQL is the only source of truth for tenant deployment state.
5. Traefik-based routing is accepted if external access is required.
6. The first milestone will target one Ubuntu host, not multi-host scheduling.
7. `openclawctl` will provide complete operator coverage for all approved APIs.

## 12. CLI Delivery Principle

CLI delivery should track API delivery, not lag behind it by milestones. For every approved endpoint group, the corresponding `openclawctl` command group should be implemented in the same milestone.

## 13. Recommended Next Step After Approval

After document approval, the first implementation task should be:

- initialize repository structure
- create migrations
- implement `POST /tenants`, `PUT /tenants/{tenant_id}/profile`, `PUT /tenants/{tenant_id}/secrets/{secret_key}`, and `POST /tenants/{tenant_id}/profile/validate`

Only after these are stable should container deployment code be added.
