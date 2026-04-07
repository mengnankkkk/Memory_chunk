# Context Refiner 学习解析文档

- 文档版本：`v2026.04.06`
- 更新日期：`2026-04-06`
- 文档类型：`Learning Guide`
- 适用代码基线：`main` 分支当前实现

## 1. 这份文档给谁看

这份文档主要给三类人：

- 第一次接触这个项目的人
- 想系统理解代码而不是只会跑命令的人
- 想把这个项目当成“上下文治理服务范例”来学习的人

## 2. 学这个项目之前，先抓住两个核心问题

### 问题 1：它到底在压什么

它压的不是“任意文本”，而是送给 LLM 的上下文。

这意味着：

- 压缩率不是唯一目标
- 语义可用性很重要
- 证据引用不能随便丢
- 代码块、日志、错误栈、JSON、表格不应该按同一规则处理

### 问题 2：它为什么不是一个简单函数

因为现实输入不是一段整洁的字符串，而是：

- 对话消息
- RAG chunks
- 结构化片段
- 分页引用
- 异步摘要任务
- 回填内容

所以它天然更像一个服务，而不是一个工具函数。

## 3. 推荐学习顺序

### 第一步：先理解系统目标

读：

- [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)

你需要先回答：

- 为什么要有这个服务
- 这个服务在调用链中的位置是什么
- 它和普通文本摘要器有什么差别

### 第二步：再理解最外层入口

读：

- [cmd/main.go](/E:/github/Memory_chunk/cmd/main.go)
- [api/refiner.proto](/E:/github/Memory_chunk/api/refiner.proto)

要观察：

- 程序从哪里启动
- 暴露了哪些 RPC
- 服务初始化了哪些组件

### 第三步：理解请求如何进入内部模型

读：

- [internal/server/refiner.go](/E:/github/Memory_chunk/internal/server/refiner.go)

要重点看：

- `mapRequest`
- `mapResponse`
- `Refine`
- `PageIn`

这一层决定了协议边界和内部对象边界。

### 第四步：理解 pipeline 是怎么跑的

读：

- [internal/engine/pipeline.go](/E:/github/Memory_chunk/internal/engine/pipeline.go)

你需要理解：

- 为什么 `RefineRequest` 会被逐步演进
- 为什么 Processor 有 capability
- 为什么 audit 是在 pipeline 中统一生成

### 第五步：理解 Processor 的分工

读：

- [internal/processor/paging.go](/E:/github/Memory_chunk/internal/processor/paging.go)
- [internal/processor/collapse.go](/E:/github/Memory_chunk/internal/processor/collapse.go)
- [internal/processor/structured.go](/E:/github/Memory_chunk/internal/processor/structured.go)
- [internal/processor/auto.go](/E:/github/Memory_chunk/internal/processor/auto.go)

学习时不要只看“做了什么”，还要看“为什么这一类逻辑被拆成独立 Processor”。

### 第六步：理解状态层

读：

- [internal/store/redis.go](/E:/github/Memory_chunk/internal/store/redis.go)
- [internal/summary/worker.go](/E:/github/Memory_chunk/internal/summary/worker.go)

你要理解：

- page 是怎么保存的
- summary 是怎么保存的
- Stream 消费组是怎么接入的
- 为什么 `PageIn` 要优先读 summary

## 4. 一条推荐的 30 分钟速读路径

如果你只有 30 分钟，建议按下面顺序：

1. [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
2. [cmd/main.go](/E:/github/Memory_chunk/cmd/main.go)
3. [internal/server/refiner.go](/E:/github/Memory_chunk/internal/server/refiner.go)
4. [internal/engine/pipeline.go](/E:/github/Memory_chunk/internal/engine/pipeline.go)
5. [internal/processor/paging.go](/E:/github/Memory_chunk/internal/processor/paging.go)
6. [internal/store/redis.go](/E:/github/Memory_chunk/internal/store/redis.go)

这样至少能理解主干。

## 5. 一条推荐的 2 小时深入路径

1. 阅读 [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
2. 阅读 [docs/principles-and-internals.md](/E:/github/Memory_chunk/docs/principles-and-internals.md)
3. 逐个过 `api -> server -> engine -> processor -> store -> summary`
4. 手动画一条 `Refine` 主链
5. 手动画一条 `PageIn` 主链
6. 对照 [docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md) 看哪些是已完成、哪些是未来计划

## 6. 学这个项目时最容易卡住的点

### 6.1 误以为 `RAGChunk` 还是纯字符串

实际上当前代码已经把它升级成了结构化片段模型。

### 6.2 误以为 `auto_compact_async` 只是占位

当前它已经能真实入队，worker 也已经能真实消费，只是摘要策略还是启发式规则。

### 6.3 误以为 audit 只是日志

实际上 `StepAudit` 和 `StepSemanticAudit` 是这个项目的重要价值点之一，因为它们让 lossy 压缩变得可解释。

### 6.4 误以为“编译通过”就等于“工程成熟”

当前代码能编译，但没有测试和观测，所以不要把当前阶段高估成生产完成品。

## 7. 建议边读边回答的 8 个问题

1. 为什么一定要统一 Token 口径，而不是到处 `len(text)`？
2. 为什么 Processor 要有 capability，而不是只靠步骤名？
3. 为什么 `paging` 必须按 Token，而不是按字符或按行？
4. 为什么 `collapse` 要保留 `sources`？
5. 为什么 `snip` 只对某些 fragment type 生效？
6. 为什么 `auto_compact_sync` 和 `auto_compact_async` 要拆开？
7. 为什么 `PageIn` 要优先返回 summary？
8. 为什么当前最优先工作不是继续堆更多处理器？

如果你能回答清楚这 8 个问题，就说明你已经掌握了项目主干。

## 8. 建议的动手练习

### 练习 1：手动跟一遍 `Refine`

目标：

- 画出请求进入 pipeline 到返回响应的完整流程

### 练习 2：加一个 Processor 占位实现

目标：

- 学会 Processor 扩展路径

建议：

- 先加一个非常简单的 `log_dedup` 雏形

### 练习 3：给 `summary` 抽一个 Provider 接口草图

目标：

- 理解当前启发式逻辑未来应该怎么演进

### 练习 4：写第一批测试草案

目标：

- 理解项目当前最真实的缺口

## 9. 如果你要继续深入

建议接着看：

- [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)
- [docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)

前者回答“现在代码怎么组织”，后者回答“接下来最值得做什么”。

## 10. 结论

学习这个项目的关键不是死记包名，而是建立一条稳定的认知主线：

`上下文治理目标 -> 协议边界 -> pipeline 调度 -> processor 分工 -> 状态层 -> 异步摘要 -> 审计 -> 后续演进`

只要这条主线建立起来，后续读代码和改代码都会快很多。
