# OpenClaw AutoDeploy 架构设计

## 1. 概述

本项目提供后端控制面，根据 PostgreSQL 记录自动为用户创建专用 OpenClaw 容器。系统通过 API 暴露所有能力，运行在 Ubuntu 24.04 上的 Linux 后台服务中。

目标工作流程：

1. 运营系统或上游表单将用户基础数据和预设选择写入 PostgreSQL
2. 控制面验证租户配置并创建部署任务
3. `ultraworker` 解析租户的镜像、运行时模板、密钥和种子文件
4. Worker 创建租户工作空间、渲染配置、启动容器并等待健康确认
5. 控制面记录生命周期状态并暴露查询、重启、停止、重新部署和销毁 API

## 2. 目标

- 为每个租户/用户自动创建一个 OpenClaw 运行时
- 通过 `user_id` 从 PostgreSQL 派生运行时配置
- 支持租户特定的种子数据，如模型 API 密钥、渠道 ID、技能列表、`SOUL.md`、`memory.md` 等启动资产
- 标准化镜像选择、配置渲染、容器启动、健康检查和生命周期恢复
- 无 GUI 运行；为外部系统和运营人员提供简洁的 API 和 CLI 接口
- 第一版保持足够简单以适应单台 Ubuntu 主机，同时为后续水平扩展留出空间

## 3. 第一阶段非目标

- 无 Web UI
- 无 Kubernetes
- 除非运行时模板无法满足需求，否则无每租户自定义镜像构建流水线
- 无自助计费或客户门户
- 第一版无多主机集群调度

## 4. 推荐技术基础

因为是全新项目，推荐基础为：

- 语言：Go 1.24+
- HTTP 框架：`chi` 或 `gin`（控制面 API 偏好 `chi`）
- 数据库：PostgreSQL 16+
- 数据库访问：`sqlc` + `pgx`
- 迁移：`golang-migrate`
- Docker 控制：通过 Go Docker SDK 的 Docker Engine API
- 反向代理：Traefik v3 + Docker provider
- 服务运行时：`systemd`
- 配置格式：YAML + 环境变量
- 静态密钥：PostgreSQL 加密列（`pgcrypto`）+ 应用主密钥（来自环境变量或文件）

这套技术栈适合 Ubuntu 后台服务，允许控制面使用单一静态二进制，部署和可观测性保持简洁。

## 5. 高层架构

核心组件：

1. **`control-plane-api`**
   - 暴露 REST API
   - 验证租户数据
   - 创建部署和生命周期任务
   - 提供实例、任务和审计查询端点

2. **`openclawctl`**
   - 面向运营人员的 CLI 二进制
   - 封装完整 API 表面积用于脚本、CI 和手动操作
   - 支持声明式 apply 模式和命令式生命周期命令
   - 为人和自动化输出表格或 JSON

3. **`ultraworker`**
   - 从 PostgreSQL 拉取待处理任务
   - 用 `FOR UPDATE SKIP LOCKED` 锁定任务
   - 解析镜像、模板集、密钥和运行时参数
   - 创建本地工作空间和 Docker 资源
   - 启动/停止/重启/重新部署/销毁租户容器
   - 执行健康验证和协调

4. **PostgreSQL**
   - 租户、模板、部署任务、实例、密钥元数据和审计日志的真实数据源

5. **Docker Engine**
   - 运行 OpenClaw 租户容器
   - 托管共享网络、命名卷和每租户挂载

6. **Traefik**
   - 使用标签将入站流量路由到租户容器
   - 避免每容器主机端口管理

7. **租户工作空间存储**
   - 本地文件系统路径如 `/srv/openclaw/tenants/<tenant_id>/`
   - 存储渲染后的配置、`SOUL.md`、`memory.md`、挂载的工作文件和生成的 env 文件

## 5.1 CLI 设计原则

系统不得依赖 GUI 进行任何操作路径。每个 API 暴露的运营能力也必须可通过第一方 CLI 访问。

CLI 要求：

- 与 API 端点完整生命周期对等
- 仅从环境变量、文件或 stdin 安全输入密钥
- JSON 输出模式用于自动化
- CI/CD 和 shell 脚本的非交互标志
- 可选的声明式 `apply` 工作流用于租户接入

## 6. 部署模型

每个租户获得：

- PostgreSQL 中的一条逻辑租户记录
- 一个运行时配置快照
- 一个专用 OpenClaw 容器
- 一个磁盘上的工作空间目录
- 一个命名 Docker 卷（如果需要持久运行时数据）
- Traefik 中的一组路由标签

