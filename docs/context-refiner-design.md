# Context Refiner 总体架构设计

- 文档版本：`v2026.04.06`
- 更新日期：`2026-04-06`
- 文档类型：`Architecture Overview`
- 适用代码基线：`main` 分支当前实现

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
gRPC Ingress
    |
    v
Request Mapping
    |
    v
Pipeline Engine
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
    +--> Redis PageStore / Summary Store / Stream Queue
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
- [internal/server/refiner.go](/E:/github/Memory_chunk/internal/server/refiner.go)

### 6.2 Protocol Mapping 层

职责：

- protobuf 与内部结构互转
- `request_id` 自动补全
- `source -> sources`
- `content -> fragments`

设计价值：

- 协议层兼容历史调用方式
- 内部逻辑统一围绕结构化对象工作

### 6.3 Pipeline Engine 层

职责：

- 统一调度 Processor
- 统一真实 Token 口径
- 基于 Processor 能力声明做跳过
- 汇总步骤级审计和语义审计

当前实现：

- [internal/engine/pipeline.go](/E:/github/Memory_chunk/internal/engine/pipeline.go)
- [internal/engine/registry.go](/E:/github/Memory_chunk/internal/engine/registry.go)

### 6.4 Processor 层

职责：

- 每个 Processor 只做单一职责的清洗/压缩动作
- 避免把复杂规则塞回引擎
- 形成稳定的扩展点

当前已实现的 Processor：

- `paging`
- `collapse`
- `compact`
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

当前实现：

- [internal/store/redis.go](/E:/github/Memory_chunk/internal/store/redis.go)
- [internal/summary/worker.go](/E:/github/Memory_chunk/internal/summary/worker.go)

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
  -> mapRequest
  -> resolve policy
  -> build pipeline
  -> count input tokens
  -> run processors in order
  -> assemble prompt
  -> mapResponse
  -> RefineResponse
```

### 8.2 Page-Out / Page-In 主链

```text
large chunk
  -> paging processor
  -> split by token
  -> save pages to Redis
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
  -> save summary to Redis summary key
  -> PageIn prefers summary result
```

## 9. 当前真实阶段判断

基于当前代码和构建结果，项目所处阶段不是“概念验证”，而是：

`核心底盘已成型，正在向产品化工程能力过渡`

更细化地说：

- Phase 1 核心底盘：大体完成
- Phase 2 摘要产品化：只做到了启发式闭环，未接真实外部模型
- Phase 3 观测评测：基本未开始
- Phase 4 结构化策略深化：已开头，尚未做深
- Phase 5 缓存复用：未开始

## 10. 当前约束与缺口

### 10.1 已确认存在的约束

- `config/service.yaml` 中真实地址仍为空
- `summary worker` 仍然是启发式摘要
- 回填结果当前更接近 page 级摘要，而不是 chunk 级摘要对象
- 没有自动化测试文件
- 没有 Prometheus / Tracing / 回归评测工具

### 10.2 这意味着什么

- 当前代码可以 `go build ./...`
- 当前代码也可以 `go test ./...`，但所有包都没有测试文件
- 当前系统主链可以视为“可开发、可继续扩展”
- 当前系统不能视为“可直接生产部署”

## 11. 下一阶段的架构演进方向

当前最值得推进的三条主线是：

1. 摘要 Provider 抽象
2. 观测与评测闭环
3. 更细粒度的结构化治理

推荐顺序：

1. 先补指标和集成测试
2. 再抽象 Summary Provider
3. 再接真实外部摘要模型
4. 再把摘要从 page 级字符串提升成 chunk 级结构化对象
5. 最后再推进缓存复用和高级处理器

## 12. 结论

当前架构方向是正确的，核心边界也已经比较清楚。

真正需要避免的不是“功能不够多”，而是两种常见偏航：

- 误把“可编译骨架”当成“生产完成品”
- 在没有测试、观测、评测的情况下继续堆复杂压缩规则

因此，当前文档体系和后续实施都应围绕一个中心原则：

`先把架构边界、质量闭环和摘要抽象做稳，再继续扩能力。`
