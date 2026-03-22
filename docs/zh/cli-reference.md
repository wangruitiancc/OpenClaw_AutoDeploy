# OpenClaw AutoDeploy CLI 参考手册

## 1. 定位

`openclawctl` 是 OpenClaw AutoDeploy 控制面的官方命令行客户端。在没有 GUI 的环境中，它是唯一的运维接口，必须覆盖全部已批准的后端能力。

设计目标：

- 通过 CLI 提供完整的 API 覆盖
- 对 CI/CD 和 shell 自动化友好
- 对运维人员输出友好
- 安全的密钥输入方式
- 同时支持命令式和声明式工作流

## 2. 二进制文件和调用方式

推荐的二进制文件名：

```text
openclawctl
```

基本形式：

```bash
openclawctl [全局选项] <资源> <命令> [flags]
```

示例：

```bash
openclawctl tenant list
openclawctl profile validate --tenant tenant_123
openclawctl deployment deploy --tenant tenant_123 --wait
```

## 3. 运行模式

### 3.1 命令式模式

运维人员直接执行单个动作的命令。

示例：

```bash
openclawctl secret set --tenant tenant_123 OPENAI_API_KEY --from-env OPENAI_API_KEY
openclawctl deployment restart --tenant tenant_123
```

### 3.2 声明式模式

运维人员应用一个租户清单文件，该文件描述租户身份、profile、文件清单和密钥引用。

示例：

```bash
openclawctl apply -f tenant.yaml
openclawctl apply -f tenant.yaml --validate-only
```

声明式模式是重复性接入和 CI 自动化的首选方式。

## 4. 全局选项

所有命令应支持：

- `--server`：控制面基础 URL
- `--token`：Bearer 令牌；不建议留在 shell 历史记录中
- `--token-file`：从文件读取令牌
- `--profile`：命名的 CLI profile
- `--output table|json|yaml`：输出格式
- `--request-id`：调用方跟踪 ID
- `--timeout`：请求超时时间
- `--verbose`：显示请求元数据
- `--no-color`：禁用 ANSI 颜色

推荐的环境变量：

- `OPENCLAWCTL_SERVER`
- `OPENCLAWCTL_TOKEN`
- `OPENCLAWCTL_PROFILE`
- `OPENCLAWCTL_OUTPUT`

## 5. 认证和本地配置

推荐的本地配置文件：

```text
~/.config/openclawctl/config.yaml
```

推荐命令：

```bash
openclawctl config init
openclawctl config set server https://control-plane.internal
openclawctl config set token-file ~/.config/openclawctl/token
openclawctl config view
```

如果后续需要登录流程，增加：

```bash
openclawctl auth login
openclawctl auth logout
openclawctl auth whoami
```

第一阶段可使用预发的内部 Bearer 令牌，无需交互式认证。

## 6. 命令分类

### 6.1 健康检查和诊断

```bash
openclawctl health
openclawctl ready
openclawctl doctor
openclawctl version
```

说明：

- `health` 对应 API 存活探活
- `ready` 对应依赖就绪状态
- `doctor` 可组合 API 就绪状态和本地配置验证

### 6.2 租户命令

```bash
openclawctl tenant create --external-user-id user_10001 --slug acme-user-10001 --display-name "Acme User 10001"
openclawctl tenant get --tenant 6d9ad1f8-9843-4d6c-bb24-c81cbf765412
openclawctl tenant list --status ready
```

命令设计规则：

- `--tenant` 接受租户 UUID
- `--slug` 过滤在列表操作时可用
- 危险操作需要确认，除非提供了 `--yes`

### 6.3 Profile 命令

```bash
openclawctl profile get --tenant tenant_123
openclawctl profile set --tenant tenant_123 --template tpl_standard --tier standard --route-key tenant-10001 --model-provider openai-compatible --model-name gpt-4.1 --channels-file channels.json --skills-file skills.json --soul-file SOUL.md --memory-file memory.md
openclawctl profile validate --tenant tenant_123
```

推荐的文件导向 flags：

- `--channels-file`
- `--skills-file`
- `--soul-file`
- `--memory-file`
- `--extra-file path=localfile`

### 6.4 密钥命令

