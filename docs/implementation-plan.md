# Context Refiner 实施方案与阶段计划

## 1. 文档目的

这份文档不再重复总体设计，而是从“怎么落地”的角度，系统说明以下内容：

- 当前代码已经做到什么程度
- 接下来每个阶段具体做什么
- 每个阶段需要改哪些模块
- 每个阶段的输入、输出、依赖与验收标准
- 哪些是必须先做的底层能力，哪些是可并行扩展项

可配合以下文档一起使用：

- 总体设计：[docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
- 待办清单：[docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md)

## 2. 当前阶段结论

项目已经从“概念设计”进入“可持续演进的服务骨架”阶段，当前不是继续堆更多简单压缩规则，而是进入以下三条主线的工程化阶段：

1. 把异步摘要从启发式闭环推进到真实模型闭环。
2. 把观测、评测、回归能力补齐，确保 KPI 有可信口径。
3. 把结构化片段的差异化处理策略继续做深，形成稳定策略层。

### 2.1 当前已落地能力

- 统一真实 Token 计数：入口、处理中、输出全部通过真实 tokenizer 计数。
- 策略驱动管线：引擎按策略顺序调度处理器，并可根据能力声明跳过步骤。
- 结构化 RAG 片段：已支持 `title`、`body`、`code`、`table`、`json`、`tool-output`、`log`、`error-stack`。
- 分页与回填：长内容可 page-out 到 Redis，后续可通过 `PageIn` 读取。
- 异步摘要闭环：`auto_compact_async` 已能把任务写入 Redis Stream，worker 消费后写回摘要，`PageIn` 优先返回摘要结果。
- 基础细分处理器：已具备 `json_trim`、`table_reduce`、`code_outline`、`error_stack_focus`。
- 可集成服务形态：gRPC 服务端、protobuf 协议、Redis 状态层、配置加载均已接通。

### 2.2 当前明确约束

- `config/service.yaml` 中真实地址仍保留为空，这是当前阶段的有意占位，而不是遗漏。
- 当前摘要 worker 仍是启发式规则，不是真实外部摘要模型。
- 当前摘要结果优先面向 page 级回填，尚未升级到 chunk 级上下文对象。
- 当前还缺少 Prometheus、Tracing、自动化评测、集成测试这些“工程化兜底”能力。

## 3. 当前系统边界与模块职责

### 3.1 服务边界

当前系统是一个独立的上下文治理服务，位于应用层和模型 API 之间，负责：

- 接收原始消息、多轮对话、RAG 数据、模型配置和 Token 预算
- 执行裁剪、折叠、分页、压缩、摘要排队、组装
- 输出更纯净、更短、更适合缓存复用的上下文
- 记录审计信息，方便后续做效果评测与回归分析

### 3.2 关键模块现状

`cmd/main.go`
- 负责启动 tokenizer、Redis、processor registry、gRPC server、summary worker。

`internal/engine/`
- 已具备管线驱动、能力声明跳过逻辑、真实 token 口径、审计输出。

`internal/processor/`
- 已具备分页、折叠、微压缩、智能打洞、同步压缩、异步摘要排队、组装，以及部分结构化细分处理器。

`internal/store/redis.go`
- 同时承担 PageStore 与 SummaryJobQueue。
- 已具备 page 存储、summary 存储、stream 入队、消费组消费、ack。

`internal/summary/worker.go`
- 已具备 Redis Stream 消费、启发式摘要、写回 summary key 的闭环。

`internal/server/refiner.go`
- 已具备 `Refine` 与 `PageIn` 两个服务接口。
- `PageIn` 已优先使用摘要结果回填。

`config/`
- 已具备服务配置与策略配置。
- 当前真实监听地址与 Redis 地址仍为空。

## 4. 实施原则

后续落地必须坚持以下原则，否则很容易出现“压缩率很好看，但实际效果变差”的偏航：

### 4.1 Token 口径必须唯一

- 输入 token、处理后 token、输出 token、预算达标判断，全部必须基于真实 tokenizer。
- 任何新模块都不能重新引入字符长度估算或 rune 粗估。

### 4.2 Lossy 压缩必须带审计

- 只要一个步骤会删信息、改结构、替换中段，就必须留下“删了什么、保留了什么、为什么删”的语义审计。

### 4.3 处理器扩展不能反向污染引擎

- 新增处理器应优先通过能力声明接入，不应继续把跳过逻辑硬编码回引擎。

### 4.4 结构化输入优先

- 后续所有高级处理器优先围绕 `fragment` 类型工作，而不是再次退回纯字符串处理。

### 4.5 异步能力必须可降级

- 外部摘要模型、异步队列、远端依赖都必须具备失败时的降级路径。
- 主链请求不能因为异步摘要失败而不可用。

## 5. 分阶段实施路线

## Phase 1：核心底盘建设

### 目标

把服务骨架、真实 token、Redis 分页、gRPC 协议、基础处理链路先做通。

### 当前状态

这一阶段已经基本完成。

### 已交付结果

- 真实 tokenizer 接入
- 处理器能力声明机制
- 结构化 RAG 片段模型
- Redis page-out / page-in
- gRPC 服务端与 protobuf
- 同步压缩 + 异步摘要排队
- 语义保真审计

### 当前残余问题

- 仍缺乏系统级观测与评测能力
- 异步摘要质量仍取决于启发式规则

## Phase 2：异步摘要产品化

### 目标

把当前“启发式闭环”升级为“可接入真实外部摘要模型、可管理生命周期、可回填、可失效”的正式能力。

### 要做的事情

1. 抽象摘要提供方接口
2. 支持外部模型调用的同步/异步封装
3. 为摘要结果设计更稳定的存储结构
4. 将 page 级摘要提升为 chunk 级摘要对象
5. 增加失效策略和重试策略

### 建议实现拆分

#### Phase 2.1：摘要 Provider 抽象

新增统一接口，例如：

```go
type SummaryProvider interface {
    Name() string
    Summarize(ctx context.Context, job SummaryJob) (SummaryArtifact, error)
}
```

需要支持两类实现：

- `HeuristicProvider`
- `LLMProvider`

作用：

- 当前 worker 不直接绑定具体摘要逻辑
- 便于本地开发、测试、线上生产使用不同 provider

#### Phase 2.2：摘要对象升级

不要只把摘要当成字符串写回 Redis，建议升级为结构化对象：

```go
type SummaryArtifact struct {
    ArtifactID      string
    SessionID       string
    RequestID       string
    ChunkID         string
    ContentHash     string
    Level           string
    SummaryText     string
    SourcePreserved bool
    FragmentTypes   []string
    Model           string
    CreatedAt       time.Time
    ExpiresAt       time.Time
}
```

这样做的价值：

- 后续可区分 page 级、chunk 级、session 级摘要
- 后续能做命中率分析、失效回收、来源追踪

#### Phase 2.3：失效与重建策略

建议最少包含以下规则：

- `content_hash` 变化后，旧摘要不再直接复用
- Redis TTL 到期后自动淘汰
- 当 provider 升级或摘要策略变化时，可通过版本字段触发重建

### 交付物

- `summary/provider` 抽象层
- 外部摘要模型适配器
- 摘要结果结构化存储
- `PageIn` / 后续回填逻辑支持 chunk 级摘要
- 摘要失败重试与降级策略

### 验收标准

- 外部摘要模型故障时，主链 `Refine` 仍可正常返回
- 同一个 `content_hash` 在有效期内可稳定复用摘要
- `PageIn` 可区分原始 page 与摘要回填对象
- 摘要结果包含来源与片段类型元数据

## Phase 3：观测与评测体系

### 目标

建立能支撑 KPI 和线上调优的观测闭环，而不是只靠人工看日志。

### 核心内容

#### 3.1 Prometheus 指标

优先级最高的一批指标建议如下：

- `refiner_input_tokens_total`
- `refiner_output_tokens_total`
- `refiner_step_latency_seconds`
- `refiner_pageout_total`
- `refiner_summary_jobs_total`
- `refiner_budget_met_total`

建议标签控制在低基数范围：

- `policy`
- `processor`
- `fragment_type`
- `summary_stage`

不要引入以下高基数标签：

- `session_id`
- `request_id`
- `chunk_id`

#### 3.2 Tracing

建议后续接 OpenTelemetry，至少覆盖：

- gRPC `Refine`
- gRPC `PageIn`
- Redis page write/read
- Redis Stream enqueue/consume/ack
- summary worker provider 调用

建议关键 span attribute：

- `policy`
- `budget`
- `input_tokens`
- `output_tokens`
- `chunk_count`
- `paged_chunk_count`
- `summary_job_count`

#### 3.3 压缩效果评测

要增加离线评测工具，而不是只看线上指标。

建议评测维度：

- 压缩前后 token 比例
- budget 达标率
- page-out 比例
- 引文保留率
- code fence 保留率
- error stack 保留率
- 摘要回填命中率

### 交付物

- 指标包与 `/metrics` 暴露
- Trace 初始化与 span 注入
- 一组标准评测样本
- 评测脚本或评测命令

### 验收标准

- 能按策略观察 token 输入输出差异
- 能定位某个 processor 是否成为延迟热点
- 能定位 page-out 和 summary job 的触发情况
- 能在回归测试中比较两版策略的压缩效果差异

## Phase 4：结构化片段策略深化

### 目标

让不同类型的上下文使用不同压缩策略，形成真正“结构感知”的治理层。

### 已完成

- `json_trim`
- `table_reduce`
- `code_outline`
- `error_stack_focus`

### 下一批建议处理器

#### `log_dedup`

目标：
- 删除高频重复日志行
- 保留首条、尾条、异常条、时间跨度边界

适用片段：
- `log`
- `tool-output`

#### `tool_output_focus`

目标：
- 聚焦工具输出中的结果摘要、错误、警告、最终状态
- 去掉大量无意义进度回显

适用片段：
- `tool-output`

#### `rag_rerank_trim`

目标：
- 对过多的 RAG chunk 做轻量再排序和裁剪
- 优先保留来源更强、内容更贴近 query 的片段

依赖：
- 需要引入 query-aware 的排序信号或打分接口

### 交付物

- 新处理器实现
- 策略配置扩展
- 语义审计扩展
- 样本回归测试

### 验收标准

- 同类冗余日志明显下降
- 工具输出的结果段保留更完整
- 结构化处理后不明显损坏 code fence / 引文 / 错误栈

## Phase 5：缓存复用与多级上下文治理

### 目标

把“单次请求压缩”升级为“多请求复用的上下文治理”。

### 关键方向

#### 5.1 基于 `content_hash` 的跨请求复用

让同一内容在不同请求中不必重复分页、重复摘要。

#### 5.2 多级摘要缓存

建议逐步演进为：

- page 级摘要
- chunk 级摘要
- session 级摘要快照

#### 5.3 热点上下文缓存

对高频片段建立更长生命周期的缓存层，提高命中率。

### 交付物

- 统一缓存索引设计
- 版本/失效/淘汰策略
- 跨请求复用逻辑
- 缓存命中率指标

### 验收标准

- 对重复内容可显著减少重复压缩与重复摘要
- 不同请求之间不会因作用域污染发生错读

## 6. 推荐执行顺序

如果按工程收益和依赖关系排序，建议使用以下顺序：

1. Prometheus 指标
2. 异步摘要 worker 集成测试
3. 摘要 Provider 抽象
4. 外部摘要模型对接
5. chunk 级摘要对象与回填
6. 失效策略与缓存索引
7. Tracing
8. `log_dedup`
9. `tool_output_focus`
10. `rag_rerank_trim`
11. 跨请求 `content_hash` 复用
12. 压缩效果评测工具

这个顺序的原因是：

- 先补观测，后补复杂能力，避免“做了但无法判断效果”
- 先把摘要链路抽象化，后接真实模型，降低返工
- 先做无争议的日志/工具输出处理，再做需要 query-aware 信号的 rerank

## 7. 具体实施主链

为了避免后续实施时出现“只做功能、不做闭环”的问题，建议每轮迭代都按下面主链推进：

1. 明确本轮只做一个中心能力
2. 列出会影响的模块与接口边界
3. 先补配置模型和领域模型
4. 再补处理链路或后台 worker
5. 同步补 protobuf / server 映射
6. 同步补审计字段或指标字段
7. 做构建验证和关键路径测试
8. 更新 `todolist`
9. 更新实施文档
10. 记录残余风险和下一轮输入

## 8. 质量与验收计划

### 8.1 单元测试

优先覆盖：

- tokenizer 真实计数
- paging 按 token 分页
- processor 跳过逻辑
- 结构化片段细分处理器
- summary worker 的摘要生成与写回逻辑

### 8.2 集成测试

优先覆盖：

- gRPC `Refine`
- Redis page-out / page-in
- Redis Stream enqueue / consume / ack
- `PageIn` 摘要优先返回

### 8.3 回归验证

每次处理器策略变化后都应该回归以下样本：

- 多轮长对话
- 大段代码块
- 长日志
- JSON 返回
- 表格文本
- 错误堆栈
- 工具输出

### 8.4 线上前检查

- 配置项是否齐全
- 地址是否已填
- TTL 是否合理
- consumer group 是否正确
- 指标是否可抓取
- tracing 是否可导出

## 9. 风险与应对

### 风险 1：只追压缩率，损失语义质量

应对：

- 所有 lossy 步骤保留语义审计
- 增加离线评测和关键样本回归

### 风险 2：摘要质量不可控

应对：

- 抽象 provider，保留启发式降级路径
- 为摘要结果增加版本和来源元数据

### 风险 3：缓存复用带来脏读

应对：

- 作用域 key 与 `content_hash` 同时存在
- 跨请求复用必须明确隔离索引和内容版本

### 风险 4：观测缺失导致 KPI 不可信

应对：

- 先补 Prometheus，再做更复杂的策略优化
- 所有关键链路都要能看见 token、延迟、命中率

## 10. 当前建议结论

当前最合适的推进方式不是继续直接叠加新处理器，而是先把文档、观测、测试和摘要抽象层定稳，然后再继续扩展复杂能力。

一句话总结当前建议：

先把“怎么观察、怎么验证、怎么抽象摘要”补齐，再把“真实模型摘要、chunk 级回填、跨请求复用”往前推。
