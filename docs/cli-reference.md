# OpenClaw AutoDeploy CLI Reference

## 1. Positioning

`openclawctl` is the first-party command-line client for the OpenClaw AutoDeploy control plane. It is the operator interface for environments without a GUI and must cover the full approved backend capability.

Design goals:

- full API coverage through CLI
- script-friendly behavior for CI/CD and shell automation
- human-friendly output for operators
- safe secret input paths
- support both imperative and declarative workflows

## 2. Binary and Invocation

Recommended binary name:

```text
openclawctl
```

Basic form:

```bash
openclawctl [global options] <resource> <command> [flags]
```

Examples:

```bash
openclawctl tenant list
openclawctl profile validate --tenant tenant_123
openclawctl deployment deploy --tenant tenant_123 --wait
```

## 3. Operating Modes

### 3.1 Imperative Mode

Operators run a direct command for one action.

Examples:

```bash
openclawctl secret set --tenant tenant_123 OPENAI_API_KEY --from-env OPENAI_API_KEY
openclawctl deployment restart --tenant tenant_123
```

### 3.2 Declarative Mode

Operators apply a tenant manifest file that describes tenant identity, profile, files, and secret references.

Examples:

```bash
openclawctl apply -f tenant.yaml
openclawctl apply -f tenant.yaml --validate-only
```

Declarative mode is the preferred path for repeatable onboarding and CI automation.

## 4. Global Options

All commands should support:

- `--server`: control plane base URL
- `--token`: bearer token; discouraged in shell history
- `--token-file`: read token from file
- `--profile`: named CLI profile
- `--output table|json|yaml`
- `--request-id`: caller trace ID
- `--timeout`: request timeout
- `--verbose`: show request metadata
- `--no-color`: disable ANSI coloring

Recommended environment variables:

- `OPENCLAWCTL_SERVER`
- `OPENCLAWCTL_TOKEN`
- `OPENCLAWCTL_PROFILE`
- `OPENCLAWCTL_OUTPUT`

## 5. Authentication and Local Config

Suggested local config file:

```text
~/.config/openclawctl/config.yaml
```

Suggested commands:

```bash
openclawctl config init
openclawctl config set server https://control-plane.internal
openclawctl config set token-file ~/.config/openclawctl/token
openclawctl config view
```

If login flow is later required, add:

```bash
openclawctl auth login
openclawctl auth logout
openclawctl auth whoami
```

Phase 1 can work with pre-issued internal bearer tokens and does not require interactive auth.

## 6. Command Taxonomy

### 6.1 Health and Diagnostics

```bash
openclawctl health
openclawctl ready
openclawctl doctor
openclawctl version
```

Notes:

- `health` maps to API liveness
- `ready` maps to dependency readiness
- `doctor` can combine API readiness with local config validation

### 6.2 Tenant Commands

```bash
openclawctl tenant create --external-user-id user_10001 --slug acme-user-10001 --display-name "Acme User 10001"
openclawctl tenant get --tenant 6d9ad1f8-9843-4d6c-bb24-c81cbf765412
openclawctl tenant list --status ready
```

Command design rules:

- `--tenant` accepts tenant UUID
- `--slug` filter is allowed where listing makes sense
- destructive operations require confirmation unless `--yes` is supplied

### 6.3 Profile Commands

```bash
openclawctl profile get --tenant tenant_123
openclawctl profile set --tenant tenant_123 --template tpl_standard --tier standard --route-key tenant-10001 --model-provider openai-compatible --model-name gpt-4.1 --channels-file channels.json --skills-file skills.json --soul-file SOUL.md --memory-file memory.md
openclawctl profile validate --tenant tenant_123
```

Recommended file-oriented flags:

- `--channels-file`
- `--skills-file`
- `--soul-file`
- `--memory-file`
- `--extra-file path=localfile`

### 6.4 Secret Commands

```bash
openclawctl secret list --tenant tenant_123
openclawctl secret set --tenant tenant_123 OPENAI_API_KEY --from-env OPENAI_API_KEY
openclawctl secret set --tenant tenant_123 DISCORD_BOT_TOKEN --from-file ./discord.token
printf '%s' "$TOKEN" | openclawctl secret set --tenant tenant_123 DISCORD_BOT_TOKEN --stdin
openclawctl secret delete --tenant tenant_123 OPENAI_API_KEY --yes
```

Secret safety rules:

- no `--value` flag for secrets
- accept secrets only from `--from-env`, `--from-file`, or `--stdin`
- command output never prints secret values

### 6.5 Template and Image Catalog Commands

```bash
openclawctl template list
openclawctl template get --template tpl_standard
openclawctl image list --status active
openclawctl image list --image-ref registry.local/openclaw-base:1.0.0
```

### 6.6 Deployment Lifecycle Commands

```bash
openclawctl deployment deploy --tenant tenant_123 --idempotency-key dep-tenant-123-v1
openclawctl deployment redeploy --tenant tenant_123 --strategy replace --idempotency-key dep-tenant-123-v2
openclawctl deployment stop --tenant tenant_123 --idempotency-key stop-tenant-123-v1
openclawctl deployment start --tenant tenant_123 --idempotency-key start-tenant-123-v1
openclawctl deployment restart --tenant tenant_123 --idempotency-key restart-tenant-123-v1
openclawctl deployment destroy --tenant tenant_123 --destroy-workspace=false --destroy-volume=false --idempotency-key destroy-tenant-123-v1 --yes
```

Optional blocking behavior:

```bash
openclawctl deployment deploy --tenant tenant_123 --wait --wait-timeout 180s
```

