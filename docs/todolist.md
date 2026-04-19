# Context Refiner Todo / Snapshot

- 文档版本：`v2026.04.19`
- 更新日期：`2026-04-19`
- 文档类型：`Snapshot / Todo / Archive`
- 适用代码基线：`main` 分支当前实现

## 1. 当前快照

### 1.1 项目状态

- 当前阶段：`核心底盘已成型，应用层 KV A-D 已落地，工程目录重构已整体关闭，正处于 Explain / 测试 / 评测扩展阶段`
- 当前主要定位：`应用层上下文清洗与稳定化接口，不负责模型层 KV block 管理`
- 当前最关键缺口：`测试闭环`、`Explain / dry-run / cache debug`、`离线 replay 评测`、`真实摘要 provider`、`前缀四段分层与 segment-level 部分复用`

### 1.2 当前已确认能力

- 已有 `gRPC Refine / PageIn` 主链
- 已有 `Pipeline-Processor` 可插拔处理链
- 已有 Redis `page artifact / summary job / prefix cache registry`
- 已有 Prometheus / Tracing / Grafana / Tempo 基础观测
- 已有应用层 `stable prefix` 规范化与分层 prefix hash
- 已有应用层 `miss reason` 诊断
- 已完成 `core/components` 首轮组件化收口：
  说明：`TextSanitizer / RAGNormalizer / PromptComponent` 以及 `FragmentTransformer / ChunkMetadataHelper` 已独立沉到 `internal/domain/core/components`，processor 改为调用组件执行，`prefix_cache_identity` 的 stable prefix section 拼接已委托 Prompt 组件
- 已完成接入实现并入 `support`：
  说明：当前 `support` 下已形成 `support/redis`、`support/summary`、`support/tempo`，同时承载通用支撑与按接入对象分类的实现
- 已有应用层 cache 策略：
  说明：`admission policy`、`namespace`、`TTL 分层`、`热点前缀统计`、`prewarm`
- 已完成应用边界收口：
  说明：已移除越界的 `downstream_kv_reuse_hint` 输出，并把内部优化目标语义收敛为应用层 `prefix-reuse-stability`
- 已完成统一输入结构升级：
  说明：HTTP / gRPC / dashboard evaluation 已统一围绕 `system + messages + memory` 工作；`memory.rag` 成为外部与内部共享的记忆入口，旧 `rag`、旧 `rag_chunks` 与旧式 `role=system` 兼容桥接已删除
- 已完成应用层预测 metadata 首轮落地：
  说明：`RefineResponse.metadata` 已补 `cache_observation_level`、`cache_prediction_result`、`predicted_reusable_tokens`、`segment_churn_reason`
- 已有结构化 `SummaryArtifact`，worker 已切到 `Provider -> SummaryArtifact -> Redis store` 链路
- 已完成工程目录整体重构（`controller / service / mapper / dto / domain / support / observability / bootstrap / tests`）
- 已有少量单元测试起步，`go test ./...` 当前可通过

### 1.3 当前应用层 KV 优化完成度

- A 规范化增强：`已完成`
- B 分层 Prefix 身份（system / memory / rag 三段）：`已完成`
- C Miss Reason 诊断：`已完成`
- D 应用层 Cache 策略：`已完成`
- E Explain / Debug 能力：`未完成`
- F 观测与评测闭环：`未完成`
- G 前缀四段分层（tools 独立）与 segment-level 部分复用：`未开始`
  说明：2026-04-18 新增的下一阶段方向，详见 2.3.2 与 2.3.3

### 1.4 当前仍未完成的关键能力

- 当前已有少量 `_test.go` 文件，但还没有形成覆盖主链的测试闭环
- 当前默认配置仍为占位值，不能直接开箱启动
- 当前 summary worker 仍为启发式摘要
- 当前摘要回填仍偏 page 级：
  说明：已升级为结构化 `SummaryArtifact`，但当前仍按 page key 挂载与读取，尚未做独立 chunk 级摘要索引
