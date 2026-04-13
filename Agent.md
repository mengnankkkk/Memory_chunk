# Context Refiner Agent Guide

> 2026-04-13 结构重构后，请优先按新的分层理解项目：
> `api -> pkg/service -> internal/controller -> internal/service -> internal/mapper + internal/dto -> internal/domain -> internal/adapter + internal/observability -> internal/bootstrap -> cmd/refiner`
> 路径细节与职责边界以 [docs/layered-architecture.md](/E:/github/Memory_chunk/docs/layered-architecture.md) 为准。

## 1. 项目定位

`Context Refiner` 是一个位于 AI 应用层与大模型 API 之间的 Go gRPC 上下文清洗服务。

它的核心目标不是“生成答案”，而是：

- 统一计算输入 Token
- 按策略执行 `processor pipeline`
- 对超长上下文做分页、压缩、裁剪与异步摘要排队
- 把可解释的审计信息返回给上游

## 2. 先读什么

如果你是第一次接手这个仓库，建议按顺序看：

1. [README.md](/E:/github/Memory_chunk/README.md)
2. [docs/README.md](/E:/github/Memory_chunk/docs/README.md)
3. [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
4. [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)
5. [cmd/refiner/main.go](/E:/github/Memory_chunk/cmd/refiner/main.go)
6. [internal/bootstrap/runtime.go](/E:/github/Memory_chunk/internal/bootstrap/runtime.go)
7. [internal/bootstrap/processors.go](/E:/github/Memory_chunk/internal/bootstrap/processors.go)
8. [internal/service/refine_service.go](/E:/github/Memory_chunk/internal/service/refine_service.go)
9. [internal/domain/core/pipeline.go](/E:/github/Memory_chunk/internal/domain/core/pipeline.go)

## 3. 关键目录

- [cmd/refiner](/E:/github/Memory_chunk/cmd/refiner)：程序启动入口
- [internal/bootstrap](/E:/github/Memory_chunk/internal/bootstrap)：runtime 组装与 processor 注册
- [internal/service](/E:/github/Memory_chunk/internal/service)：应用服务与用例编排
- [internal/controller/grpc](/E:/github/Memory_chunk/internal/controller/grpc)：gRPC 入口控制层
- [internal/domain/core](/E:/github/Memory_chunk/internal/domain/core)：领域模型、pipeline、registry
- [internal/domain/core/processor](/E:/github/Memory_chunk/internal/domain/core/processor)：上下文治理动作
- [internal/adapter/outbound](/E:/github/Memory_chunk/internal/adapter/outbound)：Redis、summary worker 等出站适配层
- [internal/support/heuristic](/E:/github/Memory_chunk/internal/support/heuristic)：跨包复用的启发式文本整理规则
- [docs](/E:/github/Memory_chunk/docs)：项目主文档体系

## 4. 不变约束

- 基本业务逻辑以 `Refine -> Pipeline -> Processor -> Assemble` 主链为准，不要为“看起来更优雅”随意改协议行为。
- `processor` 的主要职责是变换 `RefineRequest`，并同步维护 `CurrentTokens`、`Audits`、`PendingSummaryJobIDs`。
- `controller/grpc` 层只做协议适配，不应下沉复杂治理逻辑。
- `service` 层负责应用主流程，但不要吞掉 `domain/core` 的独立性。
- `summary` 当前仍是启发式摘要，不要把它误当成最终的 LLM provider 抽象。
- `internal/support/heuristic` 用来承接重复的启发式规则；不要再新增语义模糊的 `util/common/misc` 包。

## 5. 常见改动入口

### 新增或调整处理器

1. 在 [internal/domain/core/processor](/E:/github/Memory_chunk/internal/domain/core/processor) 新增或修改实现
2. 在 [internal/bootstrap/processors.go](/E:/github/Memory_chunk/internal/bootstrap/processors.go) 的 `buildRegistry` 中注册
3. 在 [config/policies.yaml](/E:/github/Memory_chunk/config/policies.yaml) 中编排步骤
4. 更新 [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)

### 调整协议映射

1. 修改 [internal/mapper/refine_request_mapper.go](/E:/github/Memory_chunk/internal/mapper/refine_request_mapper.go) 和 [internal/mapper/refine_response_mapper.go](/E:/github/Memory_chunk/internal/mapper/refine_response_mapper.go)
2. 保持 [internal/controller/grpc/refine_controller.go](/E:/github/Memory_chunk/internal/controller/grpc/refine_controller.go) 只负责 gRPC 委托
3. 检查响应字段与审计字段是否仍然自洽

### 调整启发式摘要或结构化提取规则

1. 优先检查 [internal/support/heuristic/json.go](/E:/github/Memory_chunk/internal/support/heuristic/json.go)、[internal/support/heuristic/extract.go](/E:/github/Memory_chunk/internal/support/heuristic/extract.go)、[internal/support/heuristic/lines.go](/E:/github/Memory_chunk/internal/support/heuristic/lines.go)
2. 再看 [internal/domain/core/processor/structured_processors.go](/E:/github/Memory_chunk/internal/domain/core/processor/structured_processors.go)
3. 再看 [internal/adapter/outbound/summary/heuristic_summarizer.go](/E:/github/Memory_chunk/internal/adapter/outbound/summary/heuristic_summarizer.go)

## 6. 验证方式

常用最小验证：

```powershell
gofmt -w cmd internal
go test ./...
```

如果改动涉及：

- Redis 读写：补充检查 [internal/domain/core/repository/repository_contracts.go](/E:/github/Memory_chunk/internal/domain/core/repository/repository_contracts.go) 和 [internal/adapter/outbound/redis/redis_repository.go](/E:/github/Memory_chunk/internal/adapter/outbound/redis/redis_repository.go)
- 配置：检查 [config/service.yaml](/E:/github/Memory_chunk/config/service.yaml) 与 [internal/infra/config/config.go](/E:/github/Memory_chunk/internal/infra/config/config.go)
- 文档入口：同步更新 [README.md](/E:/github/Memory_chunk/README.md) 与 [docs/README.md](/E:/github/Memory_chunk/docs/README.md)

## 7. 文档同步规则

- 改代码结构或阅读顺序：更新 [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)
- 改文档入口或索引：更新 [README.md](/E:/github/Memory_chunk/README.md) 与 [docs/README.md](/E:/github/Memory_chunk/docs/README.md)
- 改当前状态或后续优先级：更新 [docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md) / [docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)

## 8. 当前残余风险

- 仓库整体测试覆盖仍偏少，重构应优先补小而稳定的单测
- `summary` 仍以启发式规则为主，未来抽象真实 provider 时要避免把当前格式写死到上层协议
- Redis 与 worker 相关行为更适合后续补集成测试，不应仅凭静态阅读断言完全可靠
