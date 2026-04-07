# Context Refiner 测试计划

- 文档版本：`v2026.04.06`
- 更新日期：`2026-04-06`
- 文档类型：`Testing / Plan`
- 适用代码基线：`main` 分支当前实现

## 1. 这份文档解决什么问题

本文档只聚焦一件事：

`当前仓库应该先补哪些测试、按什么顺序补、每一层测试要验证什么。`

它不是测试报告，也不是用例明细库。

它的作用是把“测试缺失”这个抽象问题，拆成可以逐步落地的执行计划。

如果你还不了解系统，请先看：

- [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
- [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)

## 2. 当前测试现状

基于当前仓库内容，可以确认：

- `go test ./...` 可以执行
- 但当前没有任何 `_test.go` 文件
- 这意味着“能跑测试”不等于“已经有测试覆盖”

当前风险不是单点风险，而是整条链路都缺少回归保护：

- 配置加载与校验缺少保护
- request 映射与默认值推导缺少保护
- processor 压缩逻辑缺少保护
- Redis page-out / page-in / summary 写回缺少保护
- gRPC `Refine` / `PageIn` 服务行为缺少保护
- summary worker 的消费与写回缺少保护

## 3. 测试目标

测试计划的目标不是一开始追求超高覆盖率，而是先建立最小可信闭环。

第一阶段要达到的目标：

- 核心主链有可重复执行的自动化验证
- 关键默认值、边界值、错误路径能被回归测试覆盖
- 文档里的启动与调用方式有最小可验证依据
- 后续新增 Processor 时有地方挂回归测试

## 4. 测试分层策略

建议采用三层：

### 4.1 Unit Test

目标：

- 低成本验证纯逻辑和小范围组件行为

优先覆盖：

- `internal/config`
- `internal/server` 中的 request / response mapping
- `internal/summary` 中的摘要启发式逻辑
- `internal/processor` 中不依赖外部 Redis 的处理器逻辑

适合验证的内容：

- 默认值是否按预期补齐
- 空输入、异常输入是否能稳定处理
- 不同 fragment type 是否映射正确
- 摘要函数是否保留关键信息且不过度丢失结构

### 4.2 Integration Test

目标：

- 验证多个模块连起来是否真的能协同工作

优先覆盖：

- `internal/store/redis.go`
- `internal/server/refiner.go`
- `internal/summary/worker.go`

建议拆成两类：

- Redis 集成测试
- gRPC 服务集成测试

Redis 集成测试重点：

- `SavePage` / `LoadPage`
- `SaveSummary` / `LoadResolvedPage`
- `EnqueueSummaryJob` / `ConsumeSummaryJobs` / `AckSummaryJob`
- summary key 优先于原始 page 返回

gRPC 服务集成测试重点：

- `Refine` 在合法请求下返回 `optimized_prompt`、`audits`、`budget_met`
- `Refine` 在未知 policy、无效 budget 下返回正确错误
- `PageIn` 在 summary 已存在时返回 `is_summary = true`
- `PageIn` 在 page 不存在时返回 `NotFound`

### 4.3 End-to-End Smoke Test

目标：

- 用最小真实环境验证“配置 -> 启动 -> 调用 -> PageIn”整条链路

这层不需要一开始就做复杂。

第一版只需要覆盖：

1. 配置真实 Redis 地址
2. 启动服务
3. 调一次 `Refine`
4. 如果出现 page-out，再调一次 `PageIn`

这层更像“最小冒烟闭环”，不是完整压测。

## 5. 建议优先级

建议按下面顺序补测试，而不是全面铺开：

1. `internal/server` 的 request / response mapping
2. `internal/summary` 的摘要逻辑
3. `internal/config` 的加载与校验
4. `internal/store` 的 Redis 集成测试
5. `internal/server` 的 `Refine` / `PageIn` 集成测试
6. `internal/summary` worker 的消费与写回集成测试
7. 最小端到端冒烟测试

这样排序的原因：

- 前三类成本低、收益高
- 中间三类能保护核心链路
- 最后一类用于确认系统在真实运行形态下没有断裂

## 6. 关键测试主题

### 6.1 配置与启动

重点验证：

- `redis.addr` 为空时报错
- `grpc.listen_addr` 为空时报错
- `key_prefix`、`page_ttl`、`summary_stream` 的默认值是否生效

### 6.2 请求映射与默认值

重点验证：

- 空 `policy` 时是否回落到默认策略
- `token_budget <= 0` 且 `model.max_context_tokens` 存在时是否正确推导预算
- 空 `request_id` 时是否自动生成
- 空 `session_id` 时是否按 `session-<request_id>` 规则生成
- 无 `fragments` 但有 `content` 时是否自动补成 body fragment
- `source` 与 `sources` 是否按预期回填

### 6.3 Processor 与语义保真

重点验证：

- 压缩前后 token 是否单调下降或至少不失控增长
- `code fence`、错误栈、来源信息是否按能力声明保留
- 结构化 fragment 在不同处理器下是否保持合理输出

这里不要只测“函数能跑”，还要测：

- 边界输入
- 空片段
- 超长文本
- 混合 `body + code + table + log + error-stack`

### 6.4 Page Store 与 Summary 回填

重点验证：

- page key 写入后能稳定读回
- summary 保存后 `LoadResolvedPage` 优先返回 summary
- 原始 page 不存在时返回明确错误
- Stream 中 payload 缺失或不合法时行为是否可接受

### 6.5 Worker 逻辑

重点验证：

- `EnsureSummaryGroup` 失败时应立即报错
- 消费到任务后能为所有 `page_refs` 写回 summary
- `AckSummaryJob` 失败时能暴露错误
- 空 fragment 与不同 fragment type 的摘要结果符合当前启发式规则

### 6.6 gRPC 接口契约

重点验证：

- 错误码是否正确
- 输出字段是否完整
- `pending_summary_job_ids`、`paged_chunks` 是否在相应场景下出现
- `PageIn` 返回内容与 Redis 存储结果一致

## 7. 边界场景与异常场景

至少要覆盖这些特殊情况：

- 未知 policy
- budget 无法推导
- 空消息数组
- 空 `rag_chunks`
- fragment type 未知
- Redis 不可达
- summary stream 没创建
- page key 过期或不存在
- summary JSON 损坏
- worker 消费到非法 payload

如果这些场景不测，系统最容易在“看起来平常不会发生”的地方退化。

## 8. 建议测试数据集

建议准备一组固定样例，后续重复复用：

- 短 body 文本
- 超长 body 文本
- 含函数声明的 code 文本
- 表格文本
- JSON object / array
- 工具输出日志
- 普通日志
- error stack
- 混合型 RAG chunk

这样做的目的，是让 Processor、worker、PageIn 行为都基于同一批样例回归。

## 9. 环境建议

推荐至少准备两种执行方式：

### 9.1 本地快速执行

用于开发中高频回归：

```powershell
go test ./...
```

### 9.2 带真实 Redis 的集成执行

建议配一个本地 Redis，或用容器临时起一个：

```powershell
docker run --name context-refiner-redis-test -p 6379:6379 -d redis:7
```

然后执行与 `store`、`server`、`summary` 相关的集成测试。

对于 gRPC 层，建议优先使用：

- 进程内 server
- 或 `bufconn` 风格的内存连接

这样比依赖真实网络端口更稳定，也更适合 CI。

## 10. 阶段性交付标准

### Phase A：建立最小测试闭环

完成标准：

- 至少补齐 `config`、`server mapping`、`summary` 的单测
- `go test ./...` 不再全部是 `no test files`

### Phase B：保护核心运行链路

完成标准：

- 补齐 Redis store 集成测试
- 补齐 `Refine` / `PageIn` 集成测试
- 补齐 worker 写回链路测试

### Phase C：建立发布前冒烟验证

完成标准：

- 能用真实 Redis 走通一次最小端到端调用
- 文档里的 quickstart 与测试验证路径保持一致

## 11. 不建议现在就做的事

当前阶段不建议一上来就：

- 先追求非常高的覆盖率数字
- 先上复杂的性能测试框架
- 先写大量 UI 式测试基建
- 在没有稳定样例数据前就堆很多 snapshot

原因很简单：

- 当前项目的最大问题是“没有回归保护”
- 不是“缺一套豪华测试平台”

## 12. 推荐落地顺序

如果现在就开始补测试，建议这样推进：

1. 先补 `internal/server/refiner.go` 的 request / response mapping
2. 再补 `internal/summary/worker.go` 中的摘要逻辑
3. 再补 `internal/config/config.go` 与 `policy.go`
4. 然后补 `internal/store/redis.go`
5. 再补 `Refine` / `PageIn` 服务集成测试
6. 最后补最小冒烟测试脚本或手工验证步骤

## 13. 与其他文档的关系

- 架构边界看 [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
- 代码结构看 [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)
- 最小运行路径看 [docs/quickstart.md](/E:/github/Memory_chunk/docs/quickstart.md)
- 当前缺口与优先级看 [docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md)
- 阶段计划看 [docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)

## 14. 当前结论

当前最合理的测试建设路线，不是“先做一套很大很全的测试体系”，而是：

1. 先补最小单测闭环
2. 再补 Redis 与 gRPC 集成保护
3. 最后再做真实环境冒烟验证

这样能用最小成本，最快给当前主链建立回归护栏。
