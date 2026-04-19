# Context Refiner 代码设计说明

- 文档版本：`v2026.04.19`
- 更新日期：`2026-04-19`
- 文档类型：`Code Reference`
- 适用代码基线：`main`

> 本文档描述当前已经落地的代码结构与职责分配。
> 如果你先想理解“为什么这样分层”，请先看 [docs/layered-architecture.md](/E:/github/Memory_chunk/docs/layered-architecture.md)。

## 1. 文档目标

本文档主要回答四个问题：

1. 现在仓库是怎么分层的。
2. `Refine` / `PageIn` 的代码主链是怎么流动的。
3. 哪些包是公开 API，哪些包是内部实现。
4. 未来扩展应该落在哪一层。

## 2. 当前仓库结构

```text
api/
  refiner.proto
  refinerv1/

cmd/
  refiner/
    main.go

config/
  service.yaml
  policies.yaml

docs/

internal/
  bootstrap/
  controller/
    grpc/
  domain/
    core/
      components/
      processor/
      repository/
  dto/
  infra/
    config/
  mapper/
  observability/
    metrics/
    tracing/
  service/
  support/
    heuristic/
    redis/
    summary/
    tempo/
    tokenizer/

pkg/
  client/
  service/
```

## 3. 分层职责

### 3.1 `api/`

职责：

- 定义对外 protobuf / gRPC contract
- 固化跨进程接口边界

关键文件：

- [api/refiner.proto](/E:/github/Memory_chunk/api/refiner.proto)

### 3.2 `pkg/service/`

职责：

- 定义公开的 in-process service API
- 让“服务能力”独立于具体传输协议

关键文件：

- [pkg/service/refiner.go](/E:/github/Memory_chunk/pkg/service/refiner.go)

### 3.3 `pkg/client/`

职责：

- 提供基于 `pkg/service.RefinerService` 的薄 client wrapper
- 方便在同进程内以统一方式调用服务

关键文件：

- [pkg/client/client.go](/E:/github/Memory_chunk/pkg/client/client.go)

### 3.4 `cmd/refiner/`

职责：

- 作为极薄启动入口
- 负责监听地址、生命周期和优雅退出
- 不承载业务编排细节

关键文件：

- [cmd/refiner/main.go](/E:/github/Memory_chunk/cmd/refiner/main.go)

### 3.5 `internal/bootstrap/`

职责：

- 装配 `infra/config -> support -> domain -> service -> controller`
- 构建 registry、page store、gRPC server、summary worker
- 把启动时依赖集中在一层，避免散落到 `main`

关键文件：

- [internal/bootstrap/runtime.go](/E:/github/Memory_chunk/internal/bootstrap/runtime.go)
- [internal/bootstrap/processors.go](/E:/github/Memory_chunk/internal/bootstrap/processors.go)

### 3.6 `internal/controller/grpc/`

职责：

- 实现 gRPC handler
- 把 RPC 请求委托给 `pkg/service.RefinerService`
- 自身不承载业务逻辑

关键文件：

- [internal/controller/grpc/refine_controller.go](/E:/github/Memory_chunk/internal/controller/grpc/refine_controller.go)

### 3.7 `internal/service/`

职责：

- 作为应用服务层主入口
- 负责 policy 解析、request mapping、pipeline 调用、response mapping
- 组织 `Refine` / `PageIn` 这两个用例

关键文件：

- [internal/service/refine_service.go](/E:/github/Memory_chunk/internal/service/refine_service.go)
- [internal/mapper/refine_request_mapper.go](/E:/github/Memory_chunk/internal/mapper/refine_request_mapper.go)
- [internal/mapper/refine_response_mapper.go](/E:/github/Memory_chunk/internal/mapper/refine_response_mapper.go)
- [internal/mapper/mapper_helper.go](/E:/github/Memory_chunk/internal/mapper/mapper_helper.go)

### 3.8 `internal/domain/core/`

职责：

- 定义核心模型与抽象
- 定义 repository contract
- 实现 pipeline、registry、processor contract
- 不直接依赖 gRPC、Redis、配置文件格式

关键文件：

