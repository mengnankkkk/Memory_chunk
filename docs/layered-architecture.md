# Context Refiner Layered Architecture

- 文档版本：`v2026.04.19`
- 更新日期：`2026-04-19`
- 文档类型：`Architecture Reference`
- 适用代码基线：`main`

## 1. 设计目标

这轮重构的目标不是把 Go 项目硬拷成 Java，而是借用 `controller / service / mapper / dto` 这类分层边界，把当前工程从“原型式目录”收敛成更稳定的应用工程结构。

设计原则：

- 对外入口按职责分层，但避免过度抽象
- 核心链路按业务共性收拢到 `domain`
- 外部协议放在 `controller`，接入实现与公共支撑统一收敛到 `support`
- 观测、装配、公共支持能力独立归位
- 保持 `cmd` 极薄，业务逻辑不回流到启动层

补充说明：

- 参考了 Go 项目结构与包命名最佳实践，保留了 Go 倾向“按职责与边界组织 package”的做法
- 因此本项目采用“有限分层 + 核心域收拢”的方式，而不是把所有代码机械拆成很多空层

## 2. 当前目录

```text
api/
  refiner.proto
  refinerv1/

cmd/
  refiner/
    main.go

internal/
  bootstrap/
  controller/
    grpc/
  domain/
    core/
      components/
      processor/
      repository/
  dto/
  infra/
    config/
  mapper/
  observability/
    metrics/
    tracing/
  service/
  support/
    heuristic/
    redis/
    summary/
    tempo/
    tokenizer/

pkg/
  client/
  service/

tests/
  unit/
  integration/
  e2e/
```

## 3. 分层职责

### 3.1 `api/`

- 定义对外 protobuf contract
- 稳定承载 `Refine / PageIn` 这组跨进程接口

### 3.2 `cmd/`

- 只保留进程入口与生命周期控制
- 不承载业务编排

### 3.3 `internal/controller/`

- 对外协议入口层
- 当前是 gRPC controller，只负责接收请求、委托 service、返回响应

### 3.4 `internal/service/`

- 应用服务层
- 负责 policy 解析、调用 domain pipeline、串联 observability
- 对外仍保持 protobuf 级接口，对内已切到 `dto` 主链

### 3.5 `internal/mapper/`

- 负责协议对象与内部对象转换
- 当前已独立承载 request / response mapping
- 这是 `controller -> service -> domain` 之间的重要缓冲层

### 3.6 `internal/dto/`

- 承载应用内部 DTO
- 当前已落地：
  - `RefineRequest`
  - `RefineResponse`
  - `PageInRequest`
  - `PageInResponse`
- 用于隔离外部 protobuf contract 与内部 service / domain 演进

### 3.7 `internal/domain/`

- 放置核心业务链路
- 当前 `internal/domain/core` 承载：
  - pipeline
  - registry
  - prefix cache identity
  - prompt segmentation
  - processor contract 与具体 processors
  - repository contract
- 这一层不直接关心 gRPC、Redis、Prometheus 的实现细节

### 3.8 `internal/support/`

- 放置跨层复用能力与接入实现
- 当前已拆出：
  - `support/redis`：Redis repository 与 dashboard / trace evaluation 持久化实现
  - `support/summary`：summary worker、provider 与启发式摘要实现
  - `support/tempo`：Tempo 查询实现
  - `support/heuristic`、`support/tokenizer`：通用算法与计数能力
- 后续如果引入真实摘要模型或其他外部接入，也继续按接入对象归入该层

### 3.9 `internal/observability/`

- 放置 metrics / tracing 等观测实现
- 当前已拆出：
  - `observability/metrics`
  - `observability/tracing`
- `internal/observability/recorder.go` 保留跨层观测契约

### 3.10 `internal/domain/core/components/`

- 放置核心域内可复用组件实现
- 当前主要包括：
  - `TextSanitizer`
  - `RAGNormalizer`
  - `PromptComponent`
  - `FragmentTransformer`
  - `ChunkMetadataHelper`

### 3.11 `internal/infra/`

- 当前只保留与工程配置读取直接相关的实现
- 已缩窄为 `infra/config`
- 原先放在 `infra` 下的 Redis、summary、tokenizer、tracing、observability 已迁走

### 3.12 `internal/bootstrap/`

- 负责装配整个运行时
- 统一串联 `config -> support -> domain -> service -> controller`
- 承载 registry 组装、runtime 加载、worker 启动、metrics server 启动

### 3.13 `pkg/service/`

- 对外暴露稳定服务接口
- 让“服务能力”与“传输方式”解耦

### 3.14 `tests/`

- 承载跨模块测试视角
- 当前先落目录骨架，后续逐步迁入 unit / integration / e2e 测试

## 4. 依赖方向

推荐依赖方向如下：

1. `controller -> service`
2. `service -> mapper / dto / domain / observability`
3. `mapper -> dto / domain`
4. `bootstrap -> controller / service / support / observability / config`
5. `support/redis|summary|tempo -> domain/repository contract`

不建议出现的反向依赖：

