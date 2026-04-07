# Context Refiner 代码设计说明

- 文档版本：`v2026.04.07`
- 更新日期：`2026-04-07`
- 文档类型：`Code Reference`
- 适用代码基线：`main` 分支当前实现

## 1. 文档目标

本文档面向要读代码、改代码、扩展代码的人，重点描述：

- 包结构
- 核心对象
- 请求数据流
- Processor 设计
- Redis 设计
- 扩展点

它是“代码参考文档”，不是教程；如果你想快速跑起来，请看 [docs/quickstart.md](/E:/github/Memory_chunk/docs/quickstart.md)。

## 2. 仓库结构

```text
api/
cmd/
config/
docs/
internal/
  config/
  engine/
  heuristic/
  processor/
  server/
  store/
  summary/
  tokenizer/
pkg/
```

## 3. 包职责

### 3.1 `api/`

职责：

- 定义 gRPC 协议
- 固化对外服务边界

关键文件：

- [api/refiner.proto](/E:/github/Memory_chunk/api/refiner.proto)

### 3.2 `cmd/`

职责：

- 读取配置
- 初始化 tokenizer、Redis、registry、server、worker
- 启动 gRPC 服务

关键文件：

- [cmd/main.go](/E:/github/Memory_chunk/cmd/main.go)
- [cmd/runtime.go](/E:/github/Memory_chunk/cmd/runtime.go)
- [cmd/registry.go](/E:/github/Memory_chunk/cmd/registry.go)

### 3.3 `internal/config/`

职责：

- 解析服务配置
- 解析策略配置
- 做基础校验和默认值补全

关键文件：

- [internal/config/config.go](/E:/github/Memory_chunk/internal/config/config.go)
- [internal/config/policy.go](/E:/github/Memory_chunk/internal/config/policy.go)

### 3.4 `internal/engine/`

职责：

- 领域模型定义
- Pipeline 调度
- Registry 注册与解析

关键文件：

- [internal/engine/pipeline.go](/E:/github/Memory_chunk/internal/engine/pipeline.go)
- [internal/engine/registry.go](/E:/github/Memory_chunk/internal/engine/registry.go)

### 3.5 `internal/processor/`

职责：

- 具体上下文治理动作
- 请求拷贝
- Token 分片
- chunk 元信息辅助工具

关键文件：

- [internal/processor/request_clone.go](/E:/github/Memory_chunk/internal/processor/request_clone.go)
- [internal/processor/token_split.go](/E:/github/Memory_chunk/internal/processor/token_split.go)
- [internal/processor/chunk_metadata.go](/E:/github/Memory_chunk/internal/processor/chunk_metadata.go)
- [internal/processor/paging.go](/E:/github/Memory_chunk/internal/processor/paging.go)
- [internal/processor/collapse.go](/E:/github/Memory_chunk/internal/processor/collapse.go)
- [internal/processor/compact.go](/E:/github/Memory_chunk/internal/processor/compact.go)
- [internal/processor/structured.go](/E:/github/Memory_chunk/internal/processor/structured.go)
- [internal/processor/snip.go](/E:/github/Memory_chunk/internal/processor/snip.go)
- [internal/processor/auto.go](/E:/github/Memory_chunk/internal/processor/auto.go)
- [internal/processor/assemble.go](/E:/github/Memory_chunk/internal/processor/assemble.go)

### 3.6 `internal/heuristic/`

职责：

- 承接跨 `processor` / `summary` 复用的启发式文本整理规则
- 避免同一类 outline / json / error-stack 提取逻辑在多个包重复散落

关键文件：

- [internal/heuristic/json.go](/E:/github/Memory_chunk/internal/heuristic/json.go)
- [internal/heuristic/extract.go](/E:/github/Memory_chunk/internal/heuristic/extract.go)
- [internal/heuristic/lines.go](/E:/github/Memory_chunk/internal/heuristic/lines.go)

### 3.7 `internal/server/`

职责：

- gRPC handler
- request mapping
- response mapping
- mapping 辅助转换

关键文件：

- [internal/server/refiner.go](/E:/github/Memory_chunk/internal/server/refiner.go)
- [internal/server/request_mapping.go](/E:/github/Memory_chunk/internal/server/request_mapping.go)
- [internal/server/response_mapping.go](/E:/github/Memory_chunk/internal/server/response_mapping.go)
- [internal/server/mapping_helpers.go](/E:/github/Memory_chunk/internal/server/mapping_helpers.go)

### 3.8 `internal/store/`

职责：

- Redis 读写
- PageStore
- SummaryJobQueue
- SummaryJobConsumer

关键文件：

- [internal/store/redis.go](/E:/github/Memory_chunk/internal/store/redis.go)

### 3.9 `internal/summary/`

职责：

