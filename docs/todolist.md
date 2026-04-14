# Context Refiner Todo / Snapshot

- 文档版本：`v2026.04.13`
- 更新日期：`2026-04-13`
- 文档类型：`Snapshot / Todo / Archive`
- 适用代码基线：`main` 分支当前实现

## 1. 当前快照

### 1.1 项目状态

- 当前阶段：`核心底盘已成型，应用层 KV 命中优化已完成到 D，工程目录重构与错层清理已完成，已进入 Explain 与测试扩展阶段`
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
- 已完成应用边界收口：
  说明：已移除越界的 `downstream_kv_reuse_hint` 输出，并把内部优化目标语义收敛为应用层 `prefix-reuse-stability`
- 已完成应用层预测 metadata 首轮落地：
  说明：`RefineResponse.metadata` 已补 `cache_observation_level`、`cache_prediction_result`、`predicted_reusable_tokens`、`segment_churn_reason`
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
- 当前目录结构与旧层级残留已完成收敛：
  说明：已落地 `controller / service / mapper / dto / domain / adapter / observability / bootstrap / tests`，`dto` 已接入 service 内部调用链，空壳旧目录与主要错层文档引用也已清理
- 当前 summary worker 仍为启发式摘要
- 当前摘要回填仍偏 page 级：
  说明：已升级为结构化 `SummaryArtifact`，但当前仍按 page key 挂载与读取，尚未做独立 chunk 级摘要索引
- 当前虽然已把应用层“可命中性预测”落到 `response metadata`，但还没有接入 `explain / dry_run / metrics / dashboard`
- 当前还没有 `dry_run / explain / cache debug`
- 当前虽然已有基础 overview dashboard，但还没有离线 replay / prefix churn / top miss dashboard / alerting

## 2. 未完成任务

### 2.0 P0 工程目录结构重构

- [x] 明确接近 Java 风格的目标目录骨架
  说明：目录目标收敛为 `cmd / api / config / docs / deploy / internal / tests`，其中 `internal` 内优先形成 `controller / service / mapper / dto / domain / adapter / support / observability / bootstrap` 这组职责分层
- [x] 明确应用入口层的 OO 拆分方式
  说明：对外入口统一按 `controller -> service -> mapper -> dto` 分层；`controller` 只收请求与返回响应，`service` 只编排用例，`mapper` 负责协议对象和领域对象转换，`dto` 负责入参/出参载体
- [x] 拆出独立测试目录
  说明：把当前散落的测试视角收敛为 `tests/unit`、`tests/integration`、`tests/e2e`，实现文件旁只保留极少数必须贴身的微型单测
- [x] 把核心链路代码收拢到一处
  说明：将 `pipeline / registry / prefix identity / prompt segmentation / processors` 这条主链收敛到 `domain` 或 `core-domain` 模块，不再散落在 service、bootstrap、infra 之间
- [x] 把观测与监控代码收拢到一处
  说明：将 `prometheus / tracing / dashboard wiring / metrics recorder` 收敛为 `observability` 模块族，避免 `internal/observability` 与 `internal/infra/observability` 分裂
- [x] 把适配层代码按入站/出站拆分
  说明：`grpc / redis / future external provider` 这类适配代码按 `adapter/inbound` 与 `adapter/outbound` 归位，避免协议层、存储层、业务层混放
- [x] 把公共支持代码按共性能力拆分
  说明：`heuristic / tokenizer / shared helper` 这类非业务主链代码收敛到 `support` 模块，避免被误放进核心链路
- [x] 把启动装配与运行时 wiring 收拢到一处
  说明：将 `bootstrap / runtime / processors assembly` 归入统一的 `bootstrap` 或 `platform/bootstrap` 目录，避免应用逻辑与装配逻辑混放
- [ ] 统一文件夹与文件命名规范
  说明：目录名全小写；文件按职责后缀命名，如 `refine_controller.go`、`refine_service.go`、`request_mapper.go`、`refine_request_dto.go`、`prometheus_recorder.go`、`redis_page_repository.go`，避免 `auto.go`、`counter.go` 这类含糊命名
- [ ] 分阶段迁移，禁止一次性大搬家
  说明：按“先定分层 -> 再拆测试目录 -> 再迁 controller/service/mapper/dto -> 再迁 domain -> 再迁 observability/adapter/bootstrap -> 最后清理 import 与命名”的顺序推进，确保每一步都可编译可回归

