# Observability Stack

该目录提供一个最小可运行的观测栈，用于接收 `context-refiner` 的 Metrics 与 Traces。

## 组件

- `Prometheus`：抓取 `/metrics`
- `Grafana`：加载预置 datasource 与 dashboard
- `Tempo`：存储 tracing 数据
- `OTel Collector`：接收应用通过 OTLP HTTP 上报的 traces，再转发到 Tempo

## 启动

```bash
cd deploy/observability
docker compose up -d
```

## 默认端口

- Grafana: `http://localhost:3000`
- Prometheus: `http://localhost:9090`
- Tempo: `http://localhost:3200`
- OTel Collector OTLP HTTP: `http://localhost:4318`

## 应用配置建议

应用配置文件可保持如下设置：

```yaml
observability:
  metrics_enabled: true
  metrics_listen_addr: ":9091"
  metrics_path: "/metrics"
  tracing_enabled: true
  tracing_endpoint: "localhost:4318"
  tracing_insecure: true
  tracing_sample_rate: 1.0
```

Prometheus 默认通过 `host.docker.internal:9091` 抓取宿主机上的应用指标，适配当前 Windows 本地开发方式。