Behavior:

- default mode returns the job record immediately
- `--wait` polls the job endpoint until success, failure, or timeout

### 6.7 Job Commands

```bash
openclawctl job get --job ee18a31b-28a4-4937-9f42-6b78a0fda48f
openclawctl job list --tenant tenant_123 --status pending
openclawctl job watch --job ee18a31b-28a4-4937-9f42-6b78a0fda48f
```

`job watch` is important because lifecycle commands are asynchronous.

### 6.8 Instance Commands

```bash
openclawctl instance get --tenant tenant_123
openclawctl instance history --tenant tenant_123
openclawctl instance get-by-id --instance 446c4be5-1cf8-434f-a10c-9730cf5dd7dc
openclawctl runtime-config get --tenant tenant_123
```

### 6.9 Audit Commands

```bash
openclawctl audit list --tenant tenant_123
```

### 6.10 Declarative Apply Commands

```bash
openclawctl apply -f tenant.yaml
openclawctl apply -f tenant.yaml --validate-only
openclawctl apply -f tenant.yaml --deploy
```

Expected behavior of `apply`:

1. create tenant if absent
2. update tenant metadata if present
3. update tenant profile
4. sync secret references
5. validate
6. optionally trigger deploy

`apply` should be idempotent from the operator perspective.

## 7. Recommended Tenant Manifest Format

Example `tenant.yaml`:

```yaml
tenant:
  external_user_id: user_10001
  slug: acme-user-10001
  display_name: Acme User 10001

profile:
  template_id: tpl_standard
  resource_tier: standard
  route_key: tenant-10001
  model_provider: openai-compatible
  model_name: gpt-4.1
  channels_file: ./channels.json
  skills_file: ./skills.json
  soul_file: ./SOUL.md
  memory_file: ./memory.md
  extra_files:
    - path: prompts/team.md
      source: ./team.md

secrets:
  OPENAI_API_KEY:
    from_env: OPENAI_API_KEY
  DISCORD_BOT_TOKEN:
    from_file: ./discord.token
```

Manifest design rules:

- the manifest may reference secret sources, but must not store raw secret values in Git
- file paths are local to the CLI execution environment
- `apply` resolves the files and sends normalized API requests

## 8. API to CLI Coverage Matrix

| API Endpoint | CLI Command |
| --- | --- |
| `GET /healthz` | `openclawctl health` |
| `GET /readyz` | `openclawctl ready` |
| `POST /tenants` | `openclawctl tenant create` |
| `GET /tenants/{tenant_id}` | `openclawctl tenant get` |
| `GET /tenants` | `openclawctl tenant list` |
| `PUT /tenants/{tenant_id}/profile` | `openclawctl profile set` |
| `GET /tenants/{tenant_id}/profile` | `openclawctl profile get` |
| `POST /tenants/{tenant_id}/profile/validate` | `openclawctl profile validate` |
| `PUT /tenants/{tenant_id}/secrets/{secret_key}` | `openclawctl secret set` |
| `GET /tenants/{tenant_id}/secrets` | `openclawctl secret list` |
| `DELETE /tenants/{tenant_id}/secrets/{secret_key}` | `openclawctl secret delete` |
| `GET /templates` | `openclawctl template list` |
| `GET /templates/{template_id}` | `openclawctl template get` |
| `GET /images` | `openclawctl image list` |
| `POST /tenants/{tenant_id}/deploy` | `openclawctl deployment deploy` |
| `POST /tenants/{tenant_id}/redeploy` | `openclawctl deployment redeploy` |
| `POST /tenants/{tenant_id}/stop` | `openclawctl deployment stop` |
| `POST /tenants/{tenant_id}/start` | `openclawctl deployment start` |
| `POST /tenants/{tenant_id}/restart` | `openclawctl deployment restart` |
| `DELETE /tenants/{tenant_id}/deployment` | `openclawctl deployment destroy` |
| `GET /jobs/{job_id}` | `openclawctl job get` |
| `GET /jobs` | `openclawctl job list` |
| `GET /tenants/{tenant_id}/instance` | `openclawctl instance get` |
| `GET /tenants/{tenant_id}/instances` | `openclawctl instance history` |
| `GET /instances/{instance_id}` | `openclawctl instance get-by-id` |
| `GET /tenants/{tenant_id}/audit-logs` | `openclawctl audit list` |
| `GET /tenants/{tenant_id}/runtime-config` | `openclawctl runtime-config get` |

## 9. Exit Codes

Suggested exit codes:

- `0`: success
- `2`: validation failure
- `3`: authentication or authorization failure
- `4`: resource not found
- `5`: conflict or invalid state transition
- `6`: remote dependency unavailable
- `10`: unexpected internal or transport error

This gives scripts a stable contract.

## 10. Output Rules

- default output for list commands: table
- default output for single-resource commands: YAML or JSON summary
- `--output json` must preserve machine-readable fields
- secret values must always be masked or omitted

## 11. Phase 1 Minimum CLI Scope

The minimum acceptable first release of `openclawctl` should include:

1. `health`
2. `tenant create|get|list`
3. `profile set|get|validate`
4. `secret set|list|delete`
5. `deployment deploy|stop|start|destroy`
6. `job get|watch`
7. `instance get`
8. `apply -f tenant.yaml --validate-only|--deploy`

## 12. Recommendation

Treat CLI parity as a hard product requirement, not a later convenience layer. The cleanest implementation is to build `openclawctl` as a thin client over the approved REST API, with a shared typed API client package used by both tests and command handlers.