```bash
openclawctl secret list --tenant tenant_123
openclawctl secret set --tenant tenant_123 OPENAI_API_KEY --from-env OPENAI_API_KEY
openclawctl secret set --tenant tenant_123 DISCORD_BOT_TOKEN --from-file ./discord.token
printf '%s' "$TOKEN" | openclawctl secret set --tenant tenant_123 DISCORD_BOT_TOKEN --stdin
openclawctl secret delete --tenant tenant_123 OPENAI_API_KEY --yes
```

密钥安全规则：

- 禁止使用 `--value` flag 传入密钥
- 仅接受 `--from-env`、`--from-file` 或 `--stdin` 作为密钥来源
- 命令输出永远不打印密钥值

### 6.5 模板和镜像目录命令

```bash
openclawctl template list
openclawctl template get --template tpl_standard
openclawctl image list --status active
openclawctl image list --image-ref registry.local/openclaw-base:1.0.0
```

### 6.6 部署生命周期命令

```bash
openclawctl deployment deploy --tenant tenant_123 --idempotency-key dep-tenant-123-v1
openclawctl deployment redeploy --tenant tenant_123 --strategy replace --idempotency-key dep-tenant-123-v2
openclawctl deployment stop --tenant tenant_123 --idempotency-key stop-tenant-123-v1
openclawctl deployment start --tenant tenant_123 --idempotency-key start-tenant-123-v1
openclawctl deployment restart --tenant tenant_123 --idempotency-key restart-tenant-123-v1
openclawctl deployment destroy --tenant tenant_123 --destroy-workspace=false --destroy-volume=false --idempotency-key destroy-tenant-123-v1 --yes
```

可选的阻塞行为：

```bash
openclawctl deployment deploy --tenant tenant_123 --wait --wait-timeout 180s
```

行为说明：

- 默认模式立即返回任务记录
- `--wait` 轮询任务端点直到成功、失败或超时

### 6.7 任务命令

```bash
openclawctl job get --job ee18a31b-28a4-4937-9f42-6b78a0fda48f
openclawctl job list --tenant tenant_123 --status pending
openclawctl job watch --job ee18a31b-28a4-4937-9f42-6b78a0fda48f
```

`job watch` 很重要，因为生命周期命令是异步的。

### 6.8 实例命令

```bash
openclawctl instance get --tenant tenant_123
openclawctl instance history --tenant tenant_123
openclawctl instance get-by-id --instance 446c4be5-1cf8-434f-a10c-9730cf5dd7dc
openclawctl runtime-config get --tenant tenant_123
```

### 6.9 审计命令

```bash
openclawctl audit list --tenant tenant_123
```

### 6.10 声明式 Apply 命令

```bash
openclawctl apply -f tenant.yaml
openclawctl apply -f tenant.yaml --validate-only
openclawctl apply -f tenant.yaml --deploy
```

`apply` 的预期行为：

1. 如果不存在则创建租户
2. 如果存在则更新租户元数据
3. 更新租户 profile
4. 同步密钥引用
5. 验证
6. 可选触发部署

`apply` 从操作员视角看应该是幂等的。

## 7. 推荐的租户清单格式

示例 `tenant.yaml`：

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

清单设计规则：

- 清单可以引用密钥来源，但禁止将原始密钥值存储在 Git 中
- 文件路径相对于 CLI 执行环境的本地路径
- `apply` 解析文件并发送标准化的 API 请求

## 8. API 到 CLI 覆盖矩阵

| API 端点 | CLI 命令 |
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

## 9. 退出码

建议的退出码：

- `0`：成功
- `2`：验证失败
- `3`：认证或授权失败
- `4`：资源未找到
- `5`：冲突或状态转换无效
- `6`：远程依赖不可用
- `10`：意外内部错误或传输错误

这为脚本提供了稳定的约定。

## 10. 输出规则

- 列表命令默认输出：表格
- 单资源命令默认输出：YAML 或 JSON 摘要
- `--output json` 必须保留机器可读字段
- 密钥值必须始终被掩码或省略

## 11. 第一阶段最小 CLI 范围

`openclawctl` 首个可发布版本应包含：

1. `health`
2. `tenant create|get|list`
3. `profile set|get|validate`
4. `secret set|list|delete`
5. `deployment deploy|stop|start|destroy`
6. `job get|watch`
7. `instance get`
8. `apply -f tenant.yaml --validate-only|--deploy`

## 12. 建议

将 CLI 能力视为硬性产品需求，而非后续的便利层。最简洁的实现方式是将 `openclawctl` 构建为已批准 REST API 的薄客户端，共享的带类型 API 客户端包同时被测试和命令处理器使用。
