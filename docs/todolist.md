# Context Refiner Todo / Snapshot

- 文档版本：`v2026.04.13`
- 更新日期：`2026-04-13`
- 文档类型：`Snapshot / Todo / Archive`
- 适用代码基线：`main` 分支当前实现

## 1. 当前快照

### 1.1 项目状态

- 当前阶段：`核心底盘已成型，应用层 KV 命中优化已完成到 D，已进入工程化补齐阶段，尚未产品化完成`
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
- 已有少量单元测试起步，`go test ./...` 当前可通过

### 1.3 当前应用层 KV 优化完成度

- A 规范化增强：`已完成`
- B 分层 Prefix 身份：`已完成`
- C Miss Reason 诊断：`已完成`
- D 应用层 Cache 策略：`已完成`
- E Explain / Debug 能力：`未完成`
- F 观测与评测闭环：`未完成`

### 1.4 当前仍未完成的关键能力

- 当前已有少量 `_test.go` 文件，但还没有形成覆盖主链的测试闭环
- 当前默认配置仍为占位值，不能直接开箱启动
- 当前 summary worker 仍为启发式摘要
- 当前摘要回填仍偏 page 级
- 当前还没有固化应用层“可命中性预测”语义与解释输出
- 当前还没有 `dry_run / explain / cache debug`
- 当前虽然已有基础 overview dashboard，但还没有离线 replay / prefix churn / top miss dashboard / alerting

## 2. 未完成任务

### 2.1 P0 产品化闭环

- [ ] 补 `service mapping / summary / config` 最小单元测试闭环
- [ ] 补 `Refine` / `PageIn` / Redis / worker 集成测试
- [ ] 补最小本地运行方案
- [ ] 清理配置占位与启动说明
- [ ] 补 cache-aware 回归测试

### 2.2 P1 摘要链路升级

- [ ] 抽象 `SummaryProvider`
- [ ] 接真实外部摘要模型
- [ ] 设计结构化 `SummaryArtifact`
- [ ] 增加 summary 失效与重试策略
- [ ] 支持 chunk 级 summary artifact

### 2.3 应用层 KV 下一阶段

#### 2.3.0 2026-04-13 调研结论

- [ ] 明确系统边界：
  说明：本服务负责“应用层可命中性诊断与前缀稳定化优化”，不负责生产“下游真实 KV cache 命中监控”
- [ ] 明确应用层命中语义：
  说明：`predicted hit/miss` 只代表规范化后的稳定前缀是否更可能被复用，属于应用层诊断信号，不等同于任何下游真实 KV 命中结果
- [ ] 应用层观测设计坚持“计数器优先，不直接存 hit rate”：
  说明：优先记录 `eligible / admitted / reusable / miss_reason / segment_churn` 等应用层计数器，比例指标交给 PromQL、dashboard 或 replay 计算
- [ ] 指标标签保持低基数：
  说明：Prometheus 不适合直接挂 `prefix_hash / session_id / tenant_id` 这类高基数字段；热点前缀与 TopN 应通过 Redis 排行、debug 导出或离线报表承载
- [ ] 命中优化必须围绕“精确前缀一致性”：
  说明：静态内容要前置、动态内容要后置；`tools / system / memory / rag / active_turn` 任何前缀段变化都可能破坏应用层可复用性判断
- [ ] 应用层诊断必须保持可解释：
  说明：继续依赖 `combined/system/memory/rag hash`、`miss_reason`、`segment_reason`、`normalization_version` 做可解释诊断，但不要把它误写成真实 KV 命中率

#### 2.3.1 应用层可命中性诊断计划

- [ ] 定义统一命中分类枚举
  说明：至少区分 `predicted_hit`、`predicted_miss`、`partial_reusable`、`unstable_prefix`、`unknown`
- [ ] 在 `RefineResponse.metadata` 增加观测层级字段
  说明：补 `cache_observation_level`、`cache_prediction_result`、`predicted_reusable_tokens`、`segment_churn_reason`
- [ ] 增加“可缓存资格 -> admission -> lookup”应用层漏斗指标
  说明：我方先聚焦 `ineligible`、`short_prefix`、`low_value_prefix`、`admitted`、`created`、`predicted_reusable`
- [ ] 增加 segment churn 计数器
  说明：分别统计 `system / memory / rag / normalization / model` 导致的稳定前缀破坏次数，直接服务优化优先级
- [ ] 增加 predicted reusable tokens 统计
  说明：记录理论可复用 token 规模，用于比较不同 canonicalize / layout 策略的优化收益
- [ ] 增加 top miss waterfall / top segment churn 诊断面板
  说明：面板重点展示“哪类失稳在增多”“哪一段最破坏稳定前缀”，服务应用层治理优先级

#### 2.3.2 应用层命中优化计划

- [ ] 增加稳定前缀布局版本治理
  说明：显式管理 `prompt_layout_version`，避免布局变化导致命中回退却难以定位
- [ ] 继续清洗高抖动字段并补 deterministic serialization
  说明：重点检查 JSON key 顺序、tool schema 顺序、URL/path/source 标准化、时间戳/随机 ID/trace 字段
- [ ] 为 `tools / system / memory / rag` 增加更严格的边界与 diff 视图
  说明：便于识别究竟是哪一层引入了不稳定变化
- [ ] 扩展 prewarm 策略
  说明：从固定 system prompt 扩展到固定 tools、固定 memory 模板、固定 RAG 模板，并记录命中收益
- [ ] 增加 cache-aware replay 数据集
  说明：为典型 workload 固化 `system/tool/memory/rag` 组合样本，比较 canonicalize 前后与 layout 变更前后的命中改善
- [ ] 在 `dry_run / explain` 中补 cache optimization 建议
  说明：返回“建议前置哪些内容、哪些字段高抖动、哪一段最值得稳定化”

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
- [ ] 增加历史请求 replay 工具，用于离线估计 predicted reusable ratio 并比较不同策略的优化收益
- [ ] 增加“哪个 processor 破坏稳定前缀最多”的分析
- [ ] 增加 predicted reuse / miss reason dashboard
- [ ] 增加应用层 churn / miss 异常可见性与可选告警

### 2.4 其他增强

- [ ] 增加压缩效果评测工具
- [ ] 增加 `log_dedup`
- [ ] 增加 `tool_output_focus`
- [ ] 增加 `rag_rerank_trim`

### 2.5 当前建议顺序

1. 先补 `最小测试护栏（service mapping / summary / config）`
2. 再定义 `应用层预测` 的语义、metadata 字段和解释输出
3. 再补 `低基数 metrics + miss/churn 诊断面板`
4. 再补 `Explain / dry-run / cache debug`
5. 再补 `cache-aware replay / canonicalize-layout 对比` 评测
6. 再补 `Refine / PageIn / Redis / worker` 集成测试与最小运行闭环
7. 最后继续 `摘要链路升级与高级 Processor`

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
