# Context Refiner 设计与规划文档

## 1. 文档目的

本文档用于系统化记录本项目的：

- 项目目标与业务价值
- 当前架构设计与模块职责
- 当前代码实现状态
- 当前阶段计划
- 后续阶段路线图与扩展方向

本文档既服务于开发实现，也服务于后续项目汇报、架构评审与团队协作。

## 2. 项目定位

### 2.1 项目目标

本项目的核心目标，是构建一个位于：

- AI 应用层
- 大模型 API 层

之间的上下文清洗与压缩服务。

该服务负责在不明显破坏语义一致性的前提下，对输入给大模型的上下文进行结构化裁剪、压缩、分页与缓存对齐，从而减少无效 Token 消耗，并提高缓存复用效率。

### 2.2 核心价值

项目不是普通的字符串压缩器，而是一个面向 LLM 推理链路的上下文治理层，主要价值包括：

- 减少长上下文推理成本
- 提升提示词缓存命中率
- 控制多轮对话上下文膨胀
- 管理大规模 RAG 检索结果的上下文占用
- 管理 Agent 工具调用返回结果的污染问题
- 为后续异步摘要、上下文回填、证据保真审计提供基础设施

### 2.3 KPI

项目面向的核心 KPI：

- Token 节省率：`30% ~ 80%`
- 缓存命中率提升
- 推理延迟降低
- 在压缩后尽量维持语义可用性与证据可追踪性

### 2.4 适用场景

- 多轮长对话
- RAG 场景下的大量检索片段拼接
- Agent 工具调用返回内容过长
- 代码、日志、JSON、表格等高 Token 密度内容

## 3. 设计原则

### 3.1 核心原则

项目当前遵循以下设计原则：

- `Pipeline-Processor`：每个裁剪步骤是独立处理器
- 策略驱动：处理器执行顺序由策略配置决定
- Token 口径统一：输入、处理中、输出统一使用真实 tokenizer
- 结构感知：RAG 数据不是纯字符串，而是结构化片段
- 可解释：每步处理都要产生审计信息
- 可分页：长内容支持 page-out / page-in
- 可异步扩展：摘要等重型任务允许异步排队

### 3.2 当前架构升级点

相较于最初的简单字符串处理版本，当前设计已经做了几个关键升级：

- 从纯文本 chunk 升级为结构化 fragment chunk
- 从黑盒 Processor 升级为带能力声明的 Processor
- 从字符串长度分页升级为真实 token 分页
- 从同步摘要占位升级为同步安全压缩加异步摘要排队
- 从简单步骤日志升级为语义保真审计

## 4. 总体架构

### 4.1 逻辑架构

系统当前逻辑上可分为六层：

1. Ingress 层
2. Protocol / Mapping 层
3. Pipeline Engine 层
4. Processor 层
5. State / Queue 层
6. Egress 层

### 4.2 各层职责

#### 4.2.1 Ingress 层

职责：

- 接收 gRPC 请求
- 读取 `messages / rag_chunks / model / token_budget / policy / session_id / request_id`
- 完成基本参数校验

当前实现位置：

- `cmd/main.go`
- `internal/server/refiner.go`
- `api/refiner.proto`

#### 4.2.2 Protocol / Mapping 层

职责：

- 将 Protobuf 请求映射为内部领域模型
- 为未传入的 `request_id` 自动生成值
- 处理 `source -> sources`、`content -> fragments` 的向后兼容逻辑

当前实现位置：

- `internal/server/refiner.go`

#### 4.2.3 Pipeline Engine 层

职责：

- 根据策略顺序运行 Processor
- 在每一步之前判断是否应跳过处理器
- 统一统计输入 token、输出 token
- 收集步骤级审计与语义保真审计

当前实现位置：

- `internal/engine/pipeline.go`
- `internal/engine/registry.go`

#### 4.2.4 Processor 层

职责：

- 对上下文执行单一职责的清洗或压缩动作
- 返回新的请求对象与当前步骤审计信息

当前处理器包括：

- `paging`
- `collapse`
- `compact`
- `snip`
- `auto_compact_sync`
- `auto_compact_async`
- `assemble`

#### 4.2.5 State / Queue 层

职责：

- 将超长内容分页写入 Redis
- 从 Redis 读取分页内容
- 向 Redis Stream 写入异步摘要任务

当前实现位置：

- `internal/store/redis.go`

#### 4.2.6 Egress 层

职责：

- 输出最终压缩后的 prompt
- 返回步骤审计
- 返回分页引用
- 返回异步摘要 job id

当前实现位置：

- `internal/server/refiner.go`

## 5. 包结构设计

当前仓库结构如下：

