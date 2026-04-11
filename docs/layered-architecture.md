# Context Refiner Layered Architecture

- 文档版本：`v2026.04.11`
- 更新日期：`2026-04-11`
- 文档类型：`Architecture Reference`
- 适用代码基线：`main`

## 1. 重构目标

这次调整只改代码结构，不改业务主逻辑，目标是把项目整理成更像应用工程的分层形态：

- `api`：对外协议
- `service`：应用服务
- `adapter`：协议适配器
- `core`：核心 pipeline 与 processor
- `infra`：Redis、tokenizer、summary、配置等实现
- `bootstrap`：装配
- `cmd`：极薄启动入口

## 2. 当前目录

```text
api/
  refiner.proto
  refinerv1/

cmd/
  refiner/
    main.go

internal/
  adapter/
    grpc/
  bootstrap/
  core/
    repository/
    processor/
  infra/
    store/
      redis/
    config/
    summary/
    tokenizer/
  service/
  support/
    heuristic/

pkg/
  client/
  service/
```

## 3. 层职责

### 3.1 `api/`

- 定义 `Refine / PageIn` 的 protobuf contract
- 作为跨进程接口边界

### 3.2 `pkg/service/`

- 定义公开的 `RefinerService` 接口
- 让“服务能力”与“gRPC 传输”解耦

### 3.3 `internal/service/`

- 应用服务主流程
- 负责 policy、request mapping、pipeline 调用、response mapping

### 3.4 `internal/adapter/grpc/`

- 只做 gRPC 注册和委托
- 不承载核心业务逻辑

### 3.5 `internal/core/`

- 定义核心模型、processor contract、pipeline、registry
- 定义 repository contract
- 不直接关心 gRPC 或 Redis

### 3.6 `internal/infra/`

- 配置加载
- Redis repository implementation
- tokenizer
- summary worker

### 3.7 `internal/support/`

- 存放跨层复用但不属于主业务流的辅助能力
- 当前主要是 heuristic

### 3.8 `internal/bootstrap/`

- 装配 `infra -> core -> service -> adapter`

### 3.9 `cmd/refiner/`

- 启动入口
- 生命周期控制
- 优雅退出

## 4. 调用链路

`Refine`：

1. gRPC 请求进入 `internal/adapter/grpc`
2. adapter 委托 `pkg/service.RefinerService`
3. `internal/service` 做 policy + request mapping
4. `internal/core` 跑 pipeline 和 processor chain
5. `internal/service` 做 response mapping
6. gRPC 返回结果

`PageIn`：

1. gRPC 请求进入 adapter
2. adapter 委托 service
3. service 通过 `internal/core/repository` 访问仓储契约
4. 优先返回 summary，缺失时回退原 page

## 5. 当前关键文件

- [cmd/refiner/main.go](/E:/github/Memory_chunk/cmd/refiner/main.go)
- [internal/bootstrap/runtime.go](/E:/github/Memory_chunk/internal/bootstrap/runtime.go)
- [internal/bootstrap/processors.go](/E:/github/Memory_chunk/internal/bootstrap/processors.go)
- [internal/adapter/grpc/refiner_handler.go](/E:/github/Memory_chunk/internal/adapter/grpc/refiner_handler.go)
- [internal/service/refiner_service.go](/E:/github/Memory_chunk/internal/service/refiner_service.go)
- [internal/core/pipeline.go](/E:/github/Memory_chunk/internal/core/pipeline.go)
- [internal/core/registry.go](/E:/github/Memory_chunk/internal/core/registry.go)
- [internal/core/repository/repository.go](/E:/github/Memory_chunk/internal/core/repository/repository.go)
- [internal/infra/store/redis/repository.go](/E:/github/Memory_chunk/internal/infra/store/redis/repository.go)
- [pkg/service/refiner.go](/E:/github/Memory_chunk/pkg/service/refiner.go)

## 6. 这次重构的意义

- gRPC handler 变成薄适配层
- service 成为应用主入口，更像常见 Java 应用分层
- core 与 infra 分开，替换依赖更清晰
- repository contract 已上收到 `core`，Redis 细节留在 `infra`
- 对外公开了 service interface，为后续双模式兼容预留空间

## 7. 后续建议

1. 继续把 `PageRepository` 与 `SummaryJobRepository` 细拆成更独立的接口
2. 继续给 `pkg/client` 增加真正的 gRPC client 封装
3. 按 `service / core / infra` 维度补测试
