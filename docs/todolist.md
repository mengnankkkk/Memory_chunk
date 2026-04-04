# Context Refiner Todo List

## 已完成
- [x] 统一真实 Token 计数口径，移除入口和首轮预算判断中的估算逻辑
- [x] 将 `Processor` 升级为带能力声明的组件
- [x] 将 RAG 输入从纯文本 `chunk` 升级为结构化 `fragment`
- [x] 将分页 key 升级为作用域 key：`session_id + request_id + chunk_id + content_hash + page_index`
- [x] 增加语义保真审计字段
- [x] 将 `auto_compact` 拆分为 `auto_compact_sync` 与 `auto_compact_async`
- [x] 跑通异步摘要闭环：入队 -> worker 消费 -> 生成摘要 -> 写回 Redis -> `PageIn` 优先返回摘要
- [x] 增加结构化片段细分处理器：`json_trim` / `table_reduce` / `code_outline` / `error_stack_focus`
- [x] 补充阶段性文档：设计文档 + 实施方案文档

## 当前状态
- [x] `go build ./...` 可通过
- [x] gRPC / Protobuf 已生成并接入服务端
- [x] Redis 分页、PageIn、异步摘要 worker 已打通
- [x] 真实地址仍保留为空，占位配置未填写
- [ ] 当前摘要 worker 仍是启发式摘要，不是真实外部摘要模型
- [ ] 当前摘要回填仍以 page 级结果为主，还不是 chunk 级对象
- [ ] 当前尚未补齐系统级观测、评测和集成测试

## 当前优先级
- [ ] 对接真实外部摘要模型，替换当前启发式摘要 worker
- [ ] 为摘要结果增加更细粒度的存储索引与失效策略
- [ ] 将摘要结果升级为 chunk 级可回填对象，而不只是 page 级 summary

## 下一阶段
- [ ] 增加 Prometheus 指标
- [ ] 增加 Tracing
- [ ] 增加异步摘要 worker 集成测试
- [ ] 增加压缩效果评测工具
- [ ] 增加 `log_dedup`
- [ ] 增加 `rag_rerank_trim`
- [ ] 增加 `tool_output_focus`
- [ ] 支持跨请求 `content_hash` 级缓存复用

## 文档
- [x] 总体设计文档：`docs/context-refiner-design.md`
- [x] 详细实施方案：`docs/implementation-plan.md`
