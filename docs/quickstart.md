# Context Refiner 快速使用教程

- 文档版本：`v2026.04.18`
- 更新日期：`2026-04-18`
- 文档类型：`How-To / Quickstart`
- 适用代码基线：`main`

## 1. 这份文档解决什么问题

本文档聚焦两件事：

- 如何按当前代码在宿主机模式启动服务
- 如何切换到根目录 `docker compose` 一键部署模式

## 2. 当前推荐运行模式

当前仓库推荐采用下面这两套模式，根据你的目标选择：

### 模式 A：本机开发模式

- `context-refiner` 项目进程运行在宿主机
- Redis、Prometheus、Grafana、Tempo、OTel Collector 运行在 Docker 中

### 模式 B：Docker Compose 一键部署模式

- `context-refiner`、Redis、Prometheus、Grafana、Tempo、OTel Collector 全部运行在 Docker 中
- 在仓库根目录直接执行 `docker compose up -d --build`
- 部署完成后直接访问 `http://127.0.0.1:18080`

推荐本机开发模式的原因：

- 项目代码改动后直接 `go run` 即可，不需要每次重建业务镜像
- Redis 和观测栈由 Docker 提供，环境更稳定
- 当前仓库里的默认配置已经按这套方式对齐

## 3. 默认配置

当前 [config/service.yaml](/E:/github/Memory_chunk/config/service.yaml) 已经按本机模式对齐，关键值如下：

```yaml
grpc:
  listen_addr: "127.0.0.1:15051"

web:
  enabled: true
  listen_addr: "127.0.0.1:18080"
  page_size: 8

observability:
  metrics_enabled: true
  metrics_listen_addr: ":19091"
  metrics_path: "/metrics"
  tracing_enabled: true
  tracing_endpoint: "localhost:14318"
  tempo_query_url: "http://localhost:13200"
  tracing_insecure: true
  tracing_sample_rate: 1.0

redis:
  addr: "127.0.0.1:16379"
```

含义：

- 项目自己监听：
  - gRPC `127.0.0.1:15051`
  - Dashboard `127.0.0.1:18080`
  - Metrics `:19091`
- 项目依赖 Docker 暴露到宿主机的：
  - Redis `127.0.0.1:16379`
  - OTel Collector `localhost:14318`
  - Tempo `http://localhost:13200`

注意：

- Metrics 使用 `:19091`，而不是 `127.0.0.1:19091`
- 这是为了让 Docker 中的 Prometheus 能通过 `host.docker.internal:19091` 抓取宿主机指标
- `deploy/observability/docker-compose.yaml` 只适用于“应用跑宿主机”的模式
- 如果你想把整个应用一起放进 Docker，不要用这个目录入口，直接用根目录 `docker compose`

## 4. 启动 Docker 基础设施

在仓库根目录执行：

```powershell
cd deploy/observability
docker compose up -d
```

这会启动：

- Redis
- Prometheus
- Grafana
- Tempo
- OTel Collector

默认端口：

- Redis: `127.0.0.1:16379`
- Grafana: `http://localhost:13000`
- Prometheus: `http://localhost:19090`
- Tempo: `http://localhost:13200`
- OTel Collector: `http://localhost:14318`

## 5. 启动项目

回到仓库根目录执行：

```powershell
go run ./cmd/refiner
```

如果配置与 Docker 基础设施一致，你会看到类似日志：

```text
refiner gRPC server listening on 127.0.0.1:15051
metrics HTTP server listening on :19091/metrics
dashboard HTTP server listening on http://127.0.0.1:18080
```

## 6. 启动后可访问内容

- Dashboard: `http://127.0.0.1:18080`
- Metrics: `http://127.0.0.1:19091/metrics`
- Grafana: `http://localhost:13000`
- Prometheus: `http://localhost:19090`

当前 Dashboard 会展示：

- 服务运行状态
- TraceQL 查询入口
- trace 查询结果列表
- 单条 trace 的中文摘要、服务分布与 span 时间线

## 7. 最小验证顺序

1. 运行 `docker compose up -d`
2. 运行 `go build ./...`
3. 运行 `go run ./cmd/refiner`
4. 打开 `http://127.0.0.1:18080`
5. 打开 `http://127.0.0.1:19091/metrics`
6. 如需观测 tracing，再打开 `http://localhost:13000`

## 8. 常见失败原因

### 8.1 `ping redis failed`

原因：

- Docker 里的 Redis 没启动
- 宿主机 `16379` 端口被别的 Redis 占用

### 8.2 Prometheus 抓不到指标

原因：

- 项目没有启动
- 项目没有监听 `:19091`
- Docker Desktop 的 `host.docker.internal:19091` 不可达

### 8.3 tracing 没有数据

原因：

- OTel Collector 没启动
- 项目 `tracing_endpoint` 不是 `localhost:14318`
- Tempo / OTel Collector 的 OTLP receiver 没有绑定到 `0.0.0.0`
- 当前没有产生业务请求，span 很少

## 9. 当前这套本机模式的边界

- 这是开发模式，不是生产部署模式
- 当前业务服务本身仍然是手动 `go run` 启动，不是业务容器化部署
- 当前 summary 仍然是启发式 provider，不是真实外部摘要模型
- 当前还没有 `dry_run / explain / cache debug` 对外接口

## 10. Docker Compose 一键部署

如果你的目标不是本机开发，而是“整个应用直接容器化部署并访问”，在仓库根目录执行：

```bash
docker compose up -d --build
```

启动成功后可直接访问：

- Dashboard: `http://127.0.0.1:18080`
- Metrics: `http://127.0.0.1:19091/metrics`
- Grafana: `http://localhost:13000`
- Prometheus: `http://localhost:19090`

对应容器配置会自动加载 `config/service.docker.yaml`，并在容器网络中使用：

- Redis: `redis:6379`
- OTel Collector: `otel-collector:4318`
- Tempo: `http://tempo:3200`

如果当前机器已经启动过 `deploy/observability` 下的旧观测栈，根目录一键部署前建议先执行：

```bash
cd deploy/observability
docker compose down
cd ../..
docker compose up -d --build
```
