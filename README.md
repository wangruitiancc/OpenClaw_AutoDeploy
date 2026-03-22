# OpenClaw AutoDeploy

Backend control plane for provisioning per-tenant OpenClaw containers from PostgreSQL-backed tenant data.

[English](README.md) | [中文](docs/zh/architecture-design.md)

---

## Components

- **control-plane-api** — REST API server on `:8080`
- **ultraworker** — Background job processor
- **openclawctl** — CLI for tenant and deployment management
- PostgreSQL schema with full CRUD for tenants, profiles, secrets, jobs, and instances

## Authentication

All API endpoints under `/api/v1/*` require a Bearer token:

```
Authorization: Bearer <token>
```

Configure the token via `security.static_token` in `config.yaml` or the `OPENCLAW_SECURITY_STATIC_TOKEN` environment variable.

Public endpoints (no auth required):
- `GET /healthz` — liveness check
- `GET /metrics` — Prometheus metrics

## Prometheus Metrics

Exposed at `GET /metrics` (public):

```
# HELP openclaw_up Control plane is up.
# TYPE openclaw_up gauge
openclaw_up 1
# HELP openclaw_tenants_total Total number of tenants.
# TYPE openclaw_tenants_total gauge
openclaw_tenants_total 3
# HELP openclaw_containers_running Number of running tenant containers.
# TYPE openclaw_containers_running gauge
openclaw_containers_running 1
# HELP openclaw_jobs_pending Number of pending jobs.
# TYPE openclaw_jobs_pending gauge
openclaw_jobs_pending 0
# HELP openclaw_jobs_succeeded_total Total succeeded jobs.
# TYPE openclaw_jobs_succeeded_total counter
openclaw_jobs_succeeded_total 17
# HELP openclaw_jobs_failed_total Total failed jobs.
# TYPE openclaw_jobs_failed_total counter
openclaw_jobs_failed_total 2
```

## Multi-Tenant Routing

Each tenant container is exposed via Traefik at `{route_key}.{base_domain}` (e.g., `smoke-user-003.localtest.me`).

Traffic path: `Host header → Traefik (port 80) → host.docker.internal:18789 → OpenClaw gateway`

## Local Bring-Up

Start the local stack:

```bash
docker compose -f deploy/docker-compose.local.yaml up --build
```

Check health:

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
curl http://127.0.0.1:8080/metrics
```

## Configuration

Key config fields (`config.yaml`):

```yaml
api:
  listen_addr: ":8080"

database:
  url: "postgres://user:pass@host:5432/db?sslmode=disable"

runtime:
  workspace_root: "/tmp/openclaw-autodeploy/tenants"
  docker_network: "bridge"        # used for container --network flag
  image_registry_prefix: "ghcr.io/openclaw"  # prefix for short image refs
  bootstrap_mount_path: "/bootstrap"
  data_mount_path: "/data"
  base_domain: "localtest.me"     # tenant routing: {route_key}.localtest.me
  health_poll_interval: "3s"
  health_timeout: "60s"

security:
  master_key: "..."               # used to encrypt LLM API keys
  static_token: "your-bearer-token"

worker:
  name: "ultraworker"
  poll_interval: "15s"
  heartbeat_ttl: "45s"
  job_heartbeat_ttl: "20s"       # TTL before a stalled job is re-claimed
```

Environment variable overrides: `OPENCLAW_*` prefix (e.g., `OPENCLAW_SECURITY_STATIC_TOKEN`).

## Image Registry Prefix

Configure `runtime.image_registry_prefix` to resolve short image references:

- `"docker.io/library"` → `nginx:alpine` becomes `docker.io/library/nginx:alpine`
- `"ghcr.io/openclaw"` → `openclaw:latest` becomes `ghcr.io/openclaw/openclaw:latest`
- Leave empty if `image_catalog.image_ref` already stores fully qualified references

## Deploy a Tenant

1. Create tenant:
   ```bash
   openclawctl tenant create --external-user-id user_10001 --slug user-10001 --display-name "User 10001"
   ```

2. Set secrets:
   ```bash
   openclawctl secret set --tenant <ID> OPENAI_API_KEY --from-env OPENAI_API_KEY
   ```

3. Set profile:
   ```bash
   openclawctl profile set --tenant <ID> --template <TEMPLATE_ID> --route-key user-10001 --model-provider openai-compatible --model-name gpt-4.1
   ```

4. Validate and deploy:
   ```bash
   openclawctl profile validate --tenant <ID>
   openclawctl deployment deploy --tenant <ID>
   ```

## Rollback on Deploy Failure

When deploying a new tenant container, the previous container is stopped first. If the new container fails to start or pass health checks, the previous container is restarted automatically, preserving tenant availability.

## LLM API Key Shared Pool

LLM API keys are stored encrypted in the database and allocated to tenants via the shared pool:

1. Add a provider: `openclawctl provider add --name minimax`
2. Add an API key: `openclawctl apikey add --provider <ID> --key-file /path/to/key`
3. Allocate to tenant: `openclawctl tenant allocate-llm-key --tenant <ID> --provider <ID> --key-id <KEY_ID>`

The allocated key is injected as `{PROVIDER_NAME}_API_KEY` env var into the tenant container.

---

## Documentation (文档)

| Language | Architecture | API Reference | CLI Reference | Roadmap |
|----------|-------------|---------------|---------------|---------|
| English | [docs/en/architecture-design.md](docs/en/architecture-design.md) | [docs/en/api-reference.md](docs/en/api-reference.md) | [docs/en/cli-reference.md](docs/en/cli-reference.md) | [docs/en/implementation-roadmap.md](docs/en/implementation-roadmap.md) |
| 中文 | [docs/zh/architecture-design.md](docs/zh/architecture-design.md) | [docs/zh/api-reference.md](docs/zh/api-reference.md) | [docs/zh/cli-reference.md](docs/zh/cli-reference.md) | [docs/zh/implementation-roadmap.md](docs/zh/implementation-roadmap.md) |