- 当前虽然已把应用层"可命中性预测"落到 `response metadata`，但还没有接入 `explain / dry_run / metrics / dashboard`
- 当前还没有 `dry_run / explain / cache debug`
- 当前虽然已有基础 overview dashboard，但还没有离线 replay / prefix churn / top miss dashboard / alerting
- 当前前缀分段仍为 `system / memory / rag` 三段，`tools` schema 被并入 system，变更时会触发误判
- 当前 miss 诊断仍是"整条前缀"粒度，尚无 segment-level 部分复用语义
- 当前 active turn 仅做轻量规范化，多轮同话题下的高抖动字段去抖潜力仍未被挖掘
- 当前基础清洗虽已完成首轮组件化，但仍更偏"在线 prompt 规范化与压缩"而非"离线入库预处理"：
  说明：已形成独立 `TextSanitizer / RAGNormalizer / PromptComponent`，并补齐 HTML/XML/emoji/Unicode 噪音清洗与统一清洗报告；但文档预处理管线、sentence-aware chunking 与代码 cleaner SDK 仍未建立

## 2. 未完成任务

### 2.1 P0 产品化闭环

- [ ] 补 `service mapping / summary / config` 最小单元测试闭环
- [ ] 补 `Refine` / `PageIn` / Redis / worker 集成测试
- [ ] 补最小本地运行方案
- [ ] 清理配置占位与启动说明
- [ ] 补 cache-aware 回归测试

### 2.2 P1 摘要链路升级

- [ ] 接真实外部摘要模型
- [-] 增加 summary 失效与重试策略
  说明：`content_hash / schema_version / provider_version / expires_at` 失效已完成，读取失效 artifact 时会删除并回退原 page；worker retry 仍待补
- [ ] 支持 chunk 级 summary artifact

### 2.3 应用层 KV 下一阶段

#### 2.3.1 应用层可命中性诊断计划

- [ ] 增加"可缓存资格 -> admission -> lookup"应用层漏斗指标
  说明：我方先聚焦 `ineligible`、`short_prefix`、`low_value_prefix`、`admitted`、`created`、`predicted_reusable`
- [ ] 增加 segment churn 计数器
  说明：分别统计 `system / memory / rag / normalization / model` 导致的稳定前缀破坏次数，直接服务优化优先级
- [ ] 增加 predicted reusable tokens 统计
  说明：记录理论可复用 token 规模，用于比较不同 canonicalize / layout 策略的优化收益
- [ ] 增加 top miss waterfall / top segment churn 诊断面板
  说明：面板重点展示"哪类失稳在增多""哪一段最破坏稳定前缀"，服务应用层治理优先级

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
  说明：返回"建议前置哪些内容、哪些字段高抖动、哪一段最值得稳定化"
- [ ] **NEW 2026-04-18** 前缀分段从 3 段升级为 4 段（tools 独立）
  说明：当前 `tools` schema 被并入 system，一旦工具集变更就会误触发 `system_changed`。拟拆出独立 `tools` 段，新增 `tools_prefix_hash` 与 `tools_prefix_tokens`，纳入 admission / miss reason / namespace 体系。收益是让 tool schema 抖动不再污染 system 稳定性指标
- [ ] **NEW 2026-04-18** active turn 内部轻量去抖
  说明：active turn 当前只做空白与换行级轻量规范化。用户输入仍常夹带时间戳、粘贴的 URL query、会话 ID 等高抖动片段。拟继续做不改变语义的保守清洗（timestamp / URL query / 粘贴 session id），提升多轮同话题时的前缀延续性。注意边界：绝不改变语义，仅处理可证明无语义影响的抖动字段
- [ ] **NEW 2026-04-18** `prompt_layout_version` 纳入 miss reason 维度
  说明：当前该字段已存在但未参与诊断。拟在 layout 变更时独立归类为 `layout_changed`，与 `normalization_changed` 分开统计，避免架构级版本升级污染日常 churn 观察

#### 2.3.3 应用层缓存语义深化（NEW 2026-04-18）

- [ ] 设计 segment-level 部分复用
  说明：当前整条前缀 miss 即整体重算。拟把 prefix registry 从"整体 hash"升级为"分段 hash 链"，tools/system/memory 段命中即计入 `predicted_reusable_tokens`，即使 rag 段变化也能反映部分复用潜力。这是与下游真实 serving engine（vLLM / SGLang 的 radix tree）最匹配的布局语义