- [internal/domain/core/pipeline.go](/E:/github/Memory_chunk/internal/domain/core/pipeline.go)
- [internal/domain/core/registry.go](/E:/github/Memory_chunk/internal/domain/core/registry.go)

### 3.9 `internal/domain/core/repository/`

职责：

- 定义仓储契约与仓储 DTO
- 让应用服务、processor、worker 依赖统一 repository interface
- 避免业务层直接依赖 Redis client 细节

关键文件：

- [internal/domain/core/repository/repository_contracts.go](/E:/github/Memory_chunk/internal/domain/core/repository/repository_contracts.go)

### 3.10 `internal/domain/core/processor/`

职责：

- 放置具体上下文治理动作的编排层
- processor 只负责编排，具体文本 / RAG / prompt 实现优先下沉到 `core/components`
- 通过 registry 被 pipeline 按策略顺序执行
- 当前按阶段聚合文件组织，而不是为每个 processor 单独拆文件

关键文件：

- [internal/domain/core/processor/stage_01_preprocess_processors.go](/E:/github/Memory_chunk/internal/domain/core/processor/stage_01_preprocess_processors.go)
- [internal/domain/core/processor/stage_02_transform_processors.go](/E:/github/Memory_chunk/internal/domain/core/processor/stage_02_transform_processors.go)
- [internal/domain/core/processor/stage_03_compaction_processors.go](/E:/github/Memory_chunk/internal/domain/core/processor/stage_03_compaction_processors.go)
- [internal/domain/core/processor/stage_04_finalize_processors.go](/E:/github/Memory_chunk/internal/domain/core/processor/stage_04_finalize_processors.go)
- [internal/domain/core/processor/processor_support.go](/E:/github/Memory_chunk/internal/domain/core/processor/processor_support.go)

### 3.11 `internal/domain/core/components/`

职责：

- 承载文本清洗、RAG 规范化、prompt 组装等可复用实现
- 为 processor 提供统一组件能力，避免把实现细节分散在 pipeline / heuristic 中

关键文件：

- [internal/domain/core/components/text_sanitizer.go](/E:/github/Memory_chunk/internal/domain/core/components/text_sanitizer.go)
- [internal/domain/core/components/rag_normalizer.go](/E:/github/Memory_chunk/internal/domain/core/components/rag_normalizer.go)
- [internal/domain/core/components/prompt_component.go](/E:/github/Memory_chunk/internal/domain/core/components/prompt_component.go)
- [internal/domain/core/components/fragment_transformer.go](/E:/github/Memory_chunk/internal/domain/core/components/fragment_transformer.go)
- [internal/domain/core/components/chunk_metadata_helper.go](/E:/github/Memory_chunk/internal/domain/core/components/chunk_metadata_helper.go)

### 3.12 `internal/infra/config/`

职责：

- 负责配置加载、默认值填充与配置校验
- 保持工程配置格式与业务主链解耦

关键文件：

- [internal/infra/config/config.go](/E:/github/Memory_chunk/internal/infra/config/config.go)
- [internal/infra/config/policy.go](/E:/github/Memory_chunk/internal/infra/config/policy.go)

### 3.13 `internal/observability/`

职责：

- 定义应用层可依赖的观测抽象
- 让 service、store、worker 不直接依赖具体 metrics SDK

关键文件：

- [internal/observability/recorder.go](/E:/github/Memory_chunk/internal/observability/recorder.go)
- [internal/observability/metrics/prometheus_recorder.go](/E:/github/Memory_chunk/internal/observability/metrics/prometheus_recorder.go)
- [internal/observability/tracing/provider.go](/E:/github/Memory_chunk/internal/observability/tracing/provider.go)

### 3.14 `internal/support/`

职责：

- 存放跨层复用能力与接入实现
- 当前已按接入对象分为 `redis / summary / tempo`
- 通用算法与工具继续保留在 `heuristic / tokenizer`

关键文件：