### 2.1 P0 产品化闭环

- [ ] 补 `service mapping / summary / config` 最小单元测试闭环
- [ ] 补 `Refine` / `PageIn` / Redis / worker 集成测试
- [ ] 补最小本地运行方案
- [ ] 清理配置占位与启动说明
- [ ] 补 cache-aware 回归测试

### 2.2 P1 摘要链路升级

- [x] 抽象 `SummaryProvider`
  说明：`summary worker` 已切到 `Provider -> SummaryArtifact -> Redis store` 链路，当前默认实现仍为启发式 provider
- [ ] 接真实外部摘要模型
- [x] 设计结构化 `SummaryArtifact`
  说明：已落地 `artifact_id / content_hash / fragment_types / provider / provider_version / schema_version / created_at / expires_at`，并在 worker、Redis 存储和 `PageIn` protobuf 返回链路中生效
- [-] 增加 summary 失效与重试策略
  说明：`content_hash / schema_version / provider_version / expires_at` 失效已完成，读取失效 artifact 时会删除并回退原 page；worker retry 仍待补
- [ ] 支持 chunk 级 summary artifact

### 2.3 应用层 KV 下一阶段

#### 2.3.0 2026-04-13 调研结论

- [x] 明确系统边界：
  说明：本服务负责“应用层可命中性诊断与前缀稳定化优化”，不负责生产“下游真实 KV cache 命中监控”
- [x] 明确应用层命中语义：
  说明：`predicted hit/miss` 只代表规范化后的稳定前缀是否更可能被复用，属于应用层诊断信号，不等同于任何下游真实 KV 命中结果
- [x] 应用层观测设计坚持“计数器优先，不直接存 hit rate”：
  说明：优先记录 `eligible / admitted / reusable / miss_reason / segment_churn` 等应用层计数器，比例指标交给 PromQL、dashboard 或 replay 计算
- [x] 指标标签保持低基数：
  说明：Prometheus 不适合直接挂 `prefix_hash / session_id / tenant_id` 这类高基数字段；热点前缀与 TopN 应通过 Redis 排行、debug 导出或离线报表承载
- [x] 命中优化必须围绕“精确前缀一致性”：
  说明：静态内容要前置、动态内容要后置；`tools / system / memory / rag / active_turn` 任何前缀段变化都可能破坏应用层可复用性判断
- [x] 应用层诊断必须保持可解释：
  说明：继续依赖 `combined/system/memory/rag hash`、`miss_reason`、`segment_reason`、`normalization_version` 做可解释诊断，但不要把它误写成真实 KV 命中率

#### 2.3.1 应用层可命中性诊断计划

- [x] 定义统一命中分类枚举
  说明：当前已以 metadata string enum 形式落地 `predicted_hit`、`predicted_miss`、`partial_reusable`、`unstable_prefix`、`unknown`
- [x] 在 `RefineResponse.metadata` 增加观测层级字段
  说明：已补 `cache_observation_level`、`cache_prediction_result`、`predicted_reusable_tokens`、`segment_churn_reason`
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

1. 先补 `service mapping / summary / config` 最小单测闭环
2. 再补 `Refine / PageIn / Redis / worker` 集成测试，并继续收拢到 `tests/integration`
3. 然后推进 `Explain / dry-run / cache debug`，把预测结果做成可解释输出
4. 再补应用层低基数 metrics、dashboard 与离线 replay
5. 最后继续 `真实摘要 provider`、`summary artifact`、`重试与失效策略`

### 2026-04-13 / 工程目录重构第一阶段已完成

- [x] 完成第一轮目录骨架归位
  说明：已形成 `internal/controller`、`internal/service`、`internal/mapper`、`internal/dto`、`internal/domain`、`internal/adapter/outbound`、`internal/observability`、`internal/support`、`internal/bootstrap`
- [x] 完成核心链路迁移
  说明：`pipeline / registry / prefix cache / processor / repository contract` 已迁入 `internal/domain/core`
- [x] 完成适配层与观测层归位
  说明：`grpc controller` 迁入 `internal/controller/grpc`，`redis / summary worker` 迁入 `internal/adapter/outbound`，`prometheus / tracing` 迁入 `internal/observability`