- summary worker
- 启发式摘要逻辑

关键文件：

- [internal/summary/worker.go](/E:/github/Memory_chunk/internal/summary/worker.go)
- [internal/summary/summarizer.go](/E:/github/Memory_chunk/internal/summary/summarizer.go)

### 3.10 `internal/tokenizer/`

职责：

- 封装 `tiktoken-go`
- 提供统一计数接口

关键文件：

- [internal/tokenizer/counter.go](/E:/github/Memory_chunk/internal/tokenizer/counter.go)

### 3.11 `pkg/client/`

职责：

- 提供一个很薄的 client wrapper
- 目前更多是接口适配层，不是完整 SDK

## 4. 核心对象

### 4.1 `RefineRequest`

当前内部请求对象是整个系统的核心载体。

它不仅承载原始输入，还承载运行态信息：

- `SessionID`
- `RequestID`
- `Messages`
- `RAGChunks`
- `Model`
- `Budget`
- `Policy`
- `RuntimePolicy`
- `CurrentTokens`
- `InputTokens`
- `OptimizedPrompt`
- `Audits`
- `Metadata`
- `PendingSummaryJobIDs`

### 4.2 `Processor`

当前 Processor 接口：

```go
type Processor interface {
    Descriptor() ProcessorDescriptor
    Process(ctx context.Context, req *RefineRequest) (*RefineRequest, ProcessResult, error)
}
```

设计意义：

- 引擎只依赖统一接口
- 处理器可独立扩展
- 每个处理器返回处理结果和审计信息

### 4.3 `ProcessorCapabilities`

当前能力字段：

- `Aggressive`
- `Lossy`
- `StructuredInputOnly`
- `MinTriggerTokens`
- `PreserveCitation`

引擎使用这些字段做跳过判断，而不是靠步骤名写白名单。

### 4.4 `StepAudit` 与 `StepSemanticAudit`

作用：

- 记录每一步前后 Token 变化
- 记录细节字段
- 记录删掉什么、保留什么、为什么删

这是当前代码里很重要的一层“可解释性设计”。

## 5. 请求处理流程

### 5.1 `Refine`

主链流程：

1. gRPC 收到 `RefineRequest`
2. `mapRequest` 转内部模型
3. 自动补 `request_id` / `session_id`
4. 解析策略
5. 构造 Pipeline
6. 统一计算 `InputTokens`
7. 顺序执行 Processor
8. 收集审计
9. 输出 `OptimizedPrompt`
10. `mapResponse` 返回 protobuf 响应

### 5.2 `PageIn`

流程：

1. 接收 `page_keys`
2. 对每个 key 优先尝试读取 summary
3. 没有 summary 时读取原始 page
4. 返回 `StoredPage`

## 6. Processor 设计细节

### 6.1 `paging`

作用：

- 对超长 chunk 分页
- 第一页保留在当前 prompt
- 全量页写入 Redis
- 返回 `page_refs`

关键点：

- 使用真实 Token 分页
- key 带作用域和内容 hash
- 入口注册仍在 `cmd/`，但初始化流程已拆成 `runtime.go` / `registry.go`，便于继续扩展

### 6.2 `collapse`

作用：

- 对重复 chunk 去重
- 合并来源引用

关键点：

- 不是简单删重，而是尽量保留 citation/source

### 6.3 `compact`

作用：

- 移除多余空行
- 做安全微压缩

关键点：

- 不做语义删除
- 不做结构重排

### 6.4 结构化处理器

当前包括：

- `json_trim`
- `table_reduce`
- `code_outline`
- `error_stack_focus`

特点：

- 只处理特定 `FragmentType`
- 在大多数情况下属于 lossy 但结构感知
- 通用遍历与审计组装逻辑已收敛到共享 helper，单个 processor 只保留“目标类型 + 变换函数 + 审计语义”

### 6.5 `snip`

作用：

- 对高密度长片段做 middle-out 截断

适用类型：

- `code`
- `tool-output`
- `log`
- `error-stack`
- `json`

### 6.6 `auto_compact_sync`

作用：

- 对日志、工具输出、错误栈做同步低风险清理

特点：

- 去重行
- 合并空行
- 不做模型摘要

### 6.7 `auto_compact_async`

作用：

- 把需要重型处理的 chunk 异步投递到 Stream

当前现状：

- 已经可以排队
- 已经有 worker 消费
- 但 worker 仍是启发式摘要
- worker 运行循环与摘要渲染逻辑已拆分到不同文件，减少控制流与文本规则混杂

### 6.8 `assemble`

作用：

- 统一最终 prompt 渲染
- 再次统一输出 Token 计数口径

## 7. Redis 设计

### 7.1 Page Key

当前 key 模式：