- [internal/support/heuristic/json.go](/E:/github/Memory_chunk/internal/support/heuristic/json.go)
- [internal/support/heuristic/extract.go](/E:/github/Memory_chunk/internal/support/heuristic/extract.go)
- [internal/support/heuristic/lines.go](/E:/github/Memory_chunk/internal/support/heuristic/lines.go)
- [internal/support/redis/redis_repository.go](/E:/github/Memory_chunk/internal/support/redis/redis_repository.go)
- [internal/support/summary/summary_worker.go](/E:/github/Memory_chunk/internal/support/summary/summary_worker.go)
- [internal/support/summary/heuristic_summarizer.go](/E:/github/Memory_chunk/internal/support/summary/heuristic_summarizer.go)
- [internal/support/tempo/tempo_repository.go](/E:/github/Memory_chunk/internal/support/tempo/tempo_repository.go)

## 4. 公开 API 与内部实现边界

当前边界刻意做成“Java 风格应用分层”的样子：

- `api/` 是外部协议边界
- `pkg/service/` 是公开服务接口
- `internal/service/` 是应用服务实现
- `internal/domain/core/` 是核心业务内核
- `internal/domain/core/repository/` 是持久化契约边界
- `internal/support/redis|summary|tempo` 是接入实现细节
- `internal/controller/grpc/` 是 transport controller 入口

这样做的直接结果是：

- gRPC 不再等同于业务服务本身
- 后续如果要做 in-process 调用，可以直接依赖 `pkg/service`
- 后续如果要新增别的传输层，也不需要把逻辑再塞回 handler

## 5. 核心对象与契约

### 5.1 `RefinerService`

当前公开服务契约：

```go
type RefinerService interface {
    Refine(ctx context.Context, req *refinerv1.RefineRequest) (*refinerv1.RefineResponse, error)
    PageIn(ctx context.Context, req *refinerv1.PageInRequest) (*refinerv1.PageInResponse, error)
}
```

意义：

- 这是应用对外暴露的最小服务面
- gRPC handler 只是这个接口的一种 controller / transport 实现
- `pkg/client` 也围绕这个接口工作

### 5.2 `Pipeline`

`Pipeline` 是核心执行器，职责是：

- 统一 Token 计数口径
- 按策略顺序执行 processor
- 汇总 step audit 与 semantic audit
- 产出最终内部 prompt 与统一 audit 结果

对应代码：

- [internal/domain/core/pipeline.go](/E:/github/Memory_chunk/internal/domain/core/pipeline.go)

### 5.3 `Registry`

`Registry` 负责：

- 注册 processor
- 按 step name 解析策略所需 processor 列表

对应代码：

- [internal/domain/core/registry.go](/E:/github/Memory_chunk/internal/domain/core/registry.go)
- [internal/bootstrap/processors.go](/E:/github/Memory_chunk/internal/bootstrap/processors.go)

### 5.4 `PageRepository / SummaryJobRepository`

这两个抽象把核心链路与 Redis 细节隔开：

- `PageRepository` 负责 page / summary 的读写与回填
- `SummaryJobRepository` 负责异步摘要任务投递
- `SummaryJobConsumer` 负责 worker 侧消费与 ack

仓储契约位于：

- [internal/domain/core/repository/repository_contracts.go](/E:/github/Memory_chunk/internal/domain/core/repository/repository_contracts.go)

当前 Redis 实现位于：

- [internal/support/redis/redis_repository.go](/E:/github/Memory_chunk/internal/support/redis/redis_repository.go)

## 6. 请求主链

### 6.1 `Refine`

代码主链：

1. gRPC 请求进入 [internal/controller/grpc/refine_controller.go](/E:/github/Memory_chunk/internal/controller/grpc/refine_controller.go)
2. handler 委托给 [pkg/service/refiner.go](/E:/github/Memory_chunk/pkg/service/refiner.go) 定义的 service interface
3. [internal/service/refine_service.go](/E:/github/Memory_chunk/internal/service/refine_service.go) 解析 policy
4. [internal/mapper/refine_request_mapper.go](/E:/github/Memory_chunk/internal/mapper/refine_request_mapper.go) 把 protobuf request 收敛为 dto 并继续映射成内部模型
5. [internal/domain/core/pipeline.go](/E:/github/Memory_chunk/internal/domain/core/pipeline.go) 执行 processor chain
6. [internal/mapper/refine_response_mapper.go](/E:/github/Memory_chunk/internal/mapper/refine_response_mapper.go) 把内部结果映射回 protobuf response
7. gRPC 返回 `RefineResponse`