- [ ] 对照下游真实 KV 指标
  说明：如有条件从 vLLM / SGLang / 任意 serving engine 拉回真实 prefix cache hit rate，与应用层 `predicted_reusable_tokens` 做相关性分析。这是把"应用层诊断"升格为"可验证优化"的唯一路径。优先作为可选集成，具体接入方式在调研阶段决定

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
- [ ] 增加"哪个 processor 破坏稳定前缀最多"的分析
- [ ] 增加 predicted reuse / miss reason dashboard
- [ ] 增加应用层 churn / miss 异常可见性与可选告警
- [ ] **NEW 2026-04-18** 应用层预测 vs 下游真实命中的相关性仪表盘
  说明：依赖 2.3.3 的下游 KV 指标接入能力，用于持续校准应用层预测的有效性

### 2.4 基础清洗与入库预处理补齐

- [ ] 为通用文本清洗增加可扩展 Hook
  说明：内置规则之外，需允许业务侧按来源（scraping / log / docs / social）插入自定义 sanitizer rule，避免把项目内通用规则写死
- [ ] 增加“句子切分 -> token chunk”预处理路径
  说明：当前分页主要是按行/按 rune 切分，`SplitSentences` 仅用于摘要器；拟增加面向 RAG 入库的 sentence-aware chunking，避免正文在句中断裂
- [ ] 设计结构化文档预处理管线
  说明：目标链路为 `HTML/Markdown -> 标题层级识别 -> 章节拆分 -> 小 chunk 切分 -> 可选摘要 -> 入库`；当前系统只会消费已经结构化的 fragments，还不能从原始文档自动抽结构
- [ ] 增加文档标题层级与章节边界识别
  说明：支持 `H1/H2/H3`、Markdown 标题、常见知识库模板，保留章节结构，避免技术文档在切块后丢失上下文边界
- [ ] 增加模板/导航/版权等 boilerplate 去重
  说明：当前仅有 chunk 级重复合并，尚未专门识别文档模板、导航栏、版权声明、重复页脚等低价值重复内容
- [ ] 增加正文与代码块分路处理
  说明：当前只有在上游已标注 `FragmentTypeCode` 时才会走 `code_outline`；拟增加从 Markdown/HTML 自动识别 code fence / code block，并与正文分路处理
- [ ] 设计代码上下文清洗器 SDK
  说明：当前代码侧只有 `code_outline`、轻量尾随空格处理和超长片段 `snip`，尚未形成独立代码清洗器抽象
- [ ] 增加多语言安全删注释能力
  说明：拟支持至少 `Go / Java / Python / JavaScript / TypeScript / YAML` 常见语法，安全删除注释并保护字符串、URL、正则、shell 片段不被误伤
- [ ] 增加代码空行压缩与生成代码/重复块识别
  说明：当前没有 AST / parser 级代码清洗，也没有复制块/生成文件识别；拟补低风险重复块识别和生成代码降权/裁剪策略
- [ ] 增加“摘要 + 签名 + 关键实现”混合代码 chunk 策略
  说明：当前 `code_outline` 只抽签名线；拟为每个文件生成“文件摘要 / 重要函数签名 / 小段关键实现”的混合 chunk，提高代码 RAG 信息密度
- [ ] 为基础清洗补评测基线
  说明：需要分别评估文本去噪率、文档模板去重率、代码 chunk 有效密度、回答质量变化，避免只做规则堆叠

### 2.5 其他增强

- [ ] 增加压缩效果评测工具
- [ ] 增加 `log_dedup`
- [ ] 增加 `tool_output_focus`
- [ ] 增加 `rag_rerank_trim`
- [ ] **NEW 2026-04-18** RAG 语义去重（embedding 相似度阈值合并）
  说明：当前 RAG chunks 仅做稳定排序，未处理跨 chunk 的语义重复。低相关与语义重复是降低回答质量的两大来源，拟在 rerank 之后增加一层 embedding 去重层，阈值内视为同一语义事实并保留分数最高的那一段

