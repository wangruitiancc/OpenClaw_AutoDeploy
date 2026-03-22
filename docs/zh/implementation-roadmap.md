# OpenClaw AutoDeploy 实施路线图

## 1. 交付策略

项目应分四个里程碑实施。第一个目标是在已安装 Docker 的现有 Ubuntu 24.04 测试虚拟机上证明端到端路径，同时保持生产架构决策与后续扩展兼容。

## 2. 里程碑计划

### 里程碑 0 - 项目初始化

目标：

- 创建仓库骨架
- 初始化 Go 模块
- 添加配置加载
- 添加 PostgreSQL 迁移框架
- 添加本地开发脚本
- 添加 CLI 骨架和共享 API 客户端包
- 添加 systemd 和 Traefik 部署模板

退出标准：

- API 二进制文件启动成功
- worker 二进制文件启动成功
- 数据库迁移成功运行
- 健康端点返回健康状态（DB 和 Docker 检查可以是 stub 或真实实现）

### 里程碑 1 - 租户注册与验证

目标：

- 实现租户 CRUD
- 实现租户 Profile CRUD
- 实现加密密钥存储
- 实现 Profile 验证规则
- 实现模板目录和镜像目录读取 API
- 实现 tenant/profile/secret/template/image CLI 命令

退出标准：

- 调用方可以创建租户、存储密钥、写入 profile，并收到通过/失败验证结果
- 所有变更操作都有审计日志

### 里程碑 2 - 异步部署和生命周期控制

目标：

- 实现任务表和 worker 循环
- 实现 deploy/start/stop/restart/destroy 生命周期任务
- 实现工作空间渲染
- 实现 Docker 容器创建
- 实现健康检查等待和实例持久化
- 实现 deployment/job/instance/audit CLI 命令

退出标准：

- 一个租户可以从数据库数据部署到运行中的 OpenClaw 容器
- worker 正确更新任务和实例状态
- stop/start/redeploy 路径在测试虚拟机上工作正常

### 里程碑 3 - 调和与运维加固

目标：

- 实现调和循环
- 实现容量 guardrails
- 添加指标和结构化日志
- 添加重试和死信风格失败处理
- 添加租户工作空间元数据的备份/导出

退出标准：

- 系统能检测 DB 和 Docker 状态之间的漂移
- 可以暴露并修复降级容器
- 运维遥测数据足以支持故障排查

## 3. 按模块推荐的构建顺序

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

这个顺序使领域和持久化层在容器编排代码开始之前保持稳定。

## 4. 数据库交付计划

初始迁移应创建：

- `tenants`
- `tenant_profiles`
- `tenant_secrets`
- `deployment_templates`
- `image_catalog`
- `deployment_jobs`
- `tenant_instances`
- `audit_logs`

推荐的迁移策略：

- 只进迁移
- 合并后绝不修改旧迁移文件
- 早期阶段将枚举类状态视为受约束的文本字段，除非明确需要严格枚举

## 5. 早期应实现的验证规则

当以下任一条件缺失时，验证器应拒绝部署：

- 租户存在
- 所选模板存在且启用
- 所选镜像存在且活跃
- 必需的模型提供商和模型名称存在
- 必需的密钥存在
- 如果启用路由，路由键唯一
- 文件负载大小在配置限制内
- 资源层级有效

建议的 guardrails：

- `SOUL.md` 最大 256 KB
- `memory.md` 最大 256 KB
- 单个额外文件最大 256 KB
- 组合渲染负载最大 2 MB

## 6. 测试策略

### 单元测试

- Profile 验证
- 密钥掩码
- 模板渲染
- 状态转换
- 幂等性处理

### 集成测试

- PostgreSQL 仓库测试
- 针对本地 Docker 的 Docker 适配器测试
- 在一次性租户上完整部署流程测试

### 端到端测试

- 创建租户 -> 验证 -> 部署 -> 运行 -> 停止 -> 启动 -> 销毁
- 密钥轮换后重新部署
- 容器崩溃后恢复

## 7. 环境计划

### 本地开发

- `docker compose` 用于 PostgreSQL + Traefik + 示例 OpenClaw 基础镜像
- API 和 worker 可以从宿主机运行

### 测试虚拟机

- API 和 worker 的 systemd 服务
- 本地 Docker Engine
- 本地 PostgreSQL 或独立实例
- 如果有可用附加磁盘，租户工作空间根目录放在附加磁盘路径上

### 后续生产环境

- 如果规模需要，分离 DB 和 Docker 宿主机
- 添加远程镜像仓库
- 如果合规需要，添加外部密钥管理器

## 8. 运行时文件策略

worker 应为每次部署尝试生成版本化的运行时快照：

```text
/srv/openclaw/tenants/<tenant_id>/releases/<config_version>/
```

并暴露一个稳定的符号链接或指针指向当前运行时：

```text
/srv/openclaw/tenants/<tenant_id>/current -> releases/3
```

这简化了回滚和调试。

## 9. 建议的回滚策略

第一阶段优先使用 `replace` 部署语义：

1. 渲染新版本
2. 创建新容器
3. 等待健康检查
4. 如需要切换路由标签目标
5. 停止旧容器

如果 OpenClaw 路由模型使蓝绿部署对第一版本来说太重，使用 `recreate` 并伴随短暂的维护窗口，同时保留上次成功版本目录以实现快速恢复。

## 10. 风险和控制

### 风险：租户配置不完整
- 控制：部署前严格执行验证端点

### 风险：日志中的密钥泄露
- 控制：集中式掩码辅助函数和日志审查测试

### 风险：Docker 和 DB 状态漂移
- 控制：调和循环加审计跟踪

### 风险：宿主机容量耗尽
- 控制：调度部署任务前的硬性准入检查

### 风险：OpenClaw 启动缓慢或不确定
- 控制：健康等待超时、启动重试、显式失败状态

## 11. 文档审批验收清单

只有确认以下内容后才能开始实施：

1. Go 语言控制面已被接受
2. 一个共享基础镜像加运行时注入已被接受
3. 异步 API + `ultraworker` 任务模型已被接受
4. PostgreSQL 是租户部署状态的唯一事实来源
5. 如果需要外部访问，Traefik 路由已被接受
6. 第一个里程碑将针对一台 Ubuntu 主机，而非多主机调度
7. `openclawctl` 将为所有已批准 API 提供完整的运维覆盖

## 12. CLI 交付原则

CLI 交付应与 API 交付同步，不应落后于里程碑。对于每个已批准的端点组，相应的 `openclawctl` 命令组应在同一里程碑中实现。

## 13. 审批后建议的下一步

文档审批后，第一个实施任务应该是：

- 初始化仓库结构
- 创建迁移
- 实现 `POST /tenants`、`PUT /tenants/{tenant_id}/profile`、`PUT /tenants/{tenant_id}/secrets/{secret_key}` 和 `POST /tenants/{tenant_id}/profile/validate`

只有在这些稳定后，才能添加容器部署代码。