### 6.2 `PageIn`

代码主链：

1. gRPC 请求进入 controller
2. service 层校验并遍历 `page_keys`
3. 通过 [internal/domain/core/repository/repository_contracts.go](/E:/github/Memory_chunk/internal/domain/core/repository/repository_contracts.go) 定义的 `PageRepository` 优先读取 summary，底层当前由 [internal/support/redis/redis_repository.go](/E:/github/Memory_chunk/internal/support/redis/redis_repository.go) 实现
4. service 层组装 `PageInResponse`
5. `StoredPage` 在保持 `content / is_summary / summary_job_id` 兼容字段的同时，额外返回结构化 `summary_artifact`

## 7. Processor 设计

当前 processor 设计遵循三个原则：

- 单一职责：每个 processor 只负责一类变换
- 可编排：是否执行由 policy step 决定
- 可解释：执行结果进入 audit，而不是静默改写

补充说明：

- 这里的 processor 名称表示“步骤”，不是“文件一对一实现”
- 当前文件组织按 `stage_01` 到 `stage_04` 聚合
- `collapse / compact / canonicalize / assemble` 等步骤内部会优先委托 `core/components`，processor 本身只保留编排与胶水逻辑

当前重要 processor：

- `paging`：把超长 chunk 分页，保留第一页并把其余页写入 store
- `collapse`：对重复 chunk 去重并尽量保留来源
- `compact`：做安全微压缩，不主动改写语义
- `canonicalize`：稳定化 `rag_chunks` 和 `sources` 的顺序，减少 prefix 抖动
- `json_trim` / `table_reduce` / `code_outline` / `error_stack_focus`：按 fragment type 做结构感知处理
- `snip`：对高密度长片段做 middle-out 截断
- `auto_compact_sync`：同步低风险整理
- `auto_compact_async`：投递异步摘要任务
- `assemble`：统一拼装最终 prompt

相关代码：

- [internal/domain/core/processor/stage_01_preprocess_processors.go](/E:/github/Memory_chunk/internal/domain/core/processor/stage_01_preprocess_processors.go)
- [internal/domain/core/processor/stage_02_transform_processors.go](/E:/github/Memory_chunk/internal/domain/core/processor/stage_02_transform_processors.go)
- [internal/domain/core/processor/stage_03_compaction_processors.go](/E:/github/Memory_chunk/internal/domain/core/processor/stage_03_compaction_processors.go)
- [internal/domain/core/processor/stage_04_finalize_processors.go](/E:/github/Memory_chunk/internal/domain/core/processor/stage_04_finalize_processors.go)
- [internal/domain/core/components/prompt_component.go](/E:/github/Memory_chunk/internal/domain/core/components/prompt_component.go)

## 8. 基础设施与适配层设计

### 8.1 配置

- [internal/infra/config/config.go](/E:/github/Memory_chunk/internal/infra/config/config.go) 负责服务配置加载与校验
- [internal/infra/config/policy.go](/E:/github/Memory_chunk/internal/infra/config/policy.go) 负责 policy 配置解析

### 8.2 Redis Store

- page 内容优先按 content-addressed artifact key 存储
- summary artifact 优先回填
- summary queue 基于 Redis Stream
- 读取 summary artifact 时会执行 `content_hash / schema_version / provider_version / expires_at` 校验，不合法则删除并回退原 page

对应代码：

- [internal/support/redis/redis_repository.go](/E:/github/Memory_chunk/internal/support/redis/redis_repository.go)

### 8.3 Summary Worker

- 从 queue 消费 summary job
- 通过 `SummaryProvider` 生成启发式结构化 `SummaryArtifact`
- 把 artifact 写回 store，并在读取时执行版本/TTL/内容哈希失效校验

对应代码：

- [internal/support/summary/summary_worker.go](/E:/github/Memory_chunk/internal/support/summary/summary_worker.go)
- [internal/support/summary/heuristic_summarizer.go](/E:/github/Memory_chunk/internal/support/summary/heuristic_summarizer.go)

### 8.4 Tokenizer

- 对外只暴露统一计数能力
- 当前底层实现封装在 support 层的独立 tokenizer 包中

对应代码：

