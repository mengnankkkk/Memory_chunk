# Context Refiner 快速使用教程

- 文档版本：`v2026.04.06`
- 更新日期：`2026-04-06`
- 文档类型：`How-To / Quickstart`
- 适用代码基线：`main` 分支当前实现

## 1. 这份文档解决什么问题

本文档只聚焦一件事：

`如何把当前项目在本地跑起来，并完成一次最小调用。`

如果你想看架构，请看 [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)。

## 2. 运行前提

你至少需要：

- Go `1.25.x`
- 一个可访问的 Redis
- 本地能监听一个 gRPC 地址

当前项目默认配置是占位值，因此直接运行会失败，这是当前代码的已知状态，不是你操作错了。

## 3. 配置服务

编辑 [config/service.yaml](/E:/github/Memory_chunk/config/service.yaml)，至少填这两个字段：

```yaml
grpc:
  listen_addr: "127.0.0.1:50051"

redis:
  addr: "127.0.0.1:6379"
  username: ""
  password: ""
  db: 0
  key_prefix: "context-refiner:page"
  page_ttl: "24h"
  summary_stream: "context-refiner:summary-jobs"

tokenizer:
  model: "gpt-4o-mini"
  fallback_encoding: "cl100k_base"

pipeline:
  policy_file: "config/policies.yaml"
  default_policy: "strict_coding_assistant"
  paging_token_threshold: 320

summary_worker:
  enabled: true
  consumer_group: "context-refiner-summary"
  consumer_name: "worker-1"
  batch_size: 8
  block_timeout: "2s"
```

## 4. 启动 Redis

如果你本机已有 Redis，直接确认它在运行即可。

如果你使用 Docker，可参考：

```powershell
docker run --name context-refiner-redis -p 6379:6379 -d redis:7
```

## 5. 启动服务

在仓库根目录执行：

```powershell
go run ./cmd
```

如果配置正确，你会看到类似日志：

```text
refiner gRPC server listening on 127.0.0.1:50051
```

## 6. 最小调用方式

当前仓库没有附带命令行调用工具，最简单的方式是用 `grpcurl` 或任意 gRPC 客户端。

注意：

- 当前服务端没有开启 gRPC reflection
- 因此使用 `grpcurl` 时，建议显式传入 `-import-path . -proto api/refiner.proto`

### 6.1 调用 `Refine`

示例：

```powershell
grpcurl -plaintext -import-path . -proto api/refiner.proto -d "{
  \"messages\": [
    {\"role\": \"system\", \"content\": \"You are a coding assistant.\"},
    {\"role\": \"user\", \"content\": \"Summarize this context.\"}
  ],
  \"rag_chunks\": [
    {
      \"id\": \"chunk-1\",
      \"source\": \"doc-1\",
      \"fragments\": [
        {\"type\": \"FRAGMENT_TYPE_BODY\", \"content\": \"This is a very long body ...\"}
      ]
    }
  ],
  \"model\": {\"model\": \"gpt-4o-mini\", \"max_context_tokens\": 8192},
  \"token_budget\": 2048,
  \"policy\": \"strict_coding_assistant\",
  \"session_id\": \"demo-session\",
  \"request_id\": \"demo-request\"
}" 127.0.0.1:50051 refiner.v1.RefinerService/Refine
```

你会拿到：

- `optimized_prompt`
- `input_tokens`
- `output_tokens`
- `audits`
- `paged_chunks`
- `pending_summary_job_ids`

### 6.2 调用 `PageIn`

如果 `Refine` 返回了 `paged_chunks.page_keys`，你可以继续请求：

```powershell
grpcurl -plaintext -import-path . -proto api/refiner.proto -d "{
  \"page_keys\": [
    \"session:demo-session:request:demo-request:chunk:chunk-1:hash:abcdef123456:page:2\"
  ]
}" 127.0.0.1:50051 refiner.v1.RefinerService/PageIn
```

如果对应摘要已被 worker 写回，你会看到：

- `is_summary = true`
- `summary_job_id` 有值

如果还没有写回，就会返回原始 page 内容。

## 7. 常见失败原因

### 7.1 `grpc.listen_addr is required`

原因：

- [config/service.yaml](/E:/github/Memory_chunk/config/service.yaml) 还没填真实地址

### 7.2 `redis.addr is required`

原因：

- Redis 地址仍为空

### 7.3 `ping redis failed`

原因：

- Redis 没启动
- 地址或认证不对

### 7.4 `unknown policy`

原因：

- 请求中的 `policy` 不存在于 [config/policies.yaml](/E:/github/Memory_chunk/config/policies.yaml)

### 7.5 `load page failed`

原因：

- page key 错了
- TTL 到期了
- 对应内容尚未生成或已被清理

## 8. 当前使用上的已知限制

- 默认配置不会直接启动成功
- 当前 summary 仍是启发式摘要，不是外部模型摘要
- 当前没有官方提供的 demo client
- 当前没有 docker-compose 或一键启动脚本

## 9. 建议的最小验证顺序

1. 填写 `service.yaml`
2. 启动 Redis
3. 运行 `go build ./...`
4. 运行 `go run ./cmd`
5. 用 `grpcurl` 调一次 `Refine`
6. 观察是否返回 `paged_chunks`
7. 再调一次 `PageIn`

## 10. 下一步建议

如果你已经能跑通最小调用，下一步建议按这个顺序继续：

1. 看 [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)
2. 看 [docs/principles-and-internals.md](/E:/github/Memory_chunk/docs/principles-and-internals.md)
3. 看 [docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md)
