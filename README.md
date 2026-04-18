# Context Refiner

> 2026-04-11 结构重构后，当前生效的分层说明请优先看 [docs/layered-architecture.md](/E:/github/Memory_chunk/docs/layered-architecture.md)。
> 当前代码组织已经调整为 `api + service + adapter + core + infra + bootstrap`。

一个位于 AI 应用层与大模型 API 之间的 Go gRPC 上下文清洗服务。

根目录这份 README 现在只承担一件事：

`把你带到新的 docs 文档体系入口。`

## 1. 从哪里开始

总入口：

- [docs/README.md](/E:/github/Memory_chunk/docs/README.md)

如果你只想快速找到合适文档，可直接按用途进入：

- 项目定位与整体设计：[docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
- 本地启动与最小调用：[docs/quickstart.md](/E:/github/Memory_chunk/docs/quickstart.md)
- Docker Compose 一键部署：[docs/docker-deployment.md](/E:/github/Memory_chunk/docs/docker-deployment.md)
- 代码结构与核心模块：[docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)
- AI / Agent 协作入口：[Agent.md](/E:/github/Memory_chunk/Agent.md)
- 原理、取舍与边界：[docs/principles-and-internals.md](/E:/github/Memory_chunk/docs/principles-and-internals.md)
- 应用层 KV cache 设计：[docs/kv-cache-design.md](/E:/github/Memory_chunk/docs/kv-cache-design.md)
- 当前进度：[docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md)
- 后续路线：[docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)
- 测试补齐计划：[docs/test-plan.md](/E:/github/Memory_chunk/docs/test-plan.md)

## 2. 当前项目状态

当前仓库已经具备这些核心能力：

- gRPC `Refine` / `PageIn` 主链
- 基于 `tiktoken-go` 的统一 Token 计数
- `Pipeline-Processor` 处理器注册与执行骨架
- 结构化 `RAGFragment` 支持 `body / code / table / json / log / error-stack` 等类型
- Redis page-out / page-in
- summary job 入队、worker 消费与摘要回填
- 语义审计字段与压缩过程记录
- 应用层 stable prefix 规范化与分层 prefix hash
- 应用层 prefix cache 策略
  说明：`admission`、`namespace`、`TTL 分层`、`hot prefix`、`prewarm`
- 应用层 prefix cache miss reason 观测与 tracing

但项目还没有进入“工程化完成”状态，当前主要缺口仍然是：

- 测试
- Explain / dry-run / cache debug
- 评测 / replay / alerting
- 真实摘要 Provider
- 更稳定的摘要对象与失效策略

这些内容在 [docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md) 和
[docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)
里有更完整说明。

## 3. 最短阅读路径

如果你是第一次进入这个仓库，建议按这个顺序：

1. [docs/README.md](/E:/github/Memory_chunk/docs/README.md)
2. [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
3. [docs/quickstart.md](/E:/github/Memory_chunk/docs/quickstart.md)
4. [docs/docker-deployment.md](/E:/github/Memory_chunk/docs/docker-deployment.md)
5. [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)
6. [Agent.md](/E:/github/Memory_chunk/Agent.md)

如果你是准备补测试或继续工程化，建议按这个顺序：

1. [docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md)
2. [docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)
3. [docs/test-plan.md](/E:/github/Memory_chunk/docs/test-plan.md)

## 4. 仓库内主要目录

- [api](/E:/github/Memory_chunk/api) gRPC 协议定义与生成代码入口
- [cmd](/E:/github/Memory_chunk/cmd) 程序启动入口
- [config](/E:/github/Memory_chunk/config) 服务配置与策略配置
- [internal](/E:/github/Memory_chunk/internal) 核心实现，包括 engine、processor、server、store、summary 等模块
- [docs](/E:/github/Memory_chunk/docs) 当前维护中的文档主目录

## 5. 迁移说明

如果你之前把根 README 当作唯一说明文档使用，现在需要改成：

- 先看 [docs/README.md](/E:/github/Memory_chunk/docs/README.md)
- 再按具体目标进入对应子文档

这样做的目的，是避免继续把“概览、教程、参考、解释、计划、状态”混在一篇里。
