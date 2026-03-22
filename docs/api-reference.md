# OpenClaw AutoDeploy API Reference

## 1. API Positioning

This API is intended for internal systems or operators. There is no GUI in Phase 1. All long-running actions are asynchronous job-driven operations.

Base path:

```text
/api/v1
```

Recommended auth for Phase 1:

- `Authorization: Bearer <token>`
- internal RBAC roles: `admin`, `operator`, `viewer`

Common headers:

- `X-Request-Id`: caller-generated trace ID, optional but recommended
- `Idempotency-Key`: required for create/redeploy/destroy/start/stop requests

Response style:

- JSON only
- timestamps in ISO 8601 UTC
- errors in stable machine-readable format

CLI parity requirement:

- every operator-facing endpoint in this document must have a first-party `openclawctl` command mapping
- the CLI design is documented in `docs/cli-reference.md`

## 2. Common Objects

### 2.1 Tenant

```json
{
  "id": "6d9ad1f8-9843-4d6c-bb24-c81cbf765412",
  "external_user_id": "user_10001",
  "slug": "acme-user-10001",
  "display_name": "Acme User 10001",
  "status": "ready",
  "created_at": "2026-03-22T08:00:00Z",
  "updated_at": "2026-03-22T08:10:00Z"
}
```

### 2.2 Tenant Profile

```json
{
  "tenant_id": "6d9ad1f8-9843-4d6c-bb24-c81cbf765412",
  "template_id": "7c612cbc-35af-4de0-96af-0f470c7540ca",
  "resource_tier": "standard",
  "route_key": "tenant-10001",
  "model_provider": "openai-compatible",
  "model_name": "gpt-4.1",
  "channels": [
    {
      "type": "discord",
      "channel_id": "1234567890",
      "enabled": true
    }
  ],
  "skills": ["planner", "code-review"],
  "soul_markdown": "# SOUL\n...",
  "memory_markdown": "# MEMORY\n...",
  "extra_files": [
    {
      "path": "prompts/team.md",
      "content": "..."
    }
  ],
  "validation": {
    "is_valid": true,
    "errors": []
  }
}
```

### 2.3 Job

```json
{
  "id": "ee18a31b-28a4-4937-9f42-6b78a0fda48f",
  "tenant_id": "6d9ad1f8-9843-4d6c-bb24-c81cbf765412",
  "job_type": "deploy",
  "status": "pending",
  "attempt_count": 0,
  "requested_by": "operator:system",
  "idempotency_key": "dep-tenant-10001-v1",
  "last_error": null,
  "scheduled_at": "2026-03-22T08:12:00Z",
  "started_at": null,
  "finished_at": null
}
```

### 2.4 Instance

```json
{
  "id": "446c4be5-1cf8-434f-a10c-9730cf5dd7dc",
  "tenant_id": "6d9ad1f8-9843-4d6c-bb24-c81cbf765412",
  "container_id": "f24ec31f4f31",
  "container_name": "openclaw-tenant-10001",
  "image_ref": "registry.local/openclaw-base:1.0.0",
  "status": "running",
  "health_status": "healthy",
  "route_url": "https://tenant-10001.example.com",
  "workspace_path": "/srv/openclaw/tenants/6d9ad1f8-9843-4d6c-bb24-c81cbf765412",
  "config_version": 3,
  "started_at": "2026-03-22T08:13:10Z",
  "stopped_at": null
}
```

## 3. Common Error Format

```json
{
  "error": {
    "code": "TENANT_PROFILE_INVALID",
    "message": "Tenant profile is incomplete and cannot be deployed.",
    "details": {
      "missing_fields": ["model_provider", "tenant_secrets.OPENAI_API_KEY"]
    },
    "request_id": "req_20260322_001"
  }
}
```

Recommended error codes:

- `UNAUTHORIZED`
- `FORBIDDEN`
- `VALIDATION_ERROR`
- `TENANT_NOT_FOUND`
- `TEMPLATE_NOT_FOUND`
- `IMAGE_NOT_AVAILABLE`
- `TENANT_PROFILE_INVALID`
- `IDEMPOTENCY_CONFLICT`
- `JOB_ALREADY_RUNNING`
- `INSTANCE_NOT_FOUND`
- `INSTANCE_NOT_RUNNING`
- `DOCKER_BACKEND_UNAVAILABLE`
- `CAPACITY_EXCEEDED`
- `INTERNAL_ERROR`