```text
context-refiner/
├── api/
│   ├── refiner.proto
│   └── refinerv1/
├── cmd/
│   └── main.go
├── config/
│   ├── service.yaml
│   └── policies.yaml
├── docs/
│   └── context-refiner-design.md
├── internal/
│   ├── config/
│   ├── engine/
│   ├── processor/
│   ├── server/
│   ├── store/
│   └── tokenizer/
└── pkg/
    └── client/
```

### 5.1 `api/`

职责：

- 定义 gRPC 协议
- 作为服务间边界契约

当前协议已支持：

- 结构化 `RagFragment`
- `session_id`
- `request_id`
- Processor 能力声明
- 语义保真审计
- `pending_summary_job_ids`

### 5.2 `cmd/`

职责：

- 读取配置
- 初始化 tokenizer、Redis、registry、gRPC server
- 启动服务

### 5.3 `internal/engine/`

职责：

- 管线驱动与调度
- Processor 注册与解析
- 请求、响应、审计等领域模型

### 5.4 `internal/processor/`

职责：

- 实现具体裁剪步骤
- 保持每个处理器职责单一

### 5.5 `internal/store/`

职责：

- Redis PageStore
- Redis Stream SummaryJobQueue

### 5.6 `internal/tokenizer/`

职责：

- 封装 `tiktoken-go`
- 提供统一的 Token 计算入口

### 5.7 `internal/server/`

职责：

- gRPC handler
- 请求与内部模型转换
- 响应与 Protobuf 模型转换

### 5.8 `config/`

职责：

- 存放服务配置
- 存放策略配置

## 6. 核心领域模型

### 6.1 Message

表示对话消息：

- `role`
- `content`

主要来源：

- system prompt
- user query
- assistant 历史回答

### 6.2 RAGFragment

这是当前上下文清洗层的关键升级点。

RAG 数据不再只被视为一段无结构字符串，而是被拆成结构化片段：

- `title`
- `body`
- `code`
- `table`
- `json`
- `tool-output`
- `log`
- `error-stack`

这样做的意义：

- 不同内容类型可以应用不同压缩策略
- `snip` 可以只对 code、log、tool-output 生效
- `compact` 可以安全处理 log、tool-output 的冗余
- 后续可以针对 `json / table / code` 分别扩展结构化处理器

### 6.3 RAGChunk

一个 RAG Chunk 当前包含：

- `id`
- `source`
- `sources`
- `fragments`
- `page_refs`

设计含义：

- `source` 保留向后兼容
- `sources` 表示折叠后的多来源证据集合
- `fragments` 表示结构化正文
- `page_refs` 表示 page-out 后的 Redis 引用

### 6.4 RefineRequest

当前内部请求模型包含：

- `session_id`
- `request_id`
- `messages`
- `rag_chunks`
- `model`
- `budget`
- `policy`
- `runtime_policy`
- `current_tokens`
- `input_tokens`
- `metadata`
- `pending_summary_job_ids`

### 6.5 StepAudit

每个 Processor 当前都会输出：

- `before_tokens`
- `after_tokens`
- `duration_ms`
- `details`
- `capabilities`
- `semantic`

### 6.6 StepSemanticAudit

这是为了防止只追求压缩率而损失语义而设计的。

当前语义审计字段包括：

- `removed`
- `retained`
- `reasons`
- `source_preserved`
- `code_fence_preserved`
- `error_stack_preserved`
- `dropped_citations`

## 7. Processor 能力声明模型

### 7.1 设计动机

早期版本的问题是：

- 哪些 Processor 属于激进压缩，写死在引擎里
- 哪些 Processor 需要结构化输入，没有显式声明
- 哪些 Processor 是 lossy，没有被统一表达

这会导致：

- 引擎与具体步骤强耦合
- 扩展新步骤时必须改引擎逻辑
- 难以通过策略编排做智能跳过

### 7.2 当前能力字段

每个 Processor 现在声明：

- `Aggressive`
- `Lossy`
- `StructuredInputOnly`
- `MinTriggerTokens`
- `PreserveCitation`

### 7.3 当前引擎的跳过规则

引擎当前会根据能力声明做跳过判断：

- 当前 token 已低于 budget 时，跳过 `Aggressive=true` 的 Processor
- 当前 token 未达到触发门槛时，跳过 `MinTriggerTokens`
- 请求不具备结构化 chunk 时，跳过 `StructuredInputOnly`

这使后续扩展更稳：

- 新增 Processor 不再需要改步骤名白名单
- 策略控制与处理器能力逐渐解耦

## 8. 当前处理器链路设计

### 8.1 `paging`

职责：

- 将超长 Chunk 做 page-out
- 只保留第一页 fragments 在当前 prompt 中
- 其余页写入 Redis

当前特点：