### 2.6 当前建议顺序（更新 2026-04-19）

1. 先补 `service mapping / summary / config` 最小单测闭环
2. 再补 `Refine / PageIn / Redis / worker` 集成测试，并继续收拢到 `tests/integration`
3. 然后继续推进“基础清洗 / 入库预处理”专题，优先补 `TextSanitizer Hook`、sentence-aware chunking、文档标题分段与代码注释安全清洗
4. 再推进 `Explain / dry-run / cache debug`，把预测结果做成可解释输出
5. 启动前缀四段分层（tools 独立）与 active turn 轻量去抖，作为 KV 命中优化的下一块基石
6. 再补应用层低基数 metrics、dashboard 与离线 replay
7. 接真实摘要 provider，升级 `SummaryArtifact` 到 chunk 级
8. 设计 segment-level 部分复用语义，并尝试接入下游真实 KV 指标做对照验证

## 3. 已完成任务归档

### 2026-04-19 / core 组件层首轮抽取与清洗收口

- [x] 抽出独立 `TextSanitizer` 组件
  说明：已形成“原始文本 -> 清洗后文本 + 清洗报告”的统一入口，`collapse / compact / canonicalize` 不再各自维护分散规则
- [x] 补齐通用文本去噪规则
  说明：已支持 HTML 标签剥离、`script/style` 去除、emoji 去除、Unicode 控制字符过滤，以及 XML 声明 / doctype / comment / CDATA 噪音清理
- [x] 抽出独立 `RAGNormalizer` 组件
  说明：RAG chunk/source/fragment 的稳定化、JSON 去 volatile keys、source/pageRef 归一化已统一收口
- [x] 抽出独立 `PromptComponent` 组件
  说明：prompt 组装、stable segment 切分、chunk/message 渲染、stable prefix section 拼接已下沉到组件层，`prefix_cache_identity` 不再自行做版式拼装
- [x] 补齐组件辅助层并切换 processor 调用关系
  说明：`FragmentTransformer`、`ChunkMetadataHelper` 已一并沉到 `internal/domain/core/components`；processor 改为调用组件执行；`core/text_sanitizer.go`、`core/rag_normalizer.go`、`core/prompt_component.go` 旧包装文件已删除
- [x] 完成 `core` façade / bridge 层首轮瘦身
  说明：`component_access.go` 已拆为 `component_defaults.go` 与 `components_bridge.go`；stable prefix section 生成已下沉到 `PromptComponent.BuildStablePrefixSectionsFromMessages`；`BuildPrefixCacheIdentity` 与 `StableRAGChunks` 改为纯委托组件执行，`core` 不再保留对应 section 拼装与排序辅助逻辑

### 2026-04-18 / 统一输入结构切换为 system + messages + memory

- [x] 完成 HTTP 外部 JSON 入参切换
  说明：`/api/refine` 现仅接受 `system + messages + memory`；`memory.rag` 取代顶层 `rag`，旧顶层 `rag` 输入已删除
- [x] 完成 gRPC / protobuf 协议同步
  说明：`api/refiner.proto` 与 `api/refinerv1/*.pb.go` 已收敛为单一结构化协议；请求侧删除旧 `rag_chunks`，响应侧删除旧 `optimized_prompt` 外部字段
- [x] 完成内部 DTO / mapper / service 统一
  说明：内部请求快照、trace evaluation、dashboard 结构化展示已统一映射为 `system / messages / memory`
- [x] 完成示例与文档同步
  说明：HTTP 文档、docker 文档、mock gRPC seed 已改为显式 `System + Messages + Memory` 心智模型，避免继续扩散旧写法
- [x] 完成返回结构主视图切换
  说明：对外返回已从“服务端预拼好的最终 prompt 字符串”切换为结构化 `system + messages + memory`；调用方自行根据目标模型 API 设计下游拼装

### 2026-04-18 / 删除旧协议兼容层

- [x] 删除 HTTP 顶层 `rag` 兼容入口
  说明：`decodeMemory` 只解析 `memory.rag`，不再接受旧顶层 `rag`
