# Context Refiner 实施计划与路线图

- 文档版本：`v2026.04.19`
- 更新日期：`2026-04-19`
- 文档类型：`Roadmap / Plan`
- 适用代码基线：`main` 分支当前实现

## 1. 当前阶段结论

基于当前代码和文档同步结果，项目当前处于：

`核心底盘已成型，应用层 KV A-D 已落地，正在补测试 / Explain / 评测 / 摘要升级这四条产品化主线`

换句话说：

- 分层、主链、prefix cache、观测底座已经不是“概念草图”
- 但 explain/debug、测试闭环、离线评测、真实摘要 provider 仍未完成

## 2. 当前已经完成的里程碑

### 2.1 工程结构与主链

- `controller / service / mapper / dto / domain / support / observability / bootstrap / tests` 已收敛为当前目录结构
- `Refine / PageIn / SummaryWorker` 主链已稳定
- `pipeline + registry + processor` 主链已打通
- `support` 已吸收原独立接入实现，当前按 `redis / summary / tempo` 分类

### 2.2 core 组件化

- `TextSanitizer` 已统一文本清洗入口、顺序和清洗报告
- `RAGNormalizer` 已统一 RAG 排序、规范化与去抖
- `PromptComponent` 已统一 prompt section 组装与 stable prefix section 生成
- `FragmentTransformer / ChunkMetadataHelper` 已抽成独立组件
- processor 当前已按 `stage_01` 到 `stage_04` 聚合，职责更偏编排层

### 2.3 应用层 KV 与摘要底座

- 应用层 KV A/B/C/D 已完成：规范化增强、分层 prefix 身份、miss reason、admission/TTL/prewarm 策略
- `SummaryProvider` 抽象、启发式 provider、结构化 `SummaryArtifact` 已落地
- `PageIn` 已优先返回有效 summary artifact，失效时回退原 page

### 2.4 质量与观测现状

- 已有 Prometheus / Tracing / Grafana / Tempo 基线
- 已有少量单测起步，但还未覆盖主链
- 文档体系已切分为 architecture / code / principles / plan / todo / test plan

## 3. 当前最高优先级事项

### 3.1 测试闭环

当前缺口：

- `service / mapping / summary / config` 还没有最小单测闭环
- `Refine / PageIn / Redis / worker` 还没有稳定的集成测试

目标：

- 先形成最小回归护栏
- 再让后续 explain、清洗规则、摘要升级有可验证基础

### 3.2 Explain / Debug / 评测

当前缺口：

- 还没有 `dry_run / explain / cache debug`
- 还没有 replay 驱动的离线评测
- 还没有“预测可复用”与真实命中效果的系统对照

目标：

- 把应用层 prefix cache 诊断从“metadata 已存在”升级成“能解释、能回放、能比较”

### 3.3 摘要链路升级

当前缺口：

- 真实外部摘要 provider 未接入
- worker retry / claim 策略未补
- chunk 级摘要索引未完成

目标：

- 保持主链可降级
- 在不破坏现有启发式闭环的前提下接入更真实的摘要能力

### 3.4 基础清洗与入库预处理

当前缺口：

- `TextSanitizer` 解决的是在线清洗统一入口，不是完整离线预处理管线
- sentence-aware chunking、文档标题层级识别、代码 cleaner SDK 仍未建立

目标：

- 把“请求时清洗”扩展到“入库前结构化预处理”

## 4. 推荐分阶段推进

### Phase 2：质量闭环

重点：

- 补最小单测
- 补最小集成测试
- 补最小本地运行与配置说明

验收标准：

- `service / mapping / summary / config` 有回归护栏
- `Refine / PageIn / worker` 至少有一条集成主链可验证

### Phase 3：Explain / 评测闭环

重点：

- `dry_run`
- `explain metadata`
- `cache debug`
- replay 数据集与离线比较工具

验收标准：

- 能解释 prefix 为什么命中或 miss
- 能比较不同清洗 / layout 策略的收益差异

### Phase 4：摘要链路升级

重点：

- 接真实外部 provider
- 补 retry / claim
- 推进 chunk 级摘要索引

验收标准：

- 外部 provider 故障不影响主链返回
- 摘要 artifact 具备稳定版本与失效治理

### Phase 5：入库预处理与更深层缓存语义

重点：

- sentence-aware chunking
- 标题层级 / 章节边界识别
- 代码 cleaner SDK
- 四段 prefix 与 segment-level 部分复用

验收标准：

- 文档/RAG 入库质量可单独治理
- 应用层 prefix 预测更接近真实 serving 复用语义

## 5. 推荐执行顺序

1. 先补 `service mapping / summary / config` 最小单测闭环
2. 再补 `Refine / PageIn / Redis / worker` 集成测试与最小本地运行说明
3. 再补 `dry_run / explain / cache debug / normalized preview`
4. 再补 replay、dashboard、prefix churn 与 miss waterfall 评测闭环
5. 再接真实外部摘要 provider，并补 retry / claim
6. 再推进 chunk 级摘要索引与更细粒度生命周期治理
7. 再推进 sentence-aware chunking、文档预处理与代码 cleaner SDK
8. 最后推进四段 prefix 与 segment-level 部分复用

## 6. 当前最大的风险

### 风险 1：继续堆新规则，但没有测试闭环

后果：

- 无法证明策略变更真的提升收益
- 一旦伤语义，很难回滚和定位

### 风险 2：把启发式摘要底座误判为摘要产品化完成

后果：

- 真实业务质量不可控
- 生命周期与 provider 演进空间不足

### 风险 3：只有应用层预测，没有评测与对照

后果：

- 会高估 prefix cache 优化的真实收益
- 很难判断哪些规范化动作值得保留

## 7. 结论

当前最合理的推进方式不是继续做“更多处理步骤”，而是：

1. 先把测试、Explain、评测这条证据链补齐
2. 再把摘要链路从启发式底座升级成可演进能力
3. 最后再继续做更深的入库预处理和缓存语义优化

这也是接下来文档、代码和任务排序应共同遵循的主线。