## 4. API Endpoints

### 4.1 Health and Infrastructure

#### `GET /healthz`
Returns API liveness.

Response `200`:

```json
{
  "status": "ok"
}
```

#### `GET /readyz`
Returns dependency readiness.

Response `200`:

```json
{
  "status": "ready",
  "checks": {
    "database": "ok",
    "docker": "ok",
    "worker": "ok"
  }
}
```

### 4.2 Tenant Registration and Query

#### `POST /tenants`
Create a tenant shell record.

Request:

```json
{
  "external_user_id": "user_10001",
  "slug": "acme-user-10001",
  "display_name": "Acme User 10001"
}
```

Response `201`:

```json
{
  "tenant": {
    "id": "6d9ad1f8-9843-4d6c-bb24-c81cbf765412",
    "external_user_id": "user_10001",
    "slug": "acme-user-10001",
    "display_name": "Acme User 10001",
    "status": "draft",
    "created_at": "2026-03-22T08:00:00Z",
    "updated_at": "2026-03-22T08:00:00Z"
  }
}
```

#### `GET /tenants/{tenant_id}`
Get a single tenant.

#### `GET /tenants`
List tenants.

Query params:

- `status`
- `external_user_id`
- `slug`
- `page`
- `page_size`

### 4.3 Tenant Profile Management

#### `PUT /tenants/{tenant_id}/profile`
Create or replace the resolved deployment profile.

Request:

```json
{
  "template_id": "7c612cbc-35af-4de0-96af-0f470c7540ca",
  "resource_tier": "standard",
  "route_key": "tenant-10001",
  "model_provider": "openai-compatible",
  "model_name": "gpt-4.1",
  "channels": [
    {
      "type": "discord",
      "channel_id": "1234567890",
      "enabled": true
    }
  ],
  "skills": ["planner", "code-review"],
  "soul_markdown": "# SOUL\ncustom persona",
  "memory_markdown": "# MEMORY\nbootstrap memory",
  "extra_files": [
    {
      "path": "prompts/team.md",
      "content": "team guidance"
    }
  ]
}
```

Response `200`:

```json
{
  "profile": {
    "tenant_id": "6d9ad1f8-9843-4d6c-bb24-c81cbf765412",
    "validation": {
      "is_valid": true,
      "errors": []
    }
  }
}
```

#### `GET /tenants/{tenant_id}/profile`
Read current profile.

#### `POST /tenants/{tenant_id}/profile/validate`
Validate profile and secret completeness without deploying.

Response `200`:

```json
{
  "validation": {
    "is_valid": false,
    "errors": [
      {
        "field": "tenant_secrets.OPENAI_API_KEY",
        "message": "missing required secret"
      }
    ]
  }
}
```

### 4.4 Tenant Secret Management

#### `PUT /tenants/{tenant_id}/secrets/{secret_key}`
Create or rotate one secret.

Request:

```json
{
  "value": "sk-live-xxxxx",
  "secret_type": "api_key"
}
```

Response `200`:

```json
{
  "secret": {
    "tenant_id": "6d9ad1f8-9843-4d6c-bb24-c81cbf765412",
    "secret_key": "OPENAI_API_KEY",
    "version": 2,
    "value_fingerprint": "9f86d081",
    "status": "active"
  }
}
```

#### `GET /tenants/{tenant_id}/secrets`
List secret metadata only, never actual values.

#### `DELETE /tenants/{tenant_id}/secrets/{secret_key}`
Revoke a secret if allowed by policy.

### 4.5 Template and Image Catalog

#### `GET /templates`
List deployment templates.

#### `GET /templates/{template_id}`
Get one deployment template.

#### `GET /images`
List available base images.

Query params:

- `runtime_family`
- `status`

### 4.6 Deployment Lifecycle

#### `POST /tenants/{tenant_id}/deploy`
Create a deployment job.

Request:

```json
{
  "reason": "initial deployment",
  "wait": false
}
```

Response `202`:

