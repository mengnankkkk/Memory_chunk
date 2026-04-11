# Context Refiner 总体架构设计

- 文档版本：`v2026.04.11`
- 更新日期：`2026-04-11`
- 文档类型：`Architecture Overview`
- 适用代码基线：`main` 分支当前实现

> 2026-04-11 结构重构后，分层实现与目录职责请结合 [docs/layered-architecture.md](/E:/github/Memory_chunk/docs/layered-architecture.md) 一起看。

## 1. 文档目标

本文档回答四个问题：

1. 这个项目是什么。
2. 它解决什么问题。
3. 它当前的系统边界和架构分层是什么。
4. 当前代码已经把架构落到了什么程度。

如果你要快速导航全部文档，请先看 [docs/README.md](/E:/github/Memory_chunk/docs/README.md)。

## 2. 项目定位

`Context Refiner` 是一个位于 AI 应用层与大模型 API 层之间的上下文治理服务。

它的职责不是“普通文本压缩”，而是对送给大模型的上下文进行：

- 结构化整理
- Token 统一计数
- 安全微压缩
- 针对高密度片段的差异化裁剪
- page-out / page-in
- 异步摘要排队与摘要回填
- 语义审计

它试图解决的核心问题包括：

- 多轮对话导致上下文快速膨胀
- RAG 检索结果中存在大量重复、冗长、低价值内容
- Tool Output、日志、错误栈、JSON、表格、代码块等内容 Token 密度过高
- 只追求压缩率会损伤语义和证据可追踪性

## 3. 架构目标

当前架构面向以下目标：

- 统一 Token 口径，避免估算逻辑前后不一致
- 把清洗过程拆成独立 Processor，避免大函数堆逻辑
- 支持结构化 RAG 输入，而不是只处理纯字符串
- 支持可解释审计，避免做了 lossy 压缩却无法追责
- 支持分页与异步摘要，为后续缓存复用和重型压缩能力打底
- 保留演进空间，让后续摘要 Provider、观测能力、缓存能力可以插入
- 尽可能把稳定上下文前置，提升下游 prefix / KV cache 命中率

## 4. 非目标

当前版本明确不是以下系统：

- 不是向量检索系统
- 不是模型网关
- 不是完整的 Prompt 编排平台
- 不是带 Web UI 的运维平台
- 不是生产级观测完备的服务

这几个非目标很重要，因为它决定了当前代码虽然“能编译、能运行主链”，但还不能被误判成“已经产品化完成”。

## 5. 总体逻辑架构

```text
Client / App
    |
    v
gRPC Adapter
    |
    v
Application Service
    |
    v
Core Pipeline
    |
    +--> Processor Chain
    |      |- paging
    |      |- collapse
    |      |- compact
    |      |- structured processors
    |      |- snip
    |      |- auto_compact_sync
    |      |- auto_compact_async
    |      `- assemble
    |
    +--> Audit Output
    |
    +--> Infra Store / Summary Queue / Tokenizer / Observability
    |
    `--> PageIn / Summary Fallback
```

## 6. 分层设计

### 6.1 Ingress 层

职责：

- 接收 `Refine` / `PageIn` gRPC 请求
- 做最外层参数接入
- 把 protobuf 模型映射为内部领域模型

当前实现：

- [api/refiner.proto](/E:/github/Memory_chunk/api/refiner.proto)
- [internal/adapter/grpc/refiner_handler.go](/E:/github/Memory_chunk/internal/adapter/grpc/refiner_handler.go)

### 6.2 Application Service / Mapping 层

职责：

- 承接公开的 service API
- protobuf 与内部结构互转
- `request_id` 自动补全
- `source -> sources`
- `content -> fragments`

设计价值：

- gRPC 传输层与应用服务解耦
- 协议层兼容历史调用方式
- 内部逻辑统一围绕结构化对象工作

当前实现：

- [pkg/service/refiner.go](/E:/github/Memory_chunk/pkg/service/refiner.go)
- [internal/service/refiner_service.go](/E:/github/Memory_chunk/internal/service/refiner_service.go)
- [internal/service/request_mapping.go](/E:/github/Memory_chunk/internal/service/request_mapping.go)
- [internal/service/response_mapping.go](/E:/github/Memory_chunk/internal/service/response_mapping.go)

### 6.3 Pipeline Engine 层

职责：

- 统一调度 Processor
- 统一真实 Token 口径
- 基于 Processor 能力声明做跳过
- 汇总步骤级审计和语义审计

当前实现：

