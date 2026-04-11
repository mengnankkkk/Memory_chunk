# Context Refiner Todo 与当前进度

- 文档版本：`v2026.04.06`
- 更新日期：`2026-04-06`
- 文档类型：`Status / Todo`
- 适用代码基线：`main` 分支当前实现

## 1. 当前总体判断

- 当前阶段：`核心底盘已成型，尚未产品化完成`
- Phase 1 完成度：`高`
- 整体项目完成度：`中低`
- 当前最关键缺口：`测试`、`观测`、`真实摘要模型`、`chunk 级摘要对象`

## 2. 已完成

### 2.1 核心代码

- [x] 建立 Go 项目骨架
- [x] 建立 `refiner.proto`
- [x] 生成 protobuf / gRPC 代码
- [x] 实现 gRPC 服务端
- [x] 接入 `tiktoken-go`
- [x] 建立 `Pipeline-Processor` 主链
- [x] 引入 Processor 能力声明
- [x] 建立结构化 `RAGFragment`

### 2.2 上下文治理能力

- [x] 统一真实 Token 计数口径
- [x] `paging` 按 Token 分页并写入 Redis
- [x] `collapse` 去重并合并来源
- [x] `compact` 安全微压缩
- [x] `snip` 中段裁剪
- [x] `auto_compact_sync`
- [x] `auto_compact_async`
- [x] `json_trim`
- [x] `table_reduce`
- [x] `code_outline`
- [x] `error_stack_focus`
- [x] 语义保真审计字段

### 2.3 状态层与回填

- [x] `PageRepository` / `SummaryJobRepository` contract
- [x] Redis repository implementation
- [x] summary worker 消费任务
- [x] 摘要写回 Redis
- [x] `PageIn` 优先返回摘要结果

### 2.4 文档

- [x] 总体架构文档
- [x] 实施计划文档
- [x] 代码设计文档
- [x] 快速上手文档
- [x] 学习解析文档
- [x] 原理剖析文档
- [x] 文档索引

## 3. 已确认但尚未完成

- [x] `go build ./...` 可通过
- [x] `go test ./...` 可执行
- [ ] 当前没有任何测试文件
- [ ] 默认配置仍为占位值，不能直接启动服务
- [ ] 当前 summary worker 仍为启发式摘要
- [ ] 当前摘要回填仍偏 page 级
- [ ] 当前没有 Prometheus 指标
- [ ] 当前没有 Tracing
- [ ] 当前没有离线评测工具

## 4. 当前最高优先级

### P0

- [ ] 补最小本地运行方案
- [ ] 补单元测试
- [ ] 补 `Refine` / `PageIn` 集成测试
- [ ] 清理配置占位与启动说明

### P1

- [ ] 抽象 `SummaryProvider`
- [ ] 接真实外部摘要模型
- [ ] 设计结构化 `SummaryArtifact`
- [ ] 增加 summary 失效与重试策略

### P2

- [ ] 增加 Prometheus 指标
- [ ] 增加 Tracing
- [ ] 增加压缩效果评测工具

### P3

- [ ] 增加 `log_dedup`
- [ ] 增加 `tool_output_focus`
- [ ] 增加 `rag_rerank_trim`
- [ ] 支持跨请求 `content_hash` 复用

## 5. 当前建议顺序

1. 先补测试和最小运行闭环
2. 再抽象和升级摘要链路
3. 再补指标、Trace、评测
4. 最后继续扩高级 Processor 和缓存复用

## 6. 本轮文档维护说明

本次文档更新已完成以下整理：

- [x] 将“大而全”文档改为“多份分块文档”
- [x] 按用途拆分为 overview / tutorial / reference / explanation 风格
- [x] 把当前进度判断与未来计划单独拉出
- [x] 统一补充日期与版本号
- [x] 统一以当前代码实现为准，修正文档滞后风险