- [x] 完成 mapper 独立拆包
  说明：`request / response mapping` 已从 `service` 拆到 `internal/mapper`，当前由 `service` 显式依赖
- [x] 完成测试目录落地
  说明：已创建 `tests/unit`、`tests/integration`、`tests/e2e` 及说明文件
- [x] 完成 import 回收与编译恢复
  说明：已统一修复迁移后的 import 路径，`go test ./...` 当前可通过

### 2026-04-13 / 工程目录重构第二阶段已完成

- [x] 完成 dto 内部边界落地
  说明：已新增 `internal/dto` 下的 `RefineRequest / RefineResponse / PageInRequest / PageInResponse`，并让 `service` 内部主链切到 dto
- [x] 保持外部 contract 稳定
  说明：`pkg/service` 与 gRPC protobuf 接口保持不变，仅在 `controller/service/mapper` 内部新增 proto <-> dto <-> domain 转换
- [x] 完成一轮命名规范收尾
  说明：已落地 `refine_service.go`、`refine_request_mapper.go`、`refine_response_mapper.go`、`mapper_helper.go`、`token_counter.go`、`auto_compact_processor.go`
- [x] 在 `tests/unit` 落地第一组跨模块单测
  说明：已新增 mapper/dto 边界单测，覆盖 `proto -> dto -> domain`、`domain -> dto -> proto`、`pagein dto -> proto`
- [x] 保持编译与回归通过
  说明：`go test ./...` 当前可通过

### 2026-04-13 / 命名规范收尾第一轮已完成

- [x] 完成 processor 目录第一轮职责化命名
  说明：已统一收敛为 `*_processor.go` 或 `*_helper.go`，包括 `assemble_processor.go`、`paging_processor.go`、`structured_processors.go`、`request_clone_helper.go`、`token_split_helper.go`、`chunk_metadata_helper.go`
- [x] 完成 adapter/outbound 第一轮职责化命名
  说明：已落地 `redis_repository.go`、`summary_worker.go`、`heuristic_summarizer.go`
- [x] 完成 domain 契约与前缀身份文件命名收尾
  说明：已落地 `prefix_cache_identity.go`、`repository_contracts.go`
- [x] 完成相关文档与说明路径同步
  说明：`Agent.md`、`layered-architecture`、`code-design`、`learning-guide`、`test-plan`、`context-refiner-design` 已追平新文件名
- [x] 保持编译与测试通过
  说明：`go test ./...` 当前可通过

### 2026-04-13 / 旧目录残留与错层引用清理已完成

- [x] 清理空壳旧目录
  说明：已删除 `internal/adapter/grpc`、`internal/core`、`internal/infra/store` 及其下游空壳旧目录，避免新旧结构并存
- [x] 清理主要文档中的错层路径
  说明：已同步修正 `Agent.md`、`code-design`、`context-refiner-design`、`learning-guide`、`test-plan` 中把旧目录当成当前结构的描述
- [x] 完成结构复查与回归验证
  说明：已确认 `internal` 与 `tests` 下无空目录残留，且 `go test ./...` 当前可通过

## 3. 已完成任务归档

### 2026-04-13 / 应用边界收口已完成

- [x] 删除越界的下游复用提示输出
  说明：已删除 `downstream_kv_reuse_hint` metadata / tracing 输出，避免把应用层结果误表述为下游 KV 复用提示
- [x] 收紧内部优化目标语义
  说明：已将 `cache_optimization_target` 默认值从 `prefix-hit-rate` 收敛为应用层 `prefix-reuse-stability`
- [x] 保持 prefix cache 主链不受影响
  说明：保留 `prefix hash / miss reason / segment reason / prompt_layout_version` 等应用层必要诊断字段，`go test ./...` 当前可通过

### 2026-04-13 / 应用层预测 metadata 已完成

- [x] 落地应用层预测观测字段
  说明：`PrefixCacheProcessor` 统一补齐 `cache_observation_level`、`cache_prediction_result`、`predicted_reusable_tokens`、`segment_churn_reason`
- [x] 落地预测结果分类语义
  说明：当前已覆盖 `predicted_miss / partial_reusable / predicted_hit / unstable_prefix / unknown` 这组应用层预测结果
- [x] 增加最小单测护栏
  说明：新增 `prefix_cache` 单测，覆盖 `skipped / created / hit` 三条预测路径，`go test ./...` 当前可通过

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
