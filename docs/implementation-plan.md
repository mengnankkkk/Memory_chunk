# Context Refiner 实施计划与路线图

- 文档版本：`v2026.04.06`
- 更新日期：`2026-04-06`
- 文档类型：`Roadmap / Plan`
- 适用代码基线：`main` 分支当前实现

## 1. 当前阶段结论

基于当前代码、构建结果和仓库状态，项目当前处于：

`Phase 1 完成度较高，正在进入 Phase 2 与 Phase 3 的交界阶段`

换句话说：

- 核心底盘已经搭出来了
- 但工程化和产品化能力还没有补齐

## 2. 当前已经完成的里程碑

### 2.1 核心主链

- gRPC 协议与服务端接通
- protobuf 代码已生成
- tokenizer 统一使用 `tiktoken-go`
- pipeline + registry 已打通
- processor capability 已引入
- 最终 prompt 组装已统一

### 2.2 上下文治理能力

- 结构化 `RAGFragment`
- Redis page-out / page-in
- 去重折叠
- 安全微压缩
- `snip`
- 结构化处理器
- 异步摘要排队
- summary worker 消费与写回
- 语义审计

### 2.3 代码状态

- `go build ./...` 可通过
- `go test ./...` 可执行，但全部包为 `no test files`
- 默认配置仍为占位值，不能直接启动生产实例

## 3. 当前未完成但优先级最高的事项

### 3.1 摘要产品化

当前问题：

- worker 已有，但仍是启发式摘要
- 没有 Provider 抽象
- 没有稳定的 chunk 级摘要对象

目标：

- 允许启发式与 LLM 两种 Provider 并存
- 摘要结果具备元数据、版本、过期策略
- `PageIn` 不再只读 page 级 summary 字符串

### 3.2 观测与评测

当前问题：

- 没有 Prometheus 指标
- 没有 Trace
- 没有标准评测样本
- 没有回归度量

目标：

- 能观察输入/输出 Token
- 能定位热点 Processor
- 能评估 page-out、summary 命中和 budget 达标率

### 3.3 测试与可运行性

当前问题：

- 没有单测
- 没有集成测试
- 没有本地一键运行指引

目标：

- 补最小测试闭环
- 补最小启动闭环

## 4. 推荐分阶段推进

## Phase 2：摘要产品化

### 目标

把当前启发式闭环升级成可扩展摘要能力。

### 建议拆分

#### 2.1 Provider 抽象

引入类似接口：

```go
type SummaryProvider interface {
    Name() string
    Summarize(ctx context.Context, job SummaryJob) (SummaryArtifact, error)
}
```

#### 2.2 Summary Artifact 升级

建议从纯字符串扩展为结构化对象，至少包含：

- `artifact_id`
- `session_id`
- `request_id`
- `chunk_id`
- `content_hash`
- `summary_text`
- `fragment_types`
- `provider`
- `created_at`
- `expires_at`

#### 2.3 失效与重试

- 基于 `content_hash` 变化失效
- 基于 TTL 过期
- 基于 provider 版本触发重建
- provider 失败时保留启发式降级路径

### 验收标准

- 外部模型故障不影响 `Refine` 主链返回
- 有效期内同一内容可稳定复用摘要
- `PageIn` 能区分原始 page 与结构化 summary

## Phase 3：观测与评测

### 目标

把“感觉有效”升级成“可度量、可回归、可定位问题”。

### 第一批指标

- `refiner_input_tokens_total`
- `refiner_output_tokens_total`
- `refiner_step_latency_seconds`
- `refiner_pageout_total`
- `refiner_summary_jobs_total`
- `refiner_budget_met_total`

### 第一批 Trace 观察点

- `Refine`
- `PageIn`
- Redis page save/load
- Redis Stream enqueue/consume/ack
- summary provider 调用

### 第一批评测维度

- 压缩前后 Token 比例
- budget 达标率
- page-out 比例
- 引文保留率
- code fence 保留率
- error stack 保留率
- summary 命中率

## Phase 4：结构化策略深化

### 当前已完成起点

- `json_trim`
- `table_reduce`
- `code_outline`
- `error_stack_focus`

### 建议下一批处理器

- `log_dedup`
- `tool_output_focus`
- `rag_rerank_trim`

### 进入条件

- 必须先有测试样本和回归基线
- 否则很容易只得到“压缩率更高”而不是“语义更稳”

## Phase 5：缓存复用与多级治理

### 目标

把单次请求压缩，升级成跨请求可复用的上下文治理。

### 关键方向

- 基于 `content_hash` 的跨请求复用
- page 级 / chunk 级 / session 级摘要层次
- 热点上下文缓存
- 版本和失效策略

## 5. 推荐执行顺序

建议按以下顺序推进：

1. 补最小本地运行方案
2. 补单测与集成测试
3. 补 Prometheus 指标
4. 抽象 Summary Provider
5. 接真实外部摘要模型
6. 升级 Summary Artifact
7. 补 Tracing
8. 增加离线评测工具
9. 增加 `log_dedup`
10. 增加 `tool_output_focus`
11. 增加 `rag_rerank_trim`
12. 推进跨请求缓存复用

## 6. 最近两轮迭代建议

### 迭代 A：让系统真正“可验证”

产出建议：

- 本地运行说明
- 样例配置
- tokenizer / paging / worker 单测
- `Refine` / `PageIn` 集成测试
- 文档与实际代码一致性清理

### 迭代 B：让摘要链路真正“可升级”

产出建议：

- Summary Provider 抽象
- Heuristic Provider
- LLM Provider 占位实现
- 结构化 Summary Artifact
- summary 失效策略

## 7. 当前最大的风险

### 风险 1：继续堆新处理器，忽略测试和评测

后果：

- 很难证明策略变更是否有效
- 一旦语义损伤，定位困难

### 风险 2：误把启发式摘要当成最终方案

后果：

- 难以跨环境复用
- 难以控制摘要质量

### 风险 3：没有观测能力就开始复杂优化

后果：

- 只能靠体感调优
- KPI 不可信

## 8. 结论

当前最合理的推进方式不是“继续扩更多规则”，而是：

1. 先让系统可运行、可测试、可观察
2. 再把摘要链路抽象稳
3. 再继续做复杂策略和缓存复用

这也是后续所有计划拆分的总原则。