- 使用真实 token 计数判断是否超阈值
- 使用作用域 key：`session + request + chunk + hash + page index`
- 保证分页维度按 token，而不是按字符

当前价值：

- 减少一次请求中大 Tool Output 或长日志的上下文污染
- 为后续 page-in 提供基础

### 8.2 `collapse`

职责：

- 对重复 chunk 做折叠
- 合并多来源引用

当前特点：

- 保留 `sources`
- 不再简单删除重复内容而丢失证据来源

### 8.3 `compact`

职责：

- 执行安全微压缩
- 移除多余空行和冗余空白

当前特点：

- 不做语义级删除
- 不改变引用与结构边界

### 8.4 `snip`

职责：

- 对代码、工具输出、日志、错误栈、JSON 这类高密度内容做 middle-out 截断

当前特点：

- 保留 head 和 tail
- 中间区域以占位标记替代
- 不对普通 body 文本随意 snip

### 8.5 `auto_compact_sync`

职责：

- 做同步安全压缩
- 适用于 tool-output、log、error-stack

当前特点：

- 去除重复行
- 合并多余空行
- 不进行模型摘要
- 属于低风险、低延迟的清洗步骤

### 8.6 `auto_compact_async`

职责：

- 将重型摘要任务异步排队

当前特点：

- 当上下文依然很大时，将较大的 chunk 投递到 Redis Stream
- 记录 `pending_summary_job_ids`
- 当前阶段仅实现生产者，尚未实现消费者

### 8.7 `assemble`

职责：

- 按统一格式组装最终 prompt

特点：

- 统一 messages 与 rag 的渲染格式
- 统一输出 token 计算口径

## 9. 请求处理数据流

### 9.1 主请求流

一次 Refine 请求的当前流程如下：

1. 客户端发送 `RefineRequest`
2. gRPC Server 接收请求
3. 请求被映射为内部 `RefineRequest`
4. 若缺少 `request_id`，服务自动生成
5. 根据策略装配 Processor 列表
6. Pipeline 计算真实 `input_tokens`
7. 按顺序执行各 Processor
8. 每一步收集 Token 审计和语义审计
9. `assemble` 输出最终 prompt
10. Server 将内部响应映射回 `RefineResponse`

### 9.2 Page-Out / Page-In 流

当前分页流如下：

1. `paging` 检测某个 chunk 超过阈值
2. 按 token 分页
3. 将分页写入 Redis
4. 当前 prompt 中只保留第一页
5. 返回 `page_refs`
6. 后续调用 `PageIn` 接口时，按 `page_keys` 拉回内容

### 9.3 异步摘要流

当前已实现的是异步摘要闭环：

1. `auto_compact_async` 判断 chunk 过大
2. 构造 `SummaryJob`
3. 写入 Redis Stream
4. 后台 worker 消费 Redis Stream
5. worker 生成结构化摘要
6. 将摘要结果写回 Redis summary key
7. `PageIn` 优先返回摘要结果
8. 响应中返回 `pending_summary_job_ids`

## 10. Redis 设计

### 10.1 当前承担的职责

Redis 当前承担两种职责：

- PageStore
- SummaryJobQueue

### 10.2 PageStore

存储方式：

- `SET key value EX ttl`

key 结构：

```text
session:{session_id}:request:{request_id}:chunk:{chunk_id}:hash:{content_hash}:page:{page_index}
```

这样设计的意义：

- 防止不同请求的 chunk key 冲突
- 同一 chunk 在内容变化后可通过 `content_hash` 区分版本
- 便于按 session、request 排查问题

### 10.3 SummaryJobQueue

当前用 Redis Stream 写入摘要任务，字段包括：

- `job_id`
- `request_id`
- `chunk_id`
- `payload`

任务正文包含：

- `session_id`
- `request_id`
- `policy`
- `chunk_id`
- `source`
- `content_hash`
- `page_refs`
- `content`
- `target_tokens`
- `current_tokens`
- `created_at`

## 11. gRPC / Protobuf 设计

### 11.1 当前接口

当前暴露两个 RPC：

- `Refine`
- `PageIn`

### 11.2 `Refine`

输入：

- messages
- rag_chunks
- model
- token_budget
- policy
- session_id
- request_id

输出：

- optimized_prompt
- input_tokens
- output_tokens
- budget_met
- audits
- paged_chunks
- metadata
- pending_summary_job_ids

### 11.3 `PageIn`

输入：

- `page_keys`

输出：

- 对应的分页内容

### 11.4 协议层设计优点

- 支持结构化 RAG 输入
- 支持会话作用域
- 支持步骤级调试
- 支持异步摘要的后续扩展

## 12. 配置设计

### 12.1 服务配置

