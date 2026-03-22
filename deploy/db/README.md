# OpenClaw Database Deployment

独立部署 PostgreSQL 数据库。

## 快速启动

```bash
cd deploy/db

# 复制环境配置
cp .env.example .env
vim .env  # 修改密码

# 启动
./openclaw-db.sh start

# 运行迁移
./openclaw-db.sh migrate

# 查看连接信息
./openclaw-db.sh connection
```

## 配置文件

`.env` 文件：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `POSTGRES_USER` | `openclaw` | 数据库用户名 |
| `POSTGRES_PASSWORD` | `openclaw` | 数据库密码 |
| `POSTGRES_DB` | `openclaw_autodeploy` | 数据库名 |
| `POSTGRES_PORT` | `5432` | 宿主机映射端口 |

## 命令

```bash
./openclaw-db.sh start        # 启动
./openclaw-db.sh stop         # 停止
./openclaw-db.sh restart      # 重启
./openclaw-db.sh status       # 查看状态
./openclaw-db.sh logs         # 查看日志
./openclaw-db.sh migrate      # 运行迁移 (默认从 ../../migrations)
./openclaw-db.sh migrate /path/to/migrations  # 指定迁移目录
./openclaw-db.sh seed         # 写入种子数据 (默认 ../../scripts/seed.sql)
./openclaw-db.sh seed /path/to/seed.sql
./openclaw-db.sh backup       # 备份到 ./backups/
./openclaw-db.sh shell        # 进入 psql
./openclaw-db.sh connection   # 打印连接字符串
```

## 远程部署

在 DB 服务器上：

```bash
scp -r deploy/db user@db-server:/opt/openclaw-db
ssh user@db-server
cd /opt/openclaw-db
cp .env.example .env
vim .env  # 设置密码和端口
./openclaw-db.sh start
./openclaw-db.sh migrate
./openclaw-db.sh connection  # 拿到连接字符串给 API server 用
```

## Control Plane 连接配置

API server 的 `config.yaml` 中配置数据库连接：

```yaml
database:
  url: "postgres://openclaw:YOUR_PASSWORD@DB_SERVER_IP:5432/openclaw_autodeploy?sslmode=disable"
```

或环境变量：
```bash
export OPENCLAW_DATABASE_URL="postgres://openclaw:YOUR_PASSWORD@DB_SERVER_IP:5432/openclaw_autodeploy?sslmode=disable"
```