- [x] 删除 gRPC 请求旧字段兼容
  说明：`RefineRequest.rag_chunks` 已从 proto 删除并保留 `reserved`
- [x] 删除 gRPC 响应旧字段兼容
  说明：`RefineResponse.optimized_prompt` 已从 proto 删除并保留 `reserved`
- [x] 删除 `role=system` 隐式提升逻辑
  说明：mapper 不再把 `messages[].role=system` 自动并入顶层 `system`

### 2026-04-18 / todolist 整理与新方向并入

- [x] 归档 P0 工程目录结构重构整段
  说明：2.0 下的 10 条目标均已通过 2026-04-13 的四轮收尾贯穿完成，整段从未完成区移入归档
- [x] 归档应用层 KV 调研结论段
  说明：原 2.3.0 六条调研结论已作为不变前提贯穿后续工作，整段从未完成区移入归档
- [x] 归档已完成的可命中性诊断与摘要链路条目
  说明：`统一命中分类枚举`、`RefineResponse.metadata 观测层级字段`、`抽象 SummaryProvider`、`结构化 SummaryArtifact` 四项已完成条目从原未完成区清理
- [x] 并入下一阶段方向
  说明：新增「前缀四段分层（tools 独立）」「active turn 轻量去抖」「prompt_layout_version 纳入 miss reason」「segment-level 部分复用」「对照下游真实 KV」「RAG 语义去重」「应用层预测 vs 下游真实命中相关性仪表盘」共 7 项，分别并入 2.3.2 / 2.3.3 / F / 2.4

### 2026-04-18 / 工程目录重构整体收尾（原 2.0 归档）

- [x] 明确接近 Java 风格的目标目录骨架
  说明：目录目标收敛为 `cmd / api / config / docs / deploy / internal / tests`，`internal` 内形成 `controller / service / mapper / dto / domain / support / observability / bootstrap` 分层
- [x] 明确应用入口层的 OO 拆分方式
  说明：对外入口统一按 `controller -> service -> mapper -> dto` 分层
- [x] 拆出独立测试目录
  说明：收敛为 `tests/unit`、`tests/integration`、`tests/e2e`
- [x] 核心链路代码收拢到 `domain/core`
- [x] 观测与监控代码收拢到 `observability` 模块族
- [x] 接入实现统一收敛到 `support/*`
- [x] 公共支持代码收拢到 `support` 模块
- [x] 启动装配与运行时 wiring 收拢到 `bootstrap`
- [x] 统一文件夹与文件命名规范
  说明：已在多轮命名收尾中贯穿完成，后续局部调整纳入日常 PR
- [x] 分阶段迁移纪律
  说明：已按"先分层 -> 再拆测试 -> 再迁应用层 -> 再迁 domain -> 再迁 observability/support/bootstrap -> 清理 import 与命名"顺序分四轮稳步推进，未发生一次性大搬家

### 2026-04-18 / 应用层 KV 下一阶段调研结论归档（原 2.3.0）

- [x] 明确系统边界
  说明：本服务负责"应用层可命中性诊断与前缀稳定化优化"，不负责生产"下游真实 KV cache 命中监控"
- [x] 明确应用层命中语义
  说明：`predicted hit/miss` 只代表规范化后的稳定前缀是否更可能被复用，属于应用层诊断信号，不等同于任何下游真实 KV 命中结果
- [x] 应用层观测设计坚持"计数器优先，不直接存 hit rate"
- [x] 指标标签保持低基数
  说明：Prometheus 不适合直接挂 `prefix_hash / session_id / tenant_id` 这类高基数字段；热点前缀与 TopN 应通过 Redis 排行、debug 导出或离线报表承载
- [x] 命中优化必须围绕"精确前缀一致性"
- [x] 应用层诊断必须保持可解释
  说明：继续依赖 `combined/system/memory/rag hash`、`miss_reason`、`segment_reason`、`normalization_version` 做可解释诊断

### 2026-04-18 / 可命中性诊断与摘要链路已完成项归档

