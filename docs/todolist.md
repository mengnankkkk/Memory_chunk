# Context Refiner Todo / Snapshot

- 文档版本：`v2026.04.11`
- 更新日期：`2026-04-11`
- 文档类型：`Snapshot / Todo / Archive`
- 适用代码基线：`main` 分支当前实现

## 1. 当前快照

### 1.1 项目状态

- 当前阶段：`核心底盘已成型，应用层 KV 命中优化已完成到 D，尚未产品化完成`
- 当前主要定位：`应用层上下文清洗与稳定化接口，不负责模型层 KV block 管理`
- 当前最关键缺口：`测试闭环`、`Explain / dry-run`、`离线 replay 评测`、`真实摘要 provider`

### 1.2 当前已确认能力

- 已有 `gRPC Refine / PageIn` 主链
- 已有 `Pipeline-Processor` 可插拔处理链
- 已有 Redis `page artifact / summary job / prefix cache registry`
- 已有 Prometheus / Tracing / Grafana / Tempo 基础观测
- 已有应用层 `stable prefix` 规范化与分层 prefix hash
- 已有应用层 `miss reason` 诊断
- 已有应用层 cache 策略：
  说明：`admission policy`、`namespace`、`TTL 分层`、`热点前缀统计`、`prewarm`

### 1.3 当前应用层 KV 优化完成度

- A 规范化增强：`已完成`
- B 分层 Prefix 身份：`已完成`
- C Miss Reason 诊断：`已完成`
- D 应用层 Cache 策略：`已完成`
- E Explain / Debug 能力：`未完成`
- F 观测与评测闭环：`未完成`

### 1.4 当前仍未完成的关键能力

- 当前没有任何测试文件
- 当前默认配置仍为占位值，不能直接开箱启动
- 当前 summary worker 仍为启发式摘要
- 当前摘要回填仍偏 page 级
- 当前还没有 `dry_run / explain / cache debug`
- 当前还没有离线 replay / prefix churn / top miss dashboard

## 2. 未完成任务

### 2.1 P0 产品化闭环

- [ ] 补最小本地运行方案
- [ ] 补单元测试
- [ ] 补 `Refine` / `PageIn` 集成测试
- [ ] 清理配置占位与启动说明
- [ ] 补 cache-aware 回归测试

### 2.2 P1 摘要链路升级

- [ ] 抽象 `SummaryProvider`
- [ ] 接真实外部摘要模型
- [ ] 设计结构化 `SummaryArtifact`
- [ ] 增加 summary 失效与重试策略
- [ ] 支持 chunk 级 summary artifact

### 2.3 应用层 KV 下一阶段

#### E. Explain / Debug 能力

- [ ] 增加 `dry_run` 模式，仅返回清洗结果与缓存诊断，不写入状态
- [ ] 增加 explain metadata，输出 stable prefix 各段摘要
- [ ] 增加规范化前后 diff 摘要
- [ ] 增加 cache debug 字段，输出 hash、lookup、hit_count、miss_reason
- [ ] 增加 normalized prompt preview 或 segment preview

#### F. 观测与评测闭环

- [ ] 增加 prefix churn rate 指标
- [ ] 增加 top miss reason 指标与面板
- [ ] 增加 top hot prefix 统计视图
- [ ] 增加 canonicalize 前后 token 变化统计
- [ ] 增加历史请求 replay 工具，用于离线计算 prefix hit ratio
- [ ] 增加“哪个 processor 破坏稳定前缀最多”的分析
- [ ] 增加 prefix hit / miss reason dashboard
- [ ] 增加 dashboard alerting

### 2.4 其他增强

- [ ] 增加压缩效果评测工具
- [ ] 增加 `log_dedup`
- [ ] 增加 `tool_output_focus`
- [ ] 增加 `rag_rerank_trim`

### 2.5 当前建议顺序

1. 先补 `Explain / dry-run / cache debug`
2. 再补 `replay / dashboard / alerting`
3. 再补 `测试与最小运行闭环`
4. 最后继续 `摘要链路升级与高级 Processor`

## 3. 已完成任务归档

### 2026-04-11 / 应用层 KV 第二阶段 D 已完成

- [x] 增加 prefix cache admission policy
  说明：基于 `min_stable_prefix_tokens` 与 `min_segment_count` 过滤过短或低价值前缀
- [x] 增加热点前缀统计
  说明：Redis 增加 namespace 级 `hot prefix` 统计与 `hot_score`
- [x] 增加 TTL 分层策略
  说明：达到 `hot_threshold` 后使用 `hot_ttl`，并记录 `cache_tier`
- [x] 增加按 `policy / model / tenant` 的 cache namespace
  说明：tenant 当前默认来自 metadata 或配置 `default_tenant=global`
- [x] 增加固定模板与固定 system prompt 的预热能力
  说明：新增 `prefix_cache.prewarm` 配置，启动时写入 prefix registry

### 2026-04-11 / 应用层 KV 第二阶段 A/B/C 已完成

- [x] A 规范化增强
  说明：清洗时间戳、随机 ID、request/session/trace 等高抖动字段；规范 URL、路径、source；JSON 稳定化并剔除 volatile keys；区分 `system / memory / rag / active_turn` 规则
- [x] B 分层 Prefix 身份
  说明：新增 `system/memory/rag/combined` 四层 hash 与 token 统计，并回写 metadata
- [x] C Miss Reason 诊断
  说明：支持 `empty / short_prefix / low_value_prefix / ttl_expired / hash_changed / model_changed` 与 `system_changed / memory_changed / rag_changed / normalization_changed`

### 2026-04-11 / 应用层 KV 第一阶段已完成

- [x] stable prefix prompt 布局完成
  说明：`Stable Context -> Conversation Memory -> Active Turn`
- [x] `RAGChunk / sources / prompt` 规范化完成
- [x] content-addressed artifact/page key 复用完成
- [x] prefix cache registry 完成
  说明：按 `model + normalized stable prefix` 计算 hash 并登记

### 2026-04-11 / 观测底座已完成

- [x] Prometheus metrics 完成
  说明：`Refine / PageIn / pipeline / page artifact / store load / summary job / prefix cache`
- [x] OTel tracing 完成
  说明：覆盖 `service / pipeline / redis store / summary worker`
- [x] Grafana / Tempo / OTel Collector 本地观测栈完成
  说明：已提供 provisioning、dashboard 与 `docker compose` 配置

### 2026-04-11 / 核心底盘已完成

- [x] Go 项目骨架、protobuf、gRPC 服务端完成
- [x] `Pipeline-Processor` 主链完成
- [x] Redis page-out / page-in / summary worker 完成
- [x] `PageIn` 优先返回摘要结果完成
- [x] `go build ./...` 与 `go test ./...` 当前可通过