- [internal/support/tokenizer/token_counter.go](/E:/github/Memory_chunk/internal/support/tokenizer/token_counter.go)

### 8.5 Observability

- `internal/observability` 定义 recorder contract
- `internal/observability/metrics` 提供 Prometheus 实现
- `internal/observability/tracing` 提供 tracing provider
- runtime 会额外启动一个独立的 metrics HTTP server
- 当前指标重点覆盖 `refine/pagein`、pipeline step、token、stable prefix、page reuse、summary job

## 9. 扩展指南

### 9.1 新增 Processor

推荐步骤：

1. 先判断逻辑是否应下沉到 [internal/domain/core/components](/E:/github/Memory_chunk/internal/domain/core/components) 复用
2. 再把 processor 编排代码放入对应阶段聚合文件
3. 实现 `Descriptor()` 与 `Process()`
4. 在 [internal/bootstrap/processors.go](/E:/github/Memory_chunk/internal/bootstrap/processors.go) 注册
5. 在 `config/policies.yaml` 中编排 step
6. 补对应测试与文档

### 9.2 新增传输层

推荐方式：

1. 保持 `pkg/service.RefinerService` 不变
2. 新增一个 controller 包，例如 `internal/controller/http`
3. 把请求解析与响应回写限制在 controller 层
4. 不把业务逻辑回塞到 transport handler

### 9.3 强化 SDK / Client

如果后续要做真正的对外 SDK，有两条路：

1. 继续增强 [pkg/client/client.go](/E:/github/Memory_chunk/pkg/client/client.go)，把它作为 in-process facade
2. 单独补一个真正的 gRPC client package，面向跨进程调用

## 10. 推荐阅读顺序

建议按下面顺序读代码：

1. [docs/layered-architecture.md](/E:/github/Memory_chunk/docs/layered-architecture.md)
2. [cmd/refiner/main.go](/E:/github/Memory_chunk/cmd/refiner/main.go)
3. [internal/bootstrap/runtime.go](/E:/github/Memory_chunk/internal/bootstrap/runtime.go)
4. [internal/controller/grpc/refine_controller.go](/E:/github/Memory_chunk/internal/controller/grpc/refine_controller.go)
5. [internal/service/refine_service.go](/E:/github/Memory_chunk/internal/service/refine_service.go)
6. [internal/domain/core/pipeline.go](/E:/github/Memory_chunk/internal/domain/core/pipeline.go)
7. [internal/domain/core/processor/stage_01_preprocess_processors.go](/E:/github/Memory_chunk/internal/domain/core/processor/stage_01_preprocess_processors.go)
8. [internal/domain/core/repository/repository_contracts.go](/E:/github/Memory_chunk/internal/domain/core/repository/repository_contracts.go)
9. [internal/support/redis/redis_repository.go](/E:/github/Memory_chunk/internal/support/redis/redis_repository.go)
10. [internal/support/summary/summary_worker.go](/E:/github/Memory_chunk/internal/support/summary/summary_worker.go)

## 11. 当前设计判断

当前代码结构的主要优点：

- 对外 API、应用服务、核心逻辑、基础设施实现已经分层清楚
- gRPC 从“业务实现层”退回成了“传输适配层”
- repository contract 已从 infra 提升到 core，依赖方向更稳定
- 结构上已经收敛为单一结构化协议，对外不再保留旧兼容桥接
- 后续要继续往 Java 风格应用工程靠时，迁移成本更低

当前仍存在的约束：

- summary 当前已有 `SummaryProvider` 抽象，但只有启发式 provider，一对多 provider 体系尚未完成
- repository 目前仍由一个 Redis 实现同时承载 page 与 summary queue
- 观测、评测、测试覆盖还不完整
- 当前应用层已做 cache-aware 稳定化，但 KV 容量 / 淘汰 / 量化仍依赖下游 serving engine

## 12. 结论

当前代码已经不再是“以 gRPC handler 为中心”的组织方式，而是“以公开 service API 为中心”的应用分层方式。

这使它更像典型 Java 应用的结构：

`api -> service interface -> application service -> core domain -> support implementation -> bootstrap -> main`

后续如果继续演进，最重要的是守住这个边界，不要再把逻辑倒灌回 support 实现层或 main。