- [x] 定义统一命中分类枚举
  说明：当前已以 metadata string enum 形式落地 `predicted_hit`、`predicted_miss`、`partial_reusable`、`unstable_prefix`、`unknown`
- [x] 在 `RefineResponse.metadata` 增加观测层级字段
  说明：已补 `cache_observation_level`、`cache_prediction_result`、`predicted_reusable_tokens`、`segment_churn_reason`
- [x] 抽象 `SummaryProvider`
  说明：`summary worker` 已切到 `Provider -> SummaryArtifact -> Redis store` 链路，默认实现仍为启发式 provider
- [x] 设计结构化 `SummaryArtifact`
  说明：已落地 `artifact_id / content_hash / fragment_types / provider / provider_version / schema_version / created_at / expires_at`，并在 worker、Redis 存储和 `PageIn` protobuf 返回链路中生效

### 2026-04-13 / 工程目录重构第一阶段已完成

- [x] 完成第一轮目录骨架归位
  说明：已形成 `internal/controller`、`internal/service`、`internal/mapper`、`internal/dto`、`internal/domain`、`internal/support`、`internal/observability`、`internal/bootstrap`
- [x] 完成核心链路迁移
  说明：`pipeline / registry / prefix cache / processor / repository contract` 已迁入 `internal/domain/core`
- [x] 完成适配层与观测层归位
  说明：`grpc controller` 迁入 `internal/controller/grpc`，`redis / summary worker / tempo query` 当前收敛在 `internal/support`，`prometheus / tracing` 迁入 `internal/observability`
- [x] 完成 mapper 独立拆包
- [x] 完成测试目录落地
- [x] 完成 import 回收与编译恢复

### 2026-04-13 / 工程目录重构第二阶段已完成

- [x] 完成 dto 内部边界落地
- [x] 保持外部 contract 稳定
- [x] 完成一轮命名规范收尾
- [x] 在 `tests/unit` 落地第一组跨模块单测
- [x] 保持编译与回归通过

### 2026-04-13 / 命名规范收尾第一轮已完成

- [x] 完成 processor 目录第一轮职责化命名
- [x] 完成 support 接入对象命名收口
- [x] 完成 domain 契约与前缀身份文件命名收尾
- [x] 完成相关文档与说明路径同步
- [x] 保持编译与测试通过

### 2026-04-13 / 旧目录残留与错层引用清理已完成

- [x] 清理空壳旧目录
- [x] 清理主要文档中的错层路径
- [x] 完成结构复查与回归验证

### 2026-04-13 / 应用边界收口已完成

- [x] 删除越界的下游复用提示输出
  说明：已删除 `downstream_kv_reuse_hint` metadata / tracing 输出，避免把应用层结果误表述为下游 KV 复用提示
- [x] 收紧内部优化目标语义
  说明：已将 `cache_optimization_target` 默认值从 `prefix-hit-rate` 收敛为应用层 `prefix-reuse-stability`
- [x] 保持 prefix cache 主链不受影响

### 2026-04-13 / 应用层预测 metadata 已完成

- [x] 落地应用层预测观测字段
  说明：`PrefixCacheProcessor` 统一补齐 `cache_observation_level`、`cache_prediction_result`、`predicted_reusable_tokens`、`segment_churn_reason`
- [x] 落地预测结果分类语义
  说明：当前已覆盖 `predicted_miss / partial_reusable / predicted_hit / unstable_prefix / unknown` 这组应用层预测结果
- [x] 增加最小单测护栏
  说明：新增 `prefix_cache` 单测，覆盖 `skipped / created / hit` 三条预测路径

### 2026-04-11 / 应用层 KV 第二阶段 D 已完成

- [x] 增加 prefix cache admission policy
  说明：基于 `min_stable_prefix_tokens` 与 `min_segment_count` 过滤过短或低价值前缀
- [x] 增加热点前缀统计
  说明：Redis 增加 namespace 级 `hot prefix` 统计与 `hot_score`
- [x] 增加 TTL 分层策略
  说明：达到 `hot_threshold` 后使用 `hot_ttl`，并记录 `cache_tier`
- [x] 增加按 `policy / model / tenant` 的 cache namespace
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
