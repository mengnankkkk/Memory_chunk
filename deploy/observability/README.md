# Observability Stack

该目录现在提供一套本机开发用基础设施，用于让 `context-refiner` 在宿主机运行时，统一接入：

- Redis
- Metrics
- Traces

## 组件

- `Redis`：提供 page artifact、summary job、prefix cache 存储
- `Prometheus`：抓取 `/metrics`
- `Grafana`：加载预置 datasource 和 dashboard
- `Tempo`：存储 tracing 数据
- `OTel Collector`：接收应用通过 OTLP HTTP 上报的 traces，再转发到 Tempo

## 启动

```bash
cd deploy/observability
docker compose up -d
```

这个入口只适用于：

- `context-refiner` 进程跑在宿主机
- Redis、Prometheus、Grafana、Tempo、OTel Collector 跑在 Docker

如果你要把整个应用一起容器化，请回到仓库根目录使用 `docker compose up -d --build`。

## 默认端口

- Redis: `127.0.0.1:16379`
- Grafana: `http://localhost:13000`
- Prometheus: `http://localhost:19090`
- Tempo: `http://localhost:13200`
- OTel Collector OTLP HTTP: `http://localhost:14318`

## 应用配置建议

应用配置文件可保持如下设置：

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
  username: ""
  password: ""
  db: 0
```

说明：

- 项目进程运行在宿主机上
- Redis、Prometheus、Grafana、Tempo、OTel Collector 运行在 Docker 中
- Prometheus 通过 `host.docker.internal:19091` 抓取宿主机指标
- 项目通过 `localhost:14318` 把 tracing 发给 Docker 中暴露到宿主机的 OTel Collector
- Redis 通过 `127.0.0.1:16379` 提供给宿主机项目访问

## 本机推荐启动顺序

1. 在 `deploy/observability` 下执行 `docker compose up -d`
2. 在仓库根目录执行 `go run ./cmd/refiner`
3. 打开 `http://127.0.0.1:18080` 查看 Dashboard
4. 打开 `http://localhost:13000` 查看 Grafana

## 这次统一修正了什么

- 补上了本机运行所需的 `redis` 容器
- 保持应用 Redis 地址与 Docker 暴露端口一致：`127.0.0.1:16379`
- 保持 tracing 地址与 Docker 暴露端口一致：`localhost:14318`
- 保持 metrics 监听方式与 Prometheus 抓取方式一致：应用监听 `:19091`，Prometheus 抓 `host.docker.internal:19091`
- 移除了 `tempo` 与 `otel-collector` 同时占用宿主机 `4317` 的冲突配置