- [internal/core/pipeline.go](/E:/github/Memory_chunk/internal/core/pipeline.go)
- [internal/core/registry.go](/E:/github/Memory_chunk/internal/core/registry.go)

### 6.4 Processor 层

职责：

- 每个 Processor 只做单一职责的清洗/压缩动作
- 避免把复杂规则塞回引擎
- 形成稳定的扩展点
- 在应用层尽可能把输入稳定化，减少 prompt 形态抖动

当前已实现的 Processor：

- `paging`
- `collapse`
- `compact`
- `canonicalize`
- `json_trim`
- `table_reduce`
- `code_outline`
- `error_stack_focus`
- `snip`
- `auto_compact_sync`
- `auto_compact_async`
- `assemble`

### 6.5 State / Queue 层

职责：

- 分页内容存 Redis
- 摘要结果存 Redis
- 异步摘要任务写入 Redis Stream
- worker 通过 consumer group 消费任务
- page / summary 尽量基于 content-addressed artifact key 复用

当前实现：

- [internal/core/repository/repository.go](/E:/github/Memory_chunk/internal/core/repository/repository.go)
- [internal/observability/recorder.go](/E:/github/Memory_chunk/internal/observability/recorder.go)
- [internal/infra/store/redis/repository.go](/E:/github/Memory_chunk/internal/infra/store/redis/repository.go)
- [internal/infra/summary/worker.go](/E:/github/Memory_chunk/internal/infra/summary/worker.go)
- [internal/infra/observability/prometheus.go](/E:/github/Memory_chunk/internal/infra/observability/prometheus.go)

### 6.6 Egress 层

职责：

- 返回压缩后的 prompt
- 返回审计信息
- 返回分页引用
- 返回 pending summary job id

## 7. 核心领域模型

### 7.1 Message

表示对话消息：

- `role`
- `content`

### 7.2 RAGFragment

结构化片段类型：

- `title`
- `body`
- `code`
- `table`
- `json`
- `tool-output`
- `log`
- `error-stack`

设计意义：

- 让代码、日志、JSON、表格不再走同一套粗暴规则
- 为结构感知压缩提供稳定输入边界

### 7.3 RAGChunk

当前由四部分构成：

- `id`
- `source / sources`
- `fragments`
- `page_refs`

### 7.4 RefineRequest / RefineResponse

内部请求对象同时携带：

- 会话作用域
- 运行时策略
- 当前 Token
- 输入 Token
- 优化后 prompt
- 审计结果
- 元数据
- 异步摘要 job 引用

这意味着 pipeline 运行过程中是“就地演进请求对象”，而不是不断构造零散返回值。

## 8. 核心处理链路

### 8.1 Refine 主链

```text
RefineRequest
  -> gRPC adapter
  -> mapRequest
  -> resolve policy
  -> build pipeline
  -> count input tokens
  -> run processors in order
  -> canonicalize stable rag ordering / sources
  -> assemble prompt
  -> mapResponse
  -> RefineResponse
```

### 8.2 Page-Out / Page-In 主链

```text
large chunk
  -> paging processor
  -> split by token
  -> save pages to Redis with content-addressed artifact key
  -> keep first page in prompt
  -> expose page_refs
  -> PageIn loads page or summary
```

### 8.3 异步摘要主链

```text
large chunk after sync compaction
  -> auto_compact_async
  -> enqueue SummaryJob to Redis Stream
  -> worker consume job
  -> heuristic summarize
  -> save summary to Redis summary key for shared page refs
  -> PageIn prefers summary result
```

## 8.4 面向 KV Cache 的 Prompt 设计

当前实现已经把 prompt 组装改成“稳定块优先、动态块后置”：

```text
# Stable Context
## RAG
  ... 稳定排序后的 RAG chunks

# Conversation Memory
  ... 历史消息

# Active Turn
  ... 当前最新一轮消息
```

这样做的目标不是改变业务语义，而是让：

- 更长、更贵、更稳定的上下文尽量位于公共前缀
- 最新用户输入落在 suffix，减少对共享前缀的破坏
- RAG source 与 chunk 顺序可预测，降低检索抖动带来的 cache miss

## 8.5 应用层 Prefix Cache 策略

在 8.4 的稳定前缀布局之上，当前代码已经继续做了应用层 prefix cache 的第二阶段改造。

这里要刻意说清边界：

- 它不是模型 serving 层 KV block 复用
- 它不是 GPU/CPU/SSD 多级 KV 管理
- 它做的是应用层 prompt 稳定化、prefix 身份登记、命中诊断与策略控制

当前已完成的能力可以拆成四层：

### A. 规范化增强

