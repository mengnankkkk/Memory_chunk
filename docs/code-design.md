# Context Refiner 代码设计说明

- 文档版本：`v2026.04.11`
- 更新日期：`2026-04-11`
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
  adapter/
    grpc/
  bootstrap/
  core/
    repository/
    processor/
  infra/
    config/
    store/
      redis/
    summary/
    tokenizer/
  service/
  support/
    heuristic/

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

- 装配 `infra -> core -> service -> adapter`
- 构建 registry、page store、gRPC server、summary worker
- 把启动时依赖集中在一层，避免散落到 `main`

关键文件：

- [internal/bootstrap/runtime.go](/E:/github/Memory_chunk/internal/bootstrap/runtime.go)
- [internal/bootstrap/processors.go](/E:/github/Memory_chunk/internal/bootstrap/processors.go)

### 3.6 `internal/adapter/grpc/`

职责：

- 实现 gRPC handler
- 把 RPC 请求委托给 `pkg/service.RefinerService`
- 自身不承载业务逻辑

关键文件：

- [internal/adapter/grpc/refiner_handler.go](/E:/github/Memory_chunk/internal/adapter/grpc/refiner_handler.go)

### 3.7 `internal/service/`

职责：

- 作为应用服务层主入口
- 负责 policy 解析、request mapping、pipeline 调用、response mapping
- 组织 `Refine` / `PageIn` 这两个用例

关键文件：

- [internal/service/refiner_service.go](/E:/github/Memory_chunk/internal/service/refiner_service.go)
- [internal/service/request_mapping.go](/E:/github/Memory_chunk/internal/service/request_mapping.go)
- [internal/service/response_mapping.go](/E:/github/Memory_chunk/internal/service/response_mapping.go)
- [internal/service/mapping_helpers.go](/E:/github/Memory_chunk/internal/service/mapping_helpers.go)

### 3.8 `internal/core/`

职责：

- 定义核心模型与抽象
- 定义 repository contract
- 实现 pipeline、registry、processor contract
- 不直接依赖 gRPC、Redis、配置文件格式

关键文件：

- [internal/core/pipeline.go](/E:/github/Memory_chunk/internal/core/pipeline.go)
- [internal/core/registry.go](/E:/github/Memory_chunk/internal/core/registry.go)

### 3.9 `internal/core/repository/`

职责：

- 定义仓储契约与仓储 DTO
- 让应用服务、processor、worker 依赖统一 repository interface
- 避免业务层直接依赖 Redis client 细节

关键文件：

- [internal/core/repository/repository.go](/E:/github/Memory_chunk/internal/core/repository/repository.go)

### 3.10 `internal/core/processor/`

职责：

- 放置具体上下文治理动作
- 每个 processor 保持单一职责
- 通过 registry 被 pipeline 按策略编排执行

关键文件：

- [internal/core/processor/request_clone.go](/E:/github/Memory_chunk/internal/core/processor/request_clone.go)
- [internal/core/processor/token_split.go](/E:/github/Memory_chunk/internal/core/processor/token_split.go)
- [internal/core/processor/chunk_metadata.go](/E:/github/Memory_chunk/internal/core/processor/chunk_metadata.go)
- [internal/core/processor/paging.go](/E:/github/Memory_chunk/internal/core/processor/paging.go)
- [internal/core/processor/collapse.go](/E:/github/Memory_chunk/internal/core/processor/collapse.go)
- [internal/core/processor/compact.go](/E:/github/Memory_chunk/internal/core/processor/compact.go)
- [internal/core/processor/structured.go](/E:/github/Memory_chunk/internal/core/processor/structured.go)
- [internal/core/processor/snip.go](/E:/github/Memory_chunk/internal/core/processor/snip.go)
- [internal/core/processor/auto.go](/E:/github/Memory_chunk/internal/core/processor/auto.go)
- [internal/core/processor/assemble.go](/E:/github/Memory_chunk/internal/core/processor/assemble.go)

### 3.11 `internal/infra/`

职责：

- 承载外部依赖与技术实现
- 当前包括配置加载、Redis repository implementation、summary worker、tokenizer

关键文件：

- [internal/infra/config/config.go](/E:/github/Memory_chunk/internal/infra/config/config.go)
- [internal/infra/config/policy.go](/E:/github/Memory_chunk/internal/infra/config/policy.go)
- [internal/infra/store/redis/repository.go](/E:/github/Memory_chunk/internal/infra/store/redis/repository.go)
- [internal/infra/summary/worker.go](/E:/github/Memory_chunk/internal/infra/summary/worker.go)
- [internal/infra/summary/summarizer.go](/E:/github/Memory_chunk/internal/infra/summary/summarizer.go)
- [internal/infra/tokenizer/counter.go](/E:/github/Memory_chunk/internal/infra/tokenizer/counter.go)

