# Context Refiner Agent Guide

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
5. [cmd/main.go](/E:/github/Memory_chunk/cmd/main.go)
6. [cmd/bootstrap.go](/E:/github/Memory_chunk/cmd/bootstrap.go)
7. [internal/server/refiner.go](/E:/github/Memory_chunk/internal/server/refiner.go)
8. [internal/engine/pipeline.go](/E:/github/Memory_chunk/internal/engine/pipeline.go)

## 3. 关键目录

- [cmd](/E:/github/Memory_chunk/cmd)：程序入口与 bootstrap 组装
- [internal/config](/E:/github/Memory_chunk/internal/config)：配置与策略解析
- [internal/engine](/E:/github/Memory_chunk/internal/engine)：领域模型、pipeline、registry
- [internal/processor](/E:/github/Memory_chunk/internal/processor)：上下文治理动作
- [internal/heuristic](/E:/github/Memory_chunk/internal/heuristic)：跨包复用的启发式文本整理规则
- [internal/server](/E:/github/Memory_chunk/internal/server)：gRPC handler 与 request/response mapping
- [internal/store](/E:/github/Memory_chunk/internal/store)：Redis page store 与 summary queue
- [internal/summary](/E:/github/Memory_chunk/internal/summary)：summary worker 与启发式渲染
- [docs](/E:/github/Memory_chunk/docs)：项目主文档体系

## 4. 不变约束

- 基本业务逻辑以 `Refine -> Pipeline -> Processor -> Assemble` 主链为准，不要为“看起来更优雅”随意改协议行为。
- `processor` 的主要职责是变换 `RefineRequest`，并同步维护 `CurrentTokens`、`Audits`、`PendingSummaryJobIDs`。
- `server` 层只做协议映射、策略解析与错误边界控制，不应下沉复杂治理逻辑。
- `summary` 当前仍是启发式摘要，不要把它误当成最终的 LLM provider 抽象。
- `internal/heuristic` 用来承接重复的启发式规则；不要再新增语义模糊的 `util/common/misc` 包。

## 5. 常见改动入口

### 新增或调整处理器

1. 在 [internal/processor](/E:/github/Memory_chunk/internal/processor) 新增或修改实现
2. 在 [cmd/bootstrap.go](/E:/github/Memory_chunk/cmd/bootstrap.go) 的 `buildRegistry` 中注册
3. 在 [config/policies.yaml](/E:/github/Memory_chunk/config/policies.yaml) 中编排步骤
4. 更新 [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)

### 调整协议映射

1. 修改 [internal/server/mapping.go](/E:/github/Memory_chunk/internal/server/mapping.go)
2. 保持 [internal/server/refiner.go](/E:/github/Memory_chunk/internal/server/refiner.go) 只负责 handler 主流程
3. 检查响应字段与审计字段是否仍然自洽

### 调整启发式摘要或结构化提取规则

1. 优先检查 [internal/heuristic/text.go](/E:/github/Memory_chunk/internal/heuristic/text.go)
2. 再看 [internal/processor/structured.go](/E:/github/Memory_chunk/internal/processor/structured.go)
3. 再看 [internal/summary/render.go](/E:/github/Memory_chunk/internal/summary/render.go)

## 6. 验证方式

常用最小验证：

```powershell
gofmt -w cmd internal
go test ./...
```

如果改动涉及：

- Redis 读写：补充检查 [internal/store/redis.go](/E:/github/Memory_chunk/internal/store/redis.go)
- 配置：检查 [config/service.yaml](/E:/github/Memory_chunk/config/service.yaml) 与 [internal/config/config.go](/E:/github/Memory_chunk/internal/config/config.go)
- 文档入口：同步更新 [README.md](/E:/github/Memory_chunk/README.md) 与 [docs/README.md](/E:/github/Memory_chunk/docs/README.md)

## 7. 文档同步规则

- 改代码结构或阅读顺序：更新 [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)
- 改文档入口或索引：更新 [README.md](/E:/github/Memory_chunk/README.md) 与 [docs/README.md](/E:/github/Memory_chunk/docs/README.md)
- 改当前状态或后续优先级：更新 [docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md) / [docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)

## 8. 当前残余风险

- 仓库整体测试覆盖仍偏少，重构应优先补小而稳定的单测
- `summary` 仍以启发式规则为主，未来抽象真实 provider 时要避免把当前格式写死到上层协议
- Redis 与 worker 相关行为更适合后续补集成测试，不应仅凭静态阅读断言完全可靠
