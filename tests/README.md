# Tests Layout

测试目录按视角拆分：

- `tests/unit`：纯单元测试，优先覆盖 mapper、service 编排、support 工具和可独立验证的 domain 逻辑
- `tests/integration`：带 Redis、worker、配置装配等模块联调测试
- `tests/e2e`：从 gRPC 入口出发的端到端链路验证

说明：

- 贴身算法微测仍可保留在实现文件旁，例如 processor 的细粒度 `_test.go`
- 大多数跨模块测试逐步收敛到 `tests/` 下，避免后续继续散落
