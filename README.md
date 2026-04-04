# Memory_chunk

一个位于 AI 应用层与大模型 API 之间的 Go gRPC 上下文清洗服务。

## 当前实现

- `Pipeline-Processor` 管线骨架
- 真实 `tiktoken-go` Token 计数，输入与输出口径统一
- 带能力声明的 Processor：`Aggressive / Lossy / StructuredInputOnly / MinTriggerTokens / PreserveCitation`
- 结构化 RAG Chunk：支持 `title / body / code / table / json / tool-output / log / error-stack`
- 真实 Redis PageStore，分页 key 含 `session_id + request_id + chunk_id + content_hash + page_index`
- 双级 `auto_compact`
- 同步级：安全压缩日志、工具输出、错误栈
- 异步级：写入 Redis Stream 摘要任务，等待后续 page-in
- 语义保真审计：记录删除/保留/原因/引用保留/代码围栏保留/错误栈保留
- `refiner.proto` 与真实 gRPC 服务端

## 快速运行

1. 在 [service.yaml](E:\github\Memory_chunk\config\service.yaml) 填入 `grpc.listen_addr` 和 `redis.addr`
2. 启动 Redis
3. 运行：

```powershell
go run ./cmd
```

## 下一步

- 增加真实异步摘要消费者
- 增加 Prometheus 指标与 Tracing
- 增加集成测试与压缩效果评测