当前 `config/service.yaml` 管理：

- gRPC 监听地址
- Redis 地址与认证
- Page TTL
- Summary Stream
- tokenizer 模型
- 默认策略
- 分页阈值

### 12.2 策略配置

当前 `config/policies.yaml` 管理：

- 策略名
- `budget_ratio`
- 执行步骤顺序
- snip 参数
- auto compact 阈值

策略样例：

- `strict_coding_assistant`
- `creative_chat`

## 13. 当前已完成事项

截至当前版本，已完成内容如下：

### 13.1 基础服务能力

- Go 项目骨架
- gRPC 服务端
- protobuf 生成代码
- Redis 接入
- `tiktoken-go` 接入

### 13.2 上下文治理能力

- 真实 token 统一计数
- 结构化 RAG 输入模型
- Processor 能力声明
- 策略驱动执行链
- 分页存储与 page-in
- 双级 auto compact
- 语义保真审计

### 13.3 当前可运行状态

当前代码已经可以：

- 正常编译
- 读取配置
- 初始化真实组件
- 因空地址配置而在启动前校验失败

这符合当前先不填具体地址的要求。

## 14. 当前阶段计划

当前阶段可以定义为：

### Phase 1：核心底盘建设

该阶段目标是把上下文清洗服务的主干能力搭起来。

当前阶段已基本完成的内容：

- 统一 token 口径
- 管线与 Processor 扩展机制
- 结构化 chunk 数据模型
- Redis 分页
- gRPC 服务协议
- 基础审计能力

当前阶段仍在延续的工作重点：

- 补充更高质量的异步摘要闭环
- 补齐更多结构化片段的细分处理策略
- 增加观测性指标和压缩评测体系

## 15. 将来计划

后续建议分三个阶段推进。

### 15.1 Phase 2：异步摘要闭环

目标：

- 把当前 Redis Stream 中的摘要任务真正消费起来

建议实现：

- 增加 summary worker
- 对接真实摘要模型
- 摘要结果写回 Redis 或专用摘要存储
- 在 page-in 阶段支持摘要优先回填

价值：

- 把当前异步排队变成真正的异步压缩能力

### 15.2 Phase 3：观测与评测体系

目标：

- 让压缩效果可量化、可回归、可调优

建议实现：

- Prometheus 指标
- Trace / Span
- 每步压缩前后 token 统计
- 语义损失抽样评测
- 缓存命中率评测

建议指标：

- `refiner_input_tokens_total`
- `refiner_output_tokens_total`
- `refiner_step_latency_seconds`
- `refiner_pageout_total`
- `refiner_summary_jobs_total`
- `refiner_budget_met_total`

### 15.3 Phase 4：更细粒度的结构化压缩

目标：

- 让不同类型内容走不同压缩策略

建议扩展处理器：

- `json_trim`
- `table_reduce`
- `code_outline`
- `log_dedup`
- `error_stack_focus`
- `rag_rerank_trim`

### 15.4 Phase 5：缓存与多级上下文治理

目标：

- 进一步提升缓存命中率和跨请求复用能力

建议扩展：

- 基于 `content_hash` 的跨请求复用
- 摘要缓存
- 热点上下文缓存
- 会话级上下文快照

## 16. 风险与注意事项

### 16.1 当前主要风险

- 当前摘要 worker 仍然使用启发式规则，而不是真实外部摘要模型
- 当前 page-in 已支持摘要优先回填，但回填粒度仍是 page 级，不是更强的 chunk 级对象
- 目前缺少系统级观测指标
- 目前尚未建立自动化评测体系

### 16.2 设计上要持续坚持的原则

- Token 口径必须统一，不能再引入估算值
- 所有 lossy 压缩都必须带语义审计
- 任何新 Processor 都应声明能力而不是写死在引擎里
- 分页 key 必须包含作用域
- 结构化 chunk 模型不能再退回纯字符串主导

## 17. 推荐开发顺序

如果继续推进，我建议按以下顺序做：

1. 异步摘要消费者
2. Redis 中摘要结果存储设计
3. PageIn 与摘要回填逻辑
4. Prometheus 与 Tracing
5. 压缩效果评测工具
6. JSON、Table、Code 专项压缩处理器

## 18. 当前结论

当前项目已经从原型想法进入可持续演进的服务骨架阶段。

现阶段最重要的事情已经不是继续堆更多字符串裁剪逻辑，而是围绕以下三条主线稳定推进：

- 把异步摘要能力闭环
- 把观测与评测补齐
- 把结构化内容处理能力做深

只要这三条主线推进正确，这个项目就有机会真正成为 AI 应用层与模型 API 层之间的上下文治理基础设施，而不只是一个临时的压缩脚本。