### 3.12 `internal/support/`

职责：

- 存放跨层复用的辅助能力
- 当前主要是 `heuristic` 文本整理规则

关键文件：

- [internal/support/heuristic/json.go](/E:/github/Memory_chunk/internal/support/heuristic/json.go)
- [internal/support/heuristic/extract.go](/E:/github/Memory_chunk/internal/support/heuristic/extract.go)
- [internal/support/heuristic/lines.go](/E:/github/Memory_chunk/internal/support/heuristic/lines.go)

## 4. 公开 API 与内部实现边界

当前边界刻意做成“Java 风格应用分层”的样子：

- `api/` 是外部协议边界
- `pkg/service/` 是公开服务接口
- `internal/service/` 是应用服务实现
- `internal/core/` 是核心业务内核
- `internal/core/repository/` 是持久化契约边界
- `internal/infra/` 是底层实现细节
- `internal/adapter/grpc/` 是 transport adapter

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
- gRPC handler 只是这个接口的一种 adapter
- `pkg/client` 也围绕这个接口工作

### 5.2 `Pipeline`

`Pipeline` 是核心执行器，职责是：

- 统一 Token 计数口径
- 按策略顺序执行 processor
- 汇总 step audit 与 semantic audit
- 产出最终 `optimized_prompt`

对应代码：

- [internal/core/pipeline.go](/E:/github/Memory_chunk/internal/core/pipeline.go)

### 5.3 `Registry`

`Registry` 负责：

- 注册 processor
- 按 step name 解析策略所需 processor 列表

对应代码：

- [internal/core/registry.go](/E:/github/Memory_chunk/internal/core/registry.go)
- [internal/bootstrap/processors.go](/E:/github/Memory_chunk/internal/bootstrap/processors.go)

### 5.4 `PageRepository / SummaryJobRepository`

这两个抽象把核心链路与 Redis 细节隔开：

- `PageRepository` 负责 page / summary 的读写与回填
- `SummaryJobRepository` 负责异步摘要任务投递
- `SummaryJobConsumer` 负责 worker 侧消费与 ack

仓储契约位于：

- [internal/core/repository/repository.go](/E:/github/Memory_chunk/internal/core/repository/repository.go)

当前 Redis 实现位于：

- [internal/infra/store/redis/repository.go](/E:/github/Memory_chunk/internal/infra/store/redis/repository.go)

## 6. 请求主链

### 6.1 `Refine`

代码主链：

1. gRPC 请求进入 [internal/adapter/grpc/refiner_handler.go](/E:/github/Memory_chunk/internal/adapter/grpc/refiner_handler.go)
2. handler 委托给 [pkg/service/refiner.go](/E:/github/Memory_chunk/pkg/service/refiner.go) 定义的 service interface
3. [internal/service/refiner_service.go](/E:/github/Memory_chunk/internal/service/refiner_service.go) 解析 policy
4. [internal/service/request_mapping.go](/E:/github/Memory_chunk/internal/service/request_mapping.go) 把 protobuf request 映射成内部模型
5. [internal/core/pipeline.go](/E:/github/Memory_chunk/internal/core/pipeline.go) 执行 processor chain
6. [internal/service/response_mapping.go](/E:/github/Memory_chunk/internal/service/response_mapping.go) 把内部结果映射回 protobuf response
7. gRPC 返回 `RefineResponse`

### 6.2 `PageIn`

代码主链：

1. gRPC 请求进入 adapter
2. service 层校验并遍历 `page_keys`
3. 通过 [internal/core/repository/repository.go](/E:/github/Memory_chunk/internal/core/repository/repository.go) 定义的 `PageRepository` 优先读取 summary，底层当前由 [internal/infra/store/redis/repository.go](/E:/github/Memory_chunk/internal/infra/store/redis/repository.go) 实现
4. service 层组装 `PageInResponse`

## 7. Processor 设计

当前 processor 设计遵循三个原则：

- 单一职责：每个 processor 只负责一类变换
- 可编排：是否执行由 policy step 决定
- 可解释：执行结果进入 audit，而不是静默改写

当前重要 processor：

- `paging`：把超长 chunk 分页，保留第一页并把其余页写入 store
- `collapse`：对重复 chunk 去重并尽量保留来源
- `compact`：做安全微压缩，不主动改写语义
- `json_trim` / `table_reduce` / `code_outline` / `error_stack_focus`：按 fragment type 做结构感知处理
- `snip`：对高密度长片段做 middle-out 截断
- `auto_compact_sync`：同步低风险整理
- `auto_compact_async`：投递异步摘要任务
- `assemble`：统一拼装最终 prompt