- `domain -> controller`
- `domain -> support concrete implementation`
- `service -> redis client concrete details`
- `controller -> repository`

## 5. 当前调用链

`Refine` 主链：

1. gRPC 请求进入 [refine_controller.go](/E:/github/Memory_chunk/internal/controller/grpc/refine_controller.go)
2. controller 委托 [refiner.go](/E:/github/Memory_chunk/pkg/service/refiner.go) 定义的服务接口
3. [refine_service.go](/E:/github/Memory_chunk/internal/service/refine_service.go) 先将 protobuf request 收敛成 dto
4. [refine_request_mapper.go](/E:/github/Memory_chunk/internal/mapper/refine_request_mapper.go) 将 dto 转为 domain request
5. [pipeline.go](/E:/github/Memory_chunk/internal/domain/core/pipeline.go) 执行 processor chain
6. [refine_response_mapper.go](/E:/github/Memory_chunk/internal/mapper/refine_response_mapper.go) 将 domain response 先转 dto，再转 protobuf response
7. controller 返回 gRPC 响应

`PageIn` 主链：

1. gRPC 请求进入 controller
2. service 调用 domain repository contract
3. [redis_repository.go](/E:/github/Memory_chunk/internal/support/redis/redis_repository.go) 作为 Redis 接入实现进行 page / summary 读取
4. 优先返回 summary，缺失时回退原 page

`SummaryWorker` 主链：

1. [runtime.go](/E:/github/Memory_chunk/internal/bootstrap/runtime.go) 启动 worker
2. [summary_worker.go](/E:/github/Memory_chunk/internal/support/summary/summary_worker.go) 消费 summary job
3. [heuristic_summarizer.go](/E:/github/Memory_chunk/internal/support/summary/heuristic_summarizer.go) 生成启发式摘要
4. Redis repository 回写 summary artifact

## 6. 当前关键文件

- [main.go](/E:/github/Memory_chunk/cmd/refiner/main.go)
- [runtime.go](/E:/github/Memory_chunk/internal/bootstrap/runtime.go)
- [processors.go](/E:/github/Memory_chunk/internal/bootstrap/processors.go)
- [refine_controller.go](/E:/github/Memory_chunk/internal/controller/grpc/refine_controller.go)
- [refine_service.go](/E:/github/Memory_chunk/internal/service/refine_service.go)
- [refine_request_mapper.go](/E:/github/Memory_chunk/internal/mapper/refine_request_mapper.go)
- [refine_response_mapper.go](/E:/github/Memory_chunk/internal/mapper/refine_response_mapper.go)
- [refine_request_dto.go](/E:/github/Memory_chunk/internal/dto/refine_request_dto.go)
- [refine_response_dto.go](/E:/github/Memory_chunk/internal/dto/refine_response_dto.go)
- [pipeline.go](/E:/github/Memory_chunk/internal/domain/core/pipeline.go)
- [registry.go](/E:/github/Memory_chunk/internal/domain/core/registry.go)
- [prefix_cache_identity.go](/E:/github/Memory_chunk/internal/domain/core/prefix_cache_identity.go)
- [repository_contracts.go](/E:/github/Memory_chunk/internal/domain/core/repository/repository_contracts.go)
- [redis_repository.go](/E:/github/Memory_chunk/internal/support/redis/redis_repository.go)
- [prometheus_recorder.go](/E:/github/Memory_chunk/internal/observability/metrics/prometheus_recorder.go)
- [provider.go](/E:/github/Memory_chunk/internal/observability/tracing/provider.go)

## 7. 本轮已完成结果

- 已把 controller 稳定收敛到 `internal/controller/grpc`
- 已把核心链路从旧 `internal/core` 迁到 `internal/domain/core`
- 已把 Redis、summary worker、Tempo 查询统一收敛到 `internal/support/*`
- 已把 tokenizer 从旧 `internal/infra/tokenizer` 迁到 `internal/support/tokenizer`
- 已把 metrics / tracing 从旧 `internal/infra/*` 迁到 `internal/observability/*`
- 已把 request / response mapping 从 `internal/service` 拆到 `internal/mapper`
- 已补 `tests/unit`、`tests/integration`、`tests/e2e` 目录骨架
- 已把 `service` 内部主链从 protobuf 直接依赖收敛为 `dto`
- 已补 `tests/unit/mapper` 边界单测
- 已完成 processor 聚合与 core 组件化收口：
  说明：processor 当前按 `stage_01_preprocess / stage_02_transform / stage_03_compaction / stage_04_finalize` 聚合编排，具体文本 / RAG / prompt 实现下沉到 `core/components`；Redis、summary、Tempo 接入实现统一归入 `support/*`
- 已修复迁移后的 import 路径，`go test ./...` 可通过

## 8. 下一步

1. 将跨模块测试逐步迁入 `tests/` 目录
2. 在新结构下继续推进 `Explain / dry-run / cache debug`
3. 在 dto 边界稳定后，再考虑是否把 `pkg/service` 进一步抽成更纯净的应用接口
4. 仅在后续发现新的高收益命名问题时，再做小步重命名，不再做低收益批量扰动
