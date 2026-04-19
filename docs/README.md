# Context Refiner 文档索引

- 文档版本：`v2026.04.19`
- 更新日期：`2026-04-19`
- 文档类型：`Documentation Index`

> 2026-04-19 起，代码结构请优先以 [docs/layered-architecture.md](/E:/github/Memory_chunk/docs/layered-architecture.md) 为准。
> 如果你是来看目录分层和职责，先看分层文档，再看 `docs/code-design.md`。

## 1. 文档集说明

本目录不再使用“一份超大总文档包打天下”的方式，而是拆成多份聚焦文档。

这样拆分有三个好处：

- 不同读者可以直接进入自己需要的文档
- 文档更新时不容易相互污染
- 架构、教程、参考、解释、进度可以分别维护

这也符合常见技术文档拆分思路：把“概览、操作、参考、解释”分开维护，而不是混在一篇里。

## 2. 阅读入口

### 2.1 先想知道这个项目是什么

看：

- [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)

### 2.2 先想快速跑起来

看：

- [docs/quickstart.md](/E:/github/Memory_chunk/docs/quickstart.md)
- [docs/docker-deployment.md](/E:/github/Memory_chunk/docs/docker-deployment.md)

### 2.3 先想理解代码结构 / 分层

看：

- [docs/layered-architecture.md](/E:/github/Memory_chunk/docs/layered-architecture.md)
- [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)

### 2.4 先想系统学习这个项目

看：

- [docs/learning-guide.md](/E:/github/Memory_chunk/docs/learning-guide.md)

### 2.5 先想理解设计原理和取舍

看：

- [docs/principles-and-internals.md](/E:/github/Memory_chunk/docs/principles-and-internals.md)
- [docs/kv-cache-design.md](/E:/github/Memory_chunk/docs/kv-cache-design.md)

### 2.6 先想知道现在做到哪了、接下来做什么

看：

- [docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md)
- [docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)

### 2.7 先想知道应该怎么补测试

看：

- [docs/test-plan.md](/E:/github/Memory_chunk/docs/test-plan.md)

### 2.8 先想让 AI / Agent 快速接手项目

看：

- [Agent.md](/E:/github/Memory_chunk/Agent.md)

## 3. 文档清单

### 3.1 Overview

- [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
  当前系统定位、分层、边界、阶段判断。

### 3.2 Reference

- [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)
  代码包结构、核心对象、关键接口、处理器链路、扩展点。

### 3.3 How-To

- [docs/quickstart.md](/E:/github/Memory_chunk/docs/quickstart.md)
  本地配置、启动、调用、排障。

- [docs/docker-deployment.md](/E:/github/Memory_chunk/docs/docker-deployment.md)
  根目录 `docker compose` 一键部署、端口映射、健康检查、排障。

### 3.4 Explanation

- [docs/principles-and-internals.md](/E:/github/Memory_chunk/docs/principles-and-internals.md)
  为什么这样设计、核心原理、取舍和边界。

- [docs/kv-cache-design.md](/E:/github/Memory_chunk/docs/kv-cache-design.md)
  应用层 KV cache 的边界、prefix identity、admission、TTL、miss 诊断与实现细节。

### 3.5 Learning

- [docs/learning-guide.md](/E:/github/Memory_chunk/docs/learning-guide.md)
  学习路线、阅读顺序、关键问题、练习建议。

### 3.6 Status

- [docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md)
  当前进度、缺口、优先级。

- [docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)
  未来阶段计划、里程碑、建议执行顺序。

### 3.7 Testing

- [docs/test-plan.md](/E:/github/Memory_chunk/docs/test-plan.md)
  测试范围、测试层次、环境准备、阶段目标、准入准出标准。

### 3.8 Agent Context

- [Agent.md](/E:/github/Memory_chunk/Agent.md)
  面向项目专用 Agent 的快速上下文：结构、约束、改动入口、验证方式、文档同步规则。

## 4. 推荐阅读路径

### 路径 A：第一次接触项目

1. [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
2. [docs/quickstart.md](/E:/github/Memory_chunk/docs/quickstart.md)
3. [docs/docker-deployment.md](/E:/github/Memory_chunk/docs/docker-deployment.md)
4. [docs/learning-guide.md](/E:/github/Memory_chunk/docs/learning-guide.md)

### 路径 B：准备改代码

1. [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
2. [docs/code-design.md](/E:/github/Memory_chunk/docs/code-design.md)
3. [docs/todolist.md](/E:/github/Memory_chunk/docs/todolist.md)
4. [docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)
5. [docs/test-plan.md](/E:/github/Memory_chunk/docs/test-plan.md)
6. [Agent.md](/E:/github/Memory_chunk/Agent.md)

### 路径 C：准备评审架构

1. [docs/context-refiner-design.md](/E:/github/Memory_chunk/docs/context-refiner-design.md)
2. [docs/principles-and-internals.md](/E:/github/Memory_chunk/docs/principles-and-internals.md)
3. [docs/kv-cache-design.md](/E:/github/Memory_chunk/docs/kv-cache-design.md)
4. [docs/implementation-plan.md](/E:/github/Memory_chunk/docs/implementation-plan.md)

## 5. 维护规则

后续更新文档时建议遵守以下规则：

- 改架构边界时，先改 `context-refiner-design.md`
- 改具体代码结构时，先改 `code-design.md`
- 改启动方式或调用方式时，先改 `quickstart.md`
- 改根目录 `docker compose` 部署流程时，先改 `docker-deployment.md`
- 改当前状态与优先级时，先改 `todolist.md`
- 改阶段计划时，先改 `implementation-plan.md`
- 改测试策略、覆盖范围和阶段验收时，先改 `test-plan.md`

这样可以避免“文档有很多，但没人知道先改哪一份”的问题。