相关代码：

- [internal/core/processor/paging.go](/E:/github/Memory_chunk/internal/core/processor/paging.go)
- [internal/core/processor/collapse.go](/E:/github/Memory_chunk/internal/core/processor/collapse.go)
- [internal/core/processor/structured.go](/E:/github/Memory_chunk/internal/core/processor/structured.go)
- [internal/core/processor/auto.go](/E:/github/Memory_chunk/internal/core/processor/auto.go)
- [internal/core/processor/assemble.go](/E:/github/Memory_chunk/internal/core/processor/assemble.go)

## 8. Infra 设计

### 8.1 配置

- [internal/infra/config/config.go](/E:/github/Memory_chunk/internal/infra/config/config.go) 负责服务配置加载与校验
- [internal/infra/config/policy.go](/E:/github/Memory_chunk/internal/infra/config/policy.go) 负责 policy 配置解析

### 8.2 Redis Store

- page 内容用 page key 存储
- summary 结果优先回填
- summary queue 基于 Redis Stream

对应代码：

- [internal/infra/store/redis/repository.go](/E:/github/Memory_chunk/internal/infra/store/redis/repository.go)

### 8.3 Summary Worker

- 从 queue 消费 summary job
- 对 page refs 生成启发式摘要
- 把摘要写回 store

对应代码：

- [internal/infra/summary/worker.go](/E:/github/Memory_chunk/internal/infra/summary/worker.go)
- [internal/infra/summary/summarizer.go](/E:/github/Memory_chunk/internal/infra/summary/summarizer.go)

### 8.4 Tokenizer

- 对外只暴露统一计数能力
- 当前底层实现封装在 infra 层

对应代码：

- [internal/infra/tokenizer/counter.go](/E:/github/Memory_chunk/internal/infra/tokenizer/counter.go)

## 9. 扩展指南

### 9.1 新增 Processor

推荐步骤：

1. 在 [internal/core/processor](/E:/github/Memory_chunk/internal/core/processor) 新增实现
2. 实现 `Descriptor()` 与 `Process()`
3. 在 [internal/bootstrap/processors.go](/E:/github/Memory_chunk/internal/bootstrap/processors.go) 注册
4. 在 `config/policies.yaml` 中编排 step
5. 补对应测试与文档

### 9.2 新增传输层

推荐方式：

1. 保持 `pkg/service.RefinerService` 不变
2. 新增一个 adapter 包，例如 `internal/adapter/http`
3. 把请求解析与响应回写限制在 adapter 层
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
4. [internal/adapter/grpc/refiner_handler.go](/E:/github/Memory_chunk/internal/adapter/grpc/refiner_handler.go)
5. [internal/service/refiner_service.go](/E:/github/Memory_chunk/internal/service/refiner_service.go)
6. [internal/core/pipeline.go](/E:/github/Memory_chunk/internal/core/pipeline.go)
7. [internal/core/processor/paging.go](/E:/github/Memory_chunk/internal/core/processor/paging.go)
8. [internal/core/repository/repository.go](/E:/github/Memory_chunk/internal/core/repository/repository.go)
9. [internal/infra/store/redis/repository.go](/E:/github/Memory_chunk/internal/infra/store/redis/repository.go)
10. [internal/infra/summary/worker.go](/E:/github/Memory_chunk/internal/infra/summary/worker.go)

## 11. 当前设计判断

当前代码结构的主要优点：

- 对外 API、应用服务、核心逻辑、基础设施实现已经分层清楚
- gRPC 从“业务实现层”退回成了“传输适配层”
- repository contract 已从 infra 提升到 core，依赖方向更稳定
- 结构上已经为“双模式兼容”留好了口子
- 后续要继续往 Java 风格应用工程靠时，迁移成本更低

当前仍存在的约束：

- summary 仍是启发式逻辑，不是独立 provider 体系
- repository 目前仍由一个 Redis 实现同时承载 page 与 summary queue
- 观测、评测、测试覆盖还不完整

## 12. 结论

当前代码已经不再是“以 gRPC handler 为中心”的组织方式，而是“以公开 service API 为中心”的应用分层方式。

这使它更像典型 Java 应用的结构：

`api -> service interface -> application service -> core domain -> infra implementation -> bootstrap -> main`

后续如果继续演进，最重要的是守住这个边界，不要再把逻辑倒灌回 adapter 或 main。