推荐主机布局：

- `control-plane-api` 作为 systemd 服务
- `ultraworker` 作为 systemd 服务
- PostgreSQL 本地或托管服务
- Docker Engine 同主机（第一阶段）
- Traefik 同主机附加到 `traefik-public` 网络

## 7. 供应策略

### 7.1 基本原则

优先使用共享 OpenClaw 基础镜像 + 运行时注入，而非每租户镜像构建。

原因：

- 更快的租户配置
- 更少的镜像管理
- 更简单的回滚
- 更简单的安全修补
- 通过挂载文件和环境变量注入支持租户特定内容

### 7.2 供应输入

对于每个租户，worker 解析：

- `tenant_id`
- 选定的部署模板
- 选定的基础镜像标签
- LLM 提供商配置和 API 密钥
- 渠道和渠道凭证
- 技能列表
- `SOUL.md`
- `memory.md`
- 可选的额外启动文件
- CPU/内存层级
- 域名/子域名或路由键

### 7.3 渲染输出

worker 将租户运行时包写入工作空间，例如：

```
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

### 7.4 容器创建流程

1. 验证租户记录完整性
2. 创建状态为 `pending` 的部署任务
3. Worker 锁定任务并标记为 `running`
4. 解析基础镜像和部署模板
5. 实例化租户工作空间和配置快照
6. 创建或复用 Docker 网络和卷
7. 创建容器，包含：
   - 租户标签
   - 挂载的工作空间
   - 资源限制
   - 重启策略
   - 健康检查
8. 启动容器
9. 等待健康检查成功
10. 将实例状态更新为 `running` 并存储快照/版本

### 7.5 运行时标签

推荐标签：

- `app=openclaw`
- `service=tenant-runtime`
- `tenant.id=<tenant_id>`
- `tenant.slug=<tenant_slug>`
- `template.id=<template_id>`
- `image.tag=<image_tag>`
- `managed.by=openclaw-autodeploy`

Traefik 标签应遵循相同的租户标识。

## 8. 数据模型

### 8.1 主要表

#### `tenants` - 租户身份和业务状态

关键字段：`id` UUID PK, `external_user_id` text unique, `slug` text unique, `display_name` text, `status` text, `created_at`, `updated_at`

#### `tenant_profiles` - 已解析的业务输入用于部署

关键字段：`tenant_id` UUID PK/FK, `model_provider` text, `model_name` text, `channels` jsonb, `skills` jsonb, `soul_markdown` text, `memory_markdown` text, `extra_files` jsonb, `resource_tier` text, `route_key` text, `template_id` UUID, `is_valid` boolean, `validation_errors` jsonb

#### `image_catalog` - 可用的基础镜像

关键字段：`id` UUID PK, `image_ref` text, `version` text, `runtime_family` text, `status` text, `default_template_id` UUID

#### `deployment_templates` - 租户类型的预设组合

关键字段：`id` UUID PK, `code` text unique, `name` text, `description` text, `base_image_policy` jsonb, `default_channels` jsonb, `default_skills` jsonb, `default_files` jsonb, `resource_policy` jsonb, `enabled` boolean

#### `tenant_secrets` - 加密的敏感值

关键字段：`id` UUID PK, `tenant_id` UUID FK, `secret_type` text, `secret_key` text, `encrypted_value` bytea, `value_fingerprint` text, `version` int, `status` text

#### `deployment_jobs` - 异步任务队列和执行状态

关键字段：`id` UUID PK, `tenant_id` UUID FK, `job_type` text, `requested_by` text, `idempotency_key` text, `status` text, `payload` jsonb, `attempt_count` int, `last_error` text, `scheduled_at` timestamptz, `started_at` timestamptz, `finished_at` timestamptz, `worker_name` text, `heartbeat_at` timestamptz

#### `tenant_instances` - 当前和历史运行时实例

关键字段：`id` UUID PK, `tenant_id` UUID FK, `deployment_job_id` UUID FK, `container_id` text, `container_name` text, `image_ref` text, `status` text, `host_node` text, `workspace_path` text, `volume_name` text, `route_url` text, `health_status` text, `config_version` int, `started_at` timestamptz, `stopped_at` timestamptz

#### `audit_logs` - 不可变的操作跟踪

关键字段：`id` UUID PK, `tenant_id` UUID null, `actor` text, `action` text, `target_type` text, `target_id` text, `request_id` text, `details` jsonb, `created_at` timestamptz

### 8.2 建议的状态枚举

租户状态：`draft`, `ready`, `deploying`, `running`, `stopped`, `failed`, `archived`

任务状态：`pending`, `running`, `succeeded`, `failed`, `cancelled`

实例状态：`creating`, `starting`, `running`, `degraded`, `stopping`, `stopped`, `destroyed`, `failed`

## 9. API/Worker 交互模式

- API 永远不直接阻塞长时间 Docker 操作
- API 将任务写入 `deployment_jobs`
- `ultraworker` 异步执行任务
- 短操作可选择支持 `wait=true`（小超时）用于内部运营，但规范模式保持异步

这保持 API 延迟稳定，使重试、协调和审计更容易。

## 10. 协调和恢复

`ultraworker` 必须每 15-30 秒运行协调循环：

- 查找卡在 `running` 的任务
- 比较 DB 实例状态与 Docker 实际状态
- 检测缺失容器
- 检测不健康容器
- 更新心跳和健康状态
- 根据策略可选自动重启

恢复规则：

- 如果容器创建在启动前失败，标记任务失败并保留工作空间供检查
- 如果容器存在但 DB 缺失，协调到 `degraded`
- 如果 DB 显示运行中但 Docker 显示已退出，记录退出码并应用重启策略

## 11. 安全设计

### 11.1 密钥

- 永远不存储明文租户 API 密钥
- 使用 `pgcrypto` 或应用级信封加密静态加密
- 仅在 worker 执行路径内解密
- 优先使用挂载的 env 文件或 root 拥有的路径，而非命令行参数
- 在日志和 API 响应中掩盖密钥

### 11.2 API 访问

第一阶段建议：

- 仅内部 API
- 调用者和控制面之间的令牌或 mTLS
- RBAC 角色：`admin`、`operator`、`viewer`

### 11.3 容器加固

默认容器约束：

- `CapDrop=ALL`
- `no-new-privileges=true`
- 根文件系统只读（当 OpenClaw 允许时）
- `/tmp` 专用 tmpfs
- 每租户层级 CPU 和内存限制
- PID 限制
- 有限的日志轮转

### 11.4 文件安全

- 清理租户提供的文件名
- 仅允许在工作空间根目录下写入
- 版本化渲染配置快照
- 记录生成文件的校验和以便支持

## 12. 资源层级

建议起始层级：

| 层级 | CPU | 内存 | PID | 说明 |
| --- | --- | --- | --- | --- |
| `starter` | 0.5 核 | 512 MB | 128 | 测试/演示 |
| `standard` | 1 核 | 2 GB | 256 | 默认生产 |
| `pro` | 2 核 | 4 GB | 512 | 更重技能/渠道 |
| `enterprise` | 4 核 | 8 GB | 1024 | 保留 |

这些是控制面中的策略值，不是调用者中的硬编码。

## 13. LLM API 密钥共享池

系统支持 LLM API 密钥的共享池管理：

1. 添加提供商：`openclawctl provider add --name minimax`
2. 添加 API 密钥：`openclawctl apikey add --provider <ID> --key-file /path/to/key`
3. 分配给租户：`openclawctl tenant allocate-llm-key --tenant <ID> --provider <ID> --key-id <KEY_ID>`

分配的密钥作为 `{PROVIDER_NAME}_API_KEY` 环境变量注入租户容器。

## 14. Bearer Token 认证

所有 `/api/v1/*` 下的 API 端点需要 Bearer token：

```
Authorization: Bearer <token>
```

通过 `security.static_token` 配置或 `OPENCLAW_SECURITY_STATIC_TOKEN` 环境变量设置令牌。

公开端点（无需认证）：
- `GET /healthz` - 存活检查
- `GET /metrics` - Prometheus 指标

## 15. 部署失败回滚

部署新租户容器时，旧容器先**停止**（不销毁）。如果新容器启动或健康检查失败，旧容器**自动重启**，保持租户可用性。如果新容器成功，旧容器移除。

## 16. 多租户路由

每个租户容器通过 Traefik 在 `{route_key}.{base_domain}` 暴露（如 `user-001.example.com`）。

流量路径：`Host header → Traefik (port 80) → host.docker.internal:18789 → OpenClaw gateway`

容器使用 `--network host` 模式，使 Traefik 能通过 `host.docker.internal` 访问容器端口。
