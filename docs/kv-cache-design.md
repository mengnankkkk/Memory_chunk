# Context Refiner KV Cache 设计与实现

- 文档版本：`v2026.04.14`
- 更新日期：`2026-04-14`
- 文档类型：`Design / Implementation Analysis`
- 适用代码基线：`main`

## 1. 先说边界

这份文档讨论的 `KV cache`，不是推理引擎内部真正的 token block KV cache。

当前仓库实现的是：

- 应用层 `stable prefix` 规范化
- 应用层 prefix 身份计算
- 应用层“可命中性”诊断
- 应用层 prefix registry / TTL / hot prefix 管理

它解决的是：

- 如何让上游 prompt 更稳定
- 如何让跨请求的稳定前缀更容易复用
- 如何解释“为什么这一轮比上一轮更可能命中”

它不直接解决：

- 模型服务内部的真实 KV block 分配
- 下游 serving engine 的显存管理
- 真正意义上的“模型层 hit rate”采样

换句话说：

`这里做的是 application-layer KV cache readiness，不是 model-runtime KV cache management。`

## 2. 为什么要单独设计这一层

如果只把 prompt 拼出来直接发给模型，下游 KV cache 是否命中几乎完全不可解释。

当前系统单独引入应用层 KV 设计，主要是为了解决四个问题：

1. 稳定内容和高抖动内容没有分层，导致前缀复用机会被浪费。
2. 即使 miss，也不知道是 `system`、`memory` 还是 `rag` 哪一段变了。
3. 没有 admission policy，低价值短前缀也会污染诊断结果。
4. 没有 namespace / TTL / hot prefix 机制，跨租户、跨 policy、跨模型的复用边界会混乱。

## 3. 设计目标

当前实现围绕五个目标展开：

1. 先把 prompt 中“稳定的部分”收拢出来。
2. 对稳定部分做 deterministic normalization，减少非语义抖动。
3. 给稳定部分生成分层 hash，保留可解释诊断能力。
4. 用 Redis 持久化 prefix registry，支持 TTL、热点统计和预热。
5. 把结果回写到 `RefineResponse.metadata`，让应用层可解释。

## 4. 核心思路

### 4.1 分段

当前稳定前缀被拆成三段：

- `system`
- `memory`
- `rag`

动态内容，也就是当前活跃输入，不进入稳定前缀身份计算。

对应实现见：

- [prefix_cache_identity.go](/E:/github/Memory_chunk/internal/domain/core/prefix_cache_identity.go)

### 4.2 规范化

规范化目标不是“美化文本”，而是清洗会破坏前缀稳定性的高抖动字段。

当前已落地的规则包括：

- 时间戳归一化
- UUID / 长 hex id 归一化
- `request_id / session_id / trace_id` 等字段归一化
- URL 去掉 query / fragment
- 路径与 source label 稳定化
- JSON key 排序与 volatile key 剔除
- `RAGChunk / sources` 稳定排序

这部分实现主要在：

- [prefix_cache_identity.go](/E:/github/Memory_chunk/internal/domain/core/prefix_cache_identity.go)

### 4.3 分层身份

当前不会只生成一个总 hash，而是同时保留：

- `combined_prefix_hash`
- `system_prefix_hash`
- `memory_prefix_hash`
- `rag_prefix_hash`

以及对应 token 数：

- `stable_prefix_tokens`
- `system_prefix_tokens`
- `memory_prefix_tokens`
- `rag_prefix_tokens`

这样做的价值是：

- 命中时可以知道命中的是哪套稳定前缀
- miss 时可以知道更可能是哪一层变化导致

## 5. 请求时序

当前主链在 `PrefixCacheProcessor` 中执行：

1. 基于当前请求构建 `PrefixCacheIdentity`
2. 将 hash / token / normalization version 回写到 metadata
3. 计算 namespace
4. 做 admission 判断
5. admission 通过后写入 Redis registry
6. 根据 Redis 返回结果计算 `hit / created / ttl_expired / hash_changed / model_changed`
7. 继续派生应用层预测字段

核心实现：

- [prefix_cache_processor.go](/E:/github/Memory_chunk/internal/domain/core/processor/prefix_cache_processor.go)

## 6. Admission Policy

当前 admission 不是“所有前缀都登记”，而是做了第一层过滤：

- `empty`
- `short_prefix`
- `low_value_prefix`

对应配置项：

- `prefix_cache.min_stable_prefix_tokens`
- `prefix_cache.min_segment_count`

只有满足最小 token 和最小 segment 数量的前缀，才会进入 Redis registry。

这样做的原因是：

- 很短的前缀即使稳定，也通常没有复用价值
- 单一弱 segment 的前缀容易造成误判

## 7. Namespace 设计

Prefix key 不是全局混用，而是可以按三类维度拼 namespace：

- `tenant`
- `policy`
- `model`

当前配置项：

- `prefix_cache.namespace.include_tenant`
- `prefix_cache.namespace.include_policy`
- `prefix_cache.namespace.include_model`

对应实现：

- [prefix_cache_identity.go](/E:/github/Memory_chunk/internal/domain/core/prefix_cache_identity.go)
- [prefix_cache_processor.go](/E:/github/Memory_chunk/internal/domain/core/processor/prefix_cache_processor.go)

目的很直接：

- 不让不同模型误共用前缀身份
- 不让不同策略混淆
- 给多租户场景留下边界

## 8. Redis 存储实现

当前 Redis 侧并不是存“真正可复用的模型 KV block”，而是存 prefix registry 记录。

关键结构定义在：