- 清洗时间戳、UUID、长 hex id、request/session/trace 等高抖动字段
- 对 `system / memory / rag` 做分段稳定化
- 对 JSON 做稳定化并剔除 volatile keys
- 对 source、fragment、page refs 做去抖与稳定排序

### B. 分层 Prefix 身份

当前不仅记录一个总前缀，而是同时记录：

- `combined_prefix_hash`
- `system_prefix_hash`
- `memory_prefix_hash`
- `rag_prefix_hash`

并记录各段 token 数，用于判断到底是哪一段在破坏稳定前缀。

### C. Miss Reason 诊断

当前已经能区分：

- `empty`
- `short_prefix`
- `low_value_prefix`
- `ttl_expired`
- `hash_changed`
- `model_changed`

如果是 `hash_changed`，还会继续细分：

- `system_changed`
- `memory_changed`
- `rag_changed`
- `normalization_changed`
- `combined_changed`

### D. 应用层 Cache 策略

当前已实现：

- `admission policy`
- `namespace`
- `TTL 分层`
- `热点前缀统计`
- `prewarm`

具体行为是：

- 过短前缀会因 `min_stable_prefix_tokens` 被跳过
- 低价值前缀会因 `min_segment_count` 被跳过
- namespace 按 `tenant / policy / model` 组合，避免不同流量池互相污染
- 默认 TTL 使用 `redis.prefix_cache_ttl`
- 命中次数达到 `prefix_cache.hot_threshold` 后切到 `prefix_cache.hot_ttl`
- 热点统计按 namespace 维度记录，而不是全局混用
- `prewarm` 在服务启动时根据配置写入 prefix registry，不新增对外 API

## 8.6 当前观测边界

当前 Prometheus 与 tracing 已经能观测：

- prefix lookup 是 `hit / created / skipped`
- miss reason 是什么
- stable prefix token 规模是多少
- 当前 prefix 是否进入 `hot` 档
- 当前 TTL tier 是 `default` 还是 `hot`

但这些指标表达的是：

- 应用层 prefix cache 是否更容易被复用
- 应用层稳定前缀是否足够长、足够稳定

它们不等于推理引擎真实 KV block 命中。

## 9. 当前真实阶段判断

基于当前代码和构建结果，项目所处阶段不是“概念验证”，而是：

`核心底盘已成型，正在向产品化工程能力过渡`

更细化地说：

- Phase 1 核心底盘：大体完成
- Phase 2 摘要产品化：只做到了启发式闭环，未接真实外部模型
- Phase 3 观测评测：已完成 Prometheus 指标、OTel tracing 与 Grafana dashboard 基线接入
- Phase 4 结构化策略深化：已开头，尚未做深
- Phase 5 应用层缓存复用：A/B/C/D 已完成，E/F 尚未完成

## 10. 当前约束与缺口

### 10.1 已确认存在的约束

- `config/service.yaml` 中真实地址仍为空
- `summary worker` 仍然是启发式摘要
- 回填结果当前更接近 page 级摘要，而不是 chunk 级摘要对象
- 自动化测试仍然不足，目前只有少量单测起步
- 已有 Prometheus Metrics
- 已有应用层 Tracing 与 Dashboard
- 没有回归评测工具
- 当前还没有 `dry_run / explain / cache debug`
- 当前还没有 replay 驱动的 prefix churn / hit ratio 离线评测

### 10.2 这意味着什么

- 当前代码可以 `go build ./...`
- 当前代码也可以 `go test ./...`，但测试覆盖仍不足以保护主链
- 当前系统主链可以视为“可开发、可继续扩展”
- 当前系统不能视为“可直接生产部署”

## 11. 下一阶段的架构演进方向

当前最值得推进的三条主线是：

1. 应用层 KV 的 Explain / 观测 / 评测闭环
2. 摘要 Provider 抽象
3. 更细粒度的结构化治理

推荐顺序：

1. 先补 `dry_run / explain / cache debug / normalized preview`
2. 再补 replay、dashboard、alerting 与 prefix churn 分析
3. 再抽象 Summary Provider
4. 再接真实外部摘要模型
5. 再把摘要从 page 级字符串提升成 chunk 级结构化对象

## 12. 结论

当前架构方向是正确的，核心边界也已经比较清楚。

真正需要避免的不是“功能不够多”，而是两种常见偏航：

- 误把“可编译骨架”当成“生产完成品”
- 在没有测试、观测、评测的情况下继续堆复杂压缩规则

因此，当前文档体系和后续实施都应围绕一个中心原则：

`先把架构边界、质量闭环和摘要抽象做稳，再继续扩能力。`
