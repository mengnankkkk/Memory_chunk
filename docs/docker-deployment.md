# Docker Compose 一键部署指南

## 概述

现在可以直接在仓库根目录使用 `docker compose` 一键部署完整的 Context Refiner 应用栈，包括：

- **context-refiner-app**：核心服务（gRPC + REST API + Dashboard）
- **redis**：数据存储
- **prometheus**：指标收集
- **tempo**：链路追踪
- **otel-collector**：OpenTelemetry 收集器
- **grafana**：可视化面板

## 快速开始

### 1. 一键启动所有服务

```bash
cd /path/to/Memory_chunk
docker compose up -d --build
```

### 2. 查看服务状态

```bash
docker compose ps
```

如果你所在网络访问 `proxy.golang.org` 不稳定，可在构建前覆盖 `GOPROXY`：

```bash
GOPROXY=https://goproxy.cn,direct docker compose up -d --build
```

### 3. 查看日志

```bash
# 查看所有服务日志
docker compose logs -f

# 查看 refiner 服务日志
docker compose logs -f refiner
```

### 4. 停止所有服务

```bash
docker compose down
```

### 5. 停止并清理数据

```bash
docker compose down -v
```

## 访问入口

启动成功后，可以通过以下地址访问：

| 服务 | 地址 | 说明 |
|------|------|------|
| Dashboard | http://localhost:18080 | 前端面板 + REST API |
| gRPC | localhost:15051 | gRPC 服务端口 |
| Grafana | http://localhost:13000 | 可视化面板（admin/admin） |
| Prometheus | http://localhost:19090 | 指标查询 |
| Metrics | http://localhost:19091/metrics | Prometheus 指标端点 |

## REST API 使用示例

```bash
curl -X POST http://localhost:18080/api/refine \
  -H "Content-Type: application/json" \
  -d '{
    "system": "你是代码助手",
    "messages": [{"role":"user","content":"帮我排查问题"}],
    "rag": ["日志片段"],
    "budget": 200
  }'
```

## 端口映射

| 容器内端口 | 宿主机端口 | 服务 |
|-----------|-----------|------|
| 15051 | 15051 | gRPC |
| 18080 | 18080 | Dashboard + REST API |
| 19091 | 19091 | Metrics |
| 6379 | 16379 | Redis |
| 3200 | 13200 | Tempo |
| 4318 | 14318 | OTel Collector |
| 9090 | 19090 | Prometheus |
| 3000 | 13000 | Grafana |

## 配置说明

### 本地开发配置（宿主机运行）

使用 `config/service.yaml`：
- Redis: `127.0.0.1:16379`
- OTel: `localhost:14318`
- Tempo: `http://localhost:13200`

### Docker 部署配置（容器运行）

使用 `config/service.docker.yaml`：
- Redis: `redis:6379`
- OTel: `otel-collector:4318`
- Tempo: `http://tempo:3200`

容器启动时会自动通过 `CONFIG_FILE` 环境变量加载 `service.docker.yaml`。

同时，根目录 `docker-compose.yml` 会：

- 直接构建根目录 `Dockerfile`
- 使用容器内配置，不再依赖把宿主机 `config` 目录挂进去
- 为 `redis` 增加健康检查
- 让 Prometheus 改为抓取容器内的 `refiner:19091`

## 数据持久化

以下数据会持久化到 Docker volumes：

- `redis-data`：Redis 数据
- `prometheus-data`：Prometheus 时序数据
- `tempo-data`：Tempo 链路数据
- `grafana-data`：Grafana 配置和面板

## 常见问题

### 1. 端口冲突

如果端口被占用，修改根目录 `docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "18080:18080"  # 改为 "28080:18080"
```

### 2. 构建失败

确保在项目根目录有 `go.mod` 和 `go.sum`：

```bash
cd /path/to/Memory_chunk
ls go.mod go.sum
```

如果错误发生在 `go mod download`，通常是容器内 Go 模块代理不可达，可改用自定义 `GOPROXY`：

```bash
GOPROXY=https://goproxy.cn,direct docker compose build --no-cache refiner
```

### 3. 服务无法连接

检查容器网络：

```bash
docker network inspect context-refiner_refiner-net
```

### 4. 重新构建镜像

```bash
docker compose build --no-cache refiner
docker compose up -d
```

### 5. 查看 refiner 启动日志

```bash
docker compose logs refiner | head -20
```

应该看到：
```
refiner gRPC server listening on 0.0.0.0:15051
dashboard HTTP server listening on http://0.0.0.0:18080
metrics HTTP server listening on 0.0.0.0:19091/metrics
```

## 健康检查

```bash
# 检查 Dashboard
curl http://localhost:18080/api/snapshot

# 检查 Metrics
curl http://localhost:19091/metrics

# 检查 Redis
docker compose exec redis redis-cli PING

# 检查 Tempo
curl http://localhost:13200/api/search
```

## 生产部署建议

1. **修改 Grafana 密码**：
   ```yaml
   environment:
     GF_SECURITY_ADMIN_PASSWORD: your-secure-password
   ```

2. **配置 Redis 密码**：
   ```yaml
   redis:
     command:
       - "redis-server"
       - "--requirepass"
       - "your-redis-password"
   ```

3. **限制资源**：
   ```yaml
   refiner:
     deploy:
       resources:
         limits:
           cpus: '2'
           memory: 2G
   ```

4. **配置日志驱动**：
   ```yaml
   refiner:
     logging:
       driver: "json-file"
       options:
         max-size: "10m"
         max-file: "3"
   ```

5. **使用外部 Redis**：
   注释掉 `docker-compose.yml` 中的 `redis` 服务，修改 `config/service.docker.yaml`：
   ```yaml
   redis:
     addr: "your-redis-host:6379"
     password: "your-password"
   ```

## 兼容旧入口

如果你仍然想使用原来的观测目录入口，也可以执行：

```bash
cd deploy/observability
docker compose -f docker-compose-all.yaml up -d
```

它现在与根目录方案保持一致，也已改为抓取容器内 `refiner:19091` 指标。

## 更新日志

- `2026-04-18`：新增根目录 `docker-compose.yml`，支持仓库根目录一键部署完整应用栈，并修复完整容器部署时 Prometheus 抓取目标仍指向宿主机的问题。