- [repository_contracts.go](/E:/github/Memory_chunk/internal/domain/core/repository/repository_contracts.go)

字段核心包括：

- `namespace`
- `model_id`
- `prefix_hash`
- `system_prefix_hash`
- `memory_prefix_hash`
- `rag_prefix_hash`
- `stable_prefix_tokens`
- `prompt_layout_version`
- `artifact_key_version`
- `normalization_version`
- `cache_tier`
- `admission_decision`
- `applied_ttl_seconds`
- `hot`
- `hot_score`
- `created_at`
- `last_seen_at`
- `hit_count`

Redis 实现位于：

- [redis_repository.go](/E:/github/Memory_chunk/internal/adapter/outbound/redis/redis_repository.go)

## 9. Key 设计

当前 prefix registry 主要有三类 key：

### 9.1 namespace 级 prefix key

格式近似：

```text
{keyPrefix}:prefix:{namespace}:{prefix_hash}
```

用途：

- 存当前 prefix registry 主记录

### 9.2 session scope key

格式近似：

```text
{keyPrefix}:prefix-session:{namespace|session_id}
```

用途：

- 记录最近一次该 session scope 下的前缀状态
- 用于诊断 `ttl_expired / hash_changed / model_changed`

### 9.3 hot prefix zset

格式近似：

```text
{keyPrefix}:prefix-hot:{namespace}
```

用途：

- 统计热点前缀
- 给 hot tier TTL 与观察面板提供基础数据

## 10. Miss Reason 诊断

当前 miss 不是只有一个布尔值，而是拆成两层：

### 10.1 一级 miss reason

- `created`
- `ttl_expired`
- `hash_changed`
- `model_changed`

### 10.2 二级 segment reason

- `system_changed`
- `memory_changed`
- `rag_changed`
- `normalization_changed`
- `combined_changed`
- `cold_start`

这套设计的目的，是把“没命中”从黑盒变成可解释事件。

## 11. Hot Prefix 与 TTL 分层

当前 prefix registry 有两层 TTL：

- `default`
- `hot`

对应配置：

- `redis.prefix_cache_ttl`
- `prefix_cache.hot_threshold`
- `prefix_cache.hot_ttl`

命中次数达到阈值后：

- `hit_count` 增长
- 进入 `hot` tier
- 使用更长 TTL
- 同步更新 `hot_score`

这部分逻辑在：

- [redis_repository.go](/E:/github/Memory_chunk/internal/adapter/outbound/redis/redis_repository.go)

## 12. Prewarm 机制

当前支持在启动时对固定模板做 prefix prewarm。

配置位于：

- `prefix_cache.prewarm`

每个 prewarm 项可提供：

- `name`
- `model_id`
- `policy`
- `tenant`
- `system_prompt`
- `memory_prompt`
- `rag_prompt`

启动阶段会：

1. 对这些固定段构建 `PrefixCacheIdentity`
2. 计算 namespace
3. 直接写入 Redis prefix registry

实现见：

- [runtime.go](/E:/github/Memory_chunk/internal/bootstrap/runtime.go)

## 13. 应用层预测字段

当前 `RefineResponse.metadata` 里已经有一批应用层预测字段：

- `cache_observation_level`
- `cache_prediction_result`
- `predicted_reusable_tokens`
- `segment_churn_reason`

这里的语义不是“真实模型命中”，而是：

`规范化后的稳定前缀从应用层视角看，更可能被复用到什么程度。`

## 14. 为什么当前设计是合理的

这套设计的关键优点有五个：

1. 不依赖下游 serving engine 暴露内部指标，也能先把上游 prompt 稳定化。
2. 命中与 miss 都可解释，不再只是 hit/miss 黑盒。
3. namespace、TTL、hot prefix、prewarm 已经具备工程化最小闭环。
4. metadata 已回流到 `RefineResponse`，上层可以继续做 explain / dashboard。
5. Redis 结构仍然简单，主要靠 string + zset，不依赖额外模块。

## 15. 当前已知限制

当前实现仍有几个重要限制：

1. 这是应用层 readiness 诊断，不等同于模型层真实 KV cache 命中率。
2. 还没有 `dry_run / explain` API，把这些字段做成单独的解释输出。
3. 还没有离线 replay，把不同 normalization/layout 策略系统化比较。
4. 热点 prefix 统计还没有 TopN dashboard 和告警闭环。
5. 当前 summary artifact 与 prefix registry 是两套机制，尚未形成更深层的统一 artifact graph。

## 16. 与 SummaryArtifact 的关系

两者都属于“跨请求可复用状态”，但职责不同：

- prefix registry 管的是“这一段稳定前缀是否值得复用、为什么 miss”
- summary artifact 管的是“page-out 之后可回填的摘要对象”

当前它们共享的设计原则是：

- 尽量 content-addressed
- 尽量结构化对象而不是裸字符串
- 尽量带版本字段与 TTL
- 尽量让失效原因可解释

补充一点：

- `PageIn` 现在仍保留兼容的 `content / is_summary / summary_job_id`
- 同时已经把结构化 `summary_artifact` 直接透出到 protobuf，便于上层做 explain、debug 和更细粒度展示

## 17. 后续最值得继续做的点

从全局收益看，后续最值得继续补的是：

1. `dry_run / explain / cache debug`
2. churn / miss dashboard
3. replay 数据集与离线评测
4. 更细粒度的 segment diff 视图
5. 结合真实 serving engine 指标做对照验证

## 18. 一句话总结

当前 KV cache 设计的本质是：

`先在应用层把 prompt 稳定化、结构化、可解释化，再为下游真实 KV cache 命中创造条件。`