```json
{
  "job": {
    "id": "ee18a31b-28a4-4937-9f42-6b78a0fda48f",
    "tenant_id": "6d9ad1f8-9843-4d6c-bb24-c81cbf765412",
    "job_type": "deploy",
    "status": "pending"
  }
}
```

Rules:

- requires valid profile
- requires required secrets present
- rejects if another lifecycle job is already active for the tenant

#### `POST /tenants/{tenant_id}/redeploy`
Create a redeploy job using the latest profile.

Request:

```json
{
  "reason": "profile updated",
  "strategy": "replace"
}
```

Allowed strategies:

- `replace`
- `recreate`

#### `POST /tenants/{tenant_id}/stop`
Stop a running tenant instance.

#### `POST /tenants/{tenant_id}/start`
Start the current stopped tenant instance if reusable; otherwise the backend may create a fresh instance.

#### `POST /tenants/{tenant_id}/restart`
Restart a running instance.

#### `DELETE /tenants/{tenant_id}/deployment`
Destroy the active deployment.

Request:

```json
{
  "destroy_workspace": false,
  "destroy_volume": false,
  "reason": "tenant archived"
}
```

This endpoint destroys the active container and marks the instance as destroyed. Workspace and volume deletion should be explicit to avoid accidental data loss.

### 4.7 Job Query

#### `GET /jobs/{job_id}`
Get one job.

#### `GET /jobs`
List jobs.

Query params:

- `tenant_id`
- `job_type`
- `status`
- `page`
- `page_size`

### 4.8 Instance Query

#### `GET /tenants/{tenant_id}/instance`
Get current active instance.

#### `GET /tenants/{tenant_id}/instances`
Get instance history.

#### `GET /instances/{instance_id}`
Get one instance by ID.

### 4.9 Audit and Diagnostics

#### `GET /tenants/{tenant_id}/audit-logs`
List audit records.

#### `GET /tenants/{tenant_id}/runtime-config`
Return the latest resolved config snapshot with secrets masked.

Example response:

```json
{
  "tenant_id": "6d9ad1f8-9843-4d6c-bb24-c81cbf765412",
  "config_version": 3,
  "image_ref": "registry.local/openclaw-base:1.0.0",
  "resource_tier": "standard",
  "channels": [
    {
      "type": "discord",
      "channel_id": "1234567890",
      "enabled": true
    }
  ],
  "skills": ["planner", "code-review"],
  "secrets": {
    "OPENAI_API_KEY": "masked"
  }
}
```

## 5. Idempotency Rules

The following endpoints require `Idempotency-Key`:

- `POST /tenants/{tenant_id}/deploy`
- `POST /tenants/{tenant_id}/redeploy`
- `POST /tenants/{tenant_id}/stop`
- `POST /tenants/{tenant_id}/start`
- `POST /tenants/{tenant_id}/restart`
- `DELETE /tenants/{tenant_id}/deployment`

Behavior:

- same key + same semantic request returns original job
- same key + different request body returns `409 IDEMPOTENCY_CONFLICT`

## 6. State Transition Rules

Canonical transitions:

- `draft -> ready`
- `ready -> deploying`
- `deploying -> running`
- `deploying -> failed`
- `running -> stopped`
- `stopped -> running`
- `running -> degraded`
- `degraded -> running`
- `running|stopped|failed -> archived`

The API should reject invalid transitions with `409 VALIDATION_ERROR`.

## 7. Operational Recommendations

- use pagination for all list endpoints
- default page size 20, max 100
- log every mutating request into `audit_logs`
- mask secrets in all API payloads and logs
- return `202` for async actions, not `200`
- keep API names stable and version under `/api/v1`

## 8. Minimal First-Release Endpoint Set

If the first milestone needs a smaller surface, implement these first:

1. `POST /tenants`
2. `PUT /tenants/{tenant_id}/profile`
3. `PUT /tenants/{tenant_id}/secrets/{secret_key}`
4. `POST /tenants/{tenant_id}/profile/validate`
5. `POST /tenants/{tenant_id}/deploy`
6. `GET /jobs/{job_id}`
7. `GET /tenants/{tenant_id}/instance`
8. `POST /tenants/{tenant_id}/stop`
9. `POST /tenants/{tenant_id}/start`
10. `DELETE /tenants/{tenant_id}/deployment`
