# OpenClaw AutoDeploy API 参考文档

## 1. API 定位

本 API 面向内部系统或运营人员。第一阶段无 GUI。所有长时操作均为异步任务驱动。

基础路径：

```
/api/v1
```

第一阶段推荐认证：

- `Authorization: Bearer <token>`
- 内部 RBAC 角色：`admin`、`operator`、`viewer`

通用请求头：

- `X-Request-Id`：调用方生成的跟踪 ID，可选但推荐
- `Idempotency-Key`：create/redeploy/destroy/start/stop 请求必填

响应格式：

- 仅 JSON
- 时间戳为 ISO 8601 UTC
- 错误为稳定的机器可读格式

CLI 对等要求：

- 本文档中每个面向运营的端点必须有对应的第一方 `openclawctl` 命令映射
- CLI 设计文档见 `docs/zh/cli-reference.md`

## 2. 通用对象

### 2.1 租户

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

### 2.2 租户配置

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

### 2.3 任务

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
  "finished_at": null,
  "worker_name": "ultraworker",
  "heartbeat_at": null
}
```

### 2.4 实例

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

## 3. 通用错误格式

```json
{
  "error": {
    "code": "TENANT_PROFILE_INVALID",
    "message": "租户配置不完整，无法部署。",
    "details": {
      "missing_fields": ["model_provider", "tenant_secrets.OPENAI_API_KEY"]
    },
    "request_id": "req_20260322_001"
  }
}
```

推荐错误码：

- `UNAUTHORIZED` - 未授权
- `FORBIDDEN` - 禁止访问
- `VALIDATION_ERROR` - 验证错误
- `TENANT_NOT_FOUND` - 租户未找到
- `TEMPLATE_NOT_FOUND` - 模板未找到
- `IMAGE_NOT_AVAILABLE` - 镜像不可用
- `TENANT_PROFILE_INVALID` - 租户配置无效
- `IDEMPOTENCY_CONFLICT` - 幂等性冲突
- `JOB_ALREADY_RUNNING` - 任务已在运行
- `INSTANCE_NOT_FOUND` - 实例未找到
- `INSTANCE_NOT_RUNNING` - 实例未运行
- `DOCKER_BACKEND_UNAVAILABLE` - Docker 后端不可用
- `CAPACITY_EXCEEDED` - 容量超限
- `INTERNAL_ERROR` - 内部错误

## 4. API 端点

### 4.1 健康和基础设施

#### `GET /healthz`

返回 API 存活状态。

响应 `200`：

```json
{ "status": "ok" }
```

#### `GET /readyz`

返回依赖就绪状态。

响应 `200`：

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

#### `GET /metrics`

返回 Prometheus 指标（无需认证）。

响应 `200`：Prometheus text 格式

### 4.2 租户注册和查询

#### `POST /tenants`

创建租户记录。

请求：

```json
{
  "external_user_id": "user_10001",
  "slug": "acme-user-10001",
  "display_name": "Acme User 10001"
}
```

响应 `201`：

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

获取单个租户。

#### `GET /tenants`

列出租户。

查询参数：`status`, `external_user_id`, `slug`, `page`, `page_size`

### 4.3 租户配置管理

#### `PUT /tenants/{tenant_id}/profile`

创建或替换已解析的部署配置。

请求：

```json
{
  "template_id": "7c612cbc-35af-4de0-96af-0f470c7540ca",
  "resource_tier": "standard",
  "route_key": "tenant-10001",
  "model_provider": "openai-compatible",
  "model_name": "gpt-4.1",
  "channels": [...],
  "skills": ["planner", "code-review"],
  "soul_markdown": "# SOUL\ncustom persona",
  "memory_markdown": "# MEMORY\nbootstrap memory",
  "extra_files": [...]
}
```

#### `GET /tenants/{tenant_id}/profile`

读取当前配置。

#### `POST /tenants/{tenant_id}/profile/validate`

验证配置和密钥完整性，不进行部署。

### 4.4 租户密钥管理

#### `PUT /tenants/{tenant_id}/secrets/{secret_key}`

创建或轮换一个密钥。

请求：

```json
{
  "value": "sk-live-xxxxx",
  "secret_type": "api_key"
}
```

#### `GET /tenants/{tenant_id}/secrets`

列出密钥元数据（永远不返回实际值）。

#### `DELETE /tenants/{tenant_id}/secrets/{secret_key}`

如果策略允许，撤销密钥。

### 4.5 模板和镜像目录

#### `GET /templates`

列出部署模板。

#### `GET /templates/{template_id}`

获取一个部署模板。

#### `GET /images`

列出可用基础镜像。

查询参数：`runtime_family`, `status`

### 4.6 部署生命周期

#### `POST /tenants/{tenant_id}/deploy`

创建部署任务。

请求：

```json
{
  "reason": "initial deployment",
  "wait": false
}
```

响应 `202`：

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

规则：需要有效配置、需要必填密钥、如果租户已有其他生命周期任务在运行则拒绝

#### `POST /tenants/{tenant_id}/redeploy`

使用最新配置创建重新部署任务。

#### `POST /tenants/{tenant_id}/stop`

停止运行中的租户实例。

#### `POST /tenants/{tenant_id}/start`

启动当前停止的租户实例（如果可复用），否则后端可能创建新实例。

#### `POST /tenants/{tenant_id}/restart`

重启运行中的实例。

#### `DELETE /tenants/{tenant_id}/deployment`

销毁活动部署。

请求：

```json
{
  "destroy_workspace": false,
  "destroy_volume": false,
  "reason": "tenant archived"
}
```

### 4.7 任务查询

#### `GET /jobs/{job_id}`

获取一个任务。

#### `GET /jobs`

列出任务。

查询参数：`tenant_id`, `job_type`, `status`, `page`, `page_size`

### 4.8 实例查询

#### `GET /tenants/{tenant_id}/instance`

获取当前活动实例。

#### `GET /tenants/{tenant_id}/instances`

获取实例历史。

#### `GET /instances/{instance_id}`

按 ID 获取一个实例。

### 4.9 LLM API 密钥管理

#### `GET /providers`

列出 LLM 提供商。

#### `POST /providers`

创建/更新提供商。

#### `GET /api-keys`

列出 API 密钥（元数据，不返回实际值）。

#### `POST /api-keys`

添加 API 密钥。

#### `GET /tenants/{tenant_id}/llm-allocation`

获取租户的 LLM 密钥分配。

#### `POST /tenants/{tenant_id}/llm-allocation`

为租户分配 LLM API 密钥。

#### `DELETE /tenants/{tenant_id}/llm-allocation`

撤销租户的 LLM 密钥分配。

## 5. 幂等性规则

以下端点需要 `Idempotency-Key`：

- `POST /tenants/{tenant_id}/deploy`
- `POST /tenants/{tenant_id}/redeploy`
- `POST /tenants/{tenant_id}/stop`
- `POST /tenants/{tenant_id}/start`
- `POST /tenants/{tenant_id}/restart`
- `DELETE /tenants/{tenant_id}/deployment`

行为：

- 相同 key + 相同语义请求返回原始任务
- 相同 key + 不同请求体返回 `409 IDEMPOTENCY_CONFLICT`

## 6. 状态转换规则

规范转换：

- `draft -> ready`
- `ready -> deploying`
- `deploying -> running`
- `deploying -> failed`
- `running -> stopped`
- `stopped -> running`
- `running -> degraded`
- `degraded -> running`
- `running|stopped|failed -> archived`

API 应以 `409 VALIDATION_ERROR` 拒绝无效转换。

## 7. 运营建议

- 所有列表端点使用分页
- 默认页面大小 20，最大 100
- 将每个变更请求记录到 `audit_logs`
- 在所有 API 载荷和日志中掩盖密钥
- 异步操作返回 `202`，非 `200`
- 保持 API 名称稳定并在 `/api/v1` 下版本化

## 8. Prometheus 指标

`GET /metrics`（公开）返回：

```
# HELP openclaw_up 控制面上线状态
# TYPE openclaw_up gauge
openclaw_up 1
# HELP openclaw_tenants_total 租户总数
# TYPE openclaw_tenants_total gauge
openclaw_tenants_total 3
# HELP openclaw_containers_running 运行中的容器数
# TYPE openclaw_containers_running gauge
openclaw_containers_running 1
# HELP openclaw_jobs_pending 待处理任务数
# TYPE openclaw_jobs_pending gauge
openclaw_jobs_pending 0
# HELP openclaw_jobs_succeeded_total 成功任务总数
# TYPE openclaw_jobs_succeeded_total counter
openclaw_jobs_succeeded_total 17
# HELP openclaw_jobs_failed_total 失败任务总数
# TYPE openclaw_jobs_failed_total counter
openclaw_jobs_failed_total 2
```