```text
session:{session_id}:request:{request_id}:chunk:{chunk_id}:hash:{content_hash}:page:{page_index}
```

设计价值：

- 避免不同请求冲突
- 内容变化后通过 `content_hash` 区分版本
- 排查问题时具有作用域信息

### 7.2 Summary 存储

当前实现是：

- 原始 page 用 page key 存
- summary 结果用 `summary:{page_key}` 变种 key 存

这意味着：

- 当前 summary 更偏 page 级附属物
- 还不是完整的独立 Artifact 模型

### 7.3 Summary Queue

当前使用 Redis Stream：

- 入队：`XADD`
- 消费组：`XGROUP`
- 消费：`XREADGROUP`
- 确认：`XACK`

## 8. 配置设计

### 8.1 `config/service.yaml`

当前主要配置项：

- `grpc.listen_addr`
- `redis.addr`
- `redis.key_prefix`
- `redis.page_ttl`
- `redis.summary_stream`
- `tokenizer.model`
- `pipeline.policy_file`
- `pipeline.default_policy`
- `pipeline.paging_token_threshold`
- `summary_worker.*`

### 8.2 `config/policies.yaml`

当前主要配置项：

- `name`
- `budget_ratio`
- `steps`
- `snip_params`
- `auto_compact_threshold`

## 9. 当前代码的主要扩展点

### 9.1 新增 Processor

推荐流程：

1. 在 `internal/processor/` 新增实现
2. 实现 `Descriptor()` 和 `Process()`
3. 在 `cmd/registry.go` 注册到 registry
4. 在 `config/policies.yaml` 中编排步骤
5. 更新 `docs/todolist.md`

### 9.2 升级 summary 能力

推荐流程：

1. 抽象 `SummaryProvider`
2. 把启发式摘要挪到 `HeuristicProvider`
3. 新增 `LLMProvider`
4. 升级 `SummaryResult` 为 `SummaryArtifact`
5. 更新 `PageIn` 回填逻辑

### 9.3 增加观测

推荐切入点：

- server 入口
- pipeline 每步
- Redis page/save/load
- summary queue enqueue/consume

## 10. 当前代码阅读顺序

推荐顺序：

1. [cmd/main.go](/E:/github/Memory_chunk/cmd/main.go)
2. [cmd/runtime.go](/E:/github/Memory_chunk/cmd/runtime.go)
3. [cmd/registry.go](/E:/github/Memory_chunk/cmd/registry.go)
4. [api/refiner.proto](/E:/github/Memory_chunk/api/refiner.proto)
5. [internal/server/refiner.go](/E:/github/Memory_chunk/internal/server/refiner.go)
6. [internal/server/request_mapping.go](/E:/github/Memory_chunk/internal/server/request_mapping.go)
7. [internal/server/response_mapping.go](/E:/github/Memory_chunk/internal/server/response_mapping.go)
8. [internal/engine/pipeline.go](/E:/github/Memory_chunk/internal/engine/pipeline.go)
9. [internal/processor/request_clone.go](/E:/github/Memory_chunk/internal/processor/request_clone.go)
10. [internal/processor/token_split.go](/E:/github/Memory_chunk/internal/processor/token_split.go)
11. [internal/processor/chunk_metadata.go](/E:/github/Memory_chunk/internal/processor/chunk_metadata.go)
12. [internal/processor/paging.go](/E:/github/Memory_chunk/internal/processor/paging.go)
13. [internal/processor/structured.go](/E:/github/Memory_chunk/internal/processor/structured.go)
14. [internal/heuristic/json.go](/E:/github/Memory_chunk/internal/heuristic/json.go)
15. [internal/heuristic/extract.go](/E:/github/Memory_chunk/internal/heuristic/extract.go)
16. [internal/heuristic/lines.go](/E:/github/Memory_chunk/internal/heuristic/lines.go)
17. [internal/store/redis.go](/E:/github/Memory_chunk/internal/store/redis.go)
18. [internal/summary/worker.go](/E:/github/Memory_chunk/internal/summary/worker.go)
19. [internal/summary/summarizer.go](/E:/github/Memory_chunk/internal/summary/summarizer.go)

## 11. 当前代码设计的优点与局限

### 优点

- 分层边界清楚
- Processor 扩展方式清楚
- Token 口径统一
- 语义审计思路正确
- 已经为后续摘要和缓存能力预留空间

### 局限

- summary 仍是启发式
- 没有测试保护重构
- 没有观测支撑调优
- 配置默认不可直接运行

## 12. 结论

当前代码设计已经具备“继续扩展不会立刻失控”的基础。

后续最重要的不是再继续往 `processor/` 塞逻辑，而是：

- 把测试补起来
- 把摘要抽象稳下来
- 把观测和评测补起来

这样代码设计才能真正进入稳定演进阶段。
