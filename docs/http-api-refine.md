# HTTP API 接入指南 - /api/refine

## 概述

`POST /api/refine` 是一个简化的 REST 端点，让外部应用无需 gRPC 即可接入上下文清洗服务。

- **地址**：`http://127.0.0.1:18080/api/refine`（与 dashboard 同端口）
- **方法**：`POST`
- **Content-Type**：`application/json; charset=utf-8`

## 请求格式

### 最小示例（只需 system + messages）

```json
{
  "system": "你是一个代码助手",
  "messages": [
    {"role": "user", "content": "帮我排查 docker compose 的 EOF 问题"}
  ]
}
```

### 完整示例（带 memory 和可选参数）

```json
{
  "system": "你是一个偏后端的代码助手，回答必须中文，并保留关键栈信息。",
  "messages": [
    {"role": "user", "content": "我的 otel-collector 一直 EOF"},
    {"role": "assistant", "content": "先检查 4318 端口"},
    {"role": "user", "content": "已验证端口通，下面是日志"}
  ],
  "memory": {
    "rag": [
      "2026-04-17 WARN otel-collector export EOF",
      "services:\n  otel-collector:\n    ports: [4318]"
    ]
  },
  "model": "gpt-4o-mini",
  "budget": 260,
  "policy": "strict_coding_assistant",
  "session_id": "my-session-123",
  "request_id": "my-req-456"
}
```

### 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `system` | string | 可选 | 系统 prompt，独立于 `messages` 传递 |
| `messages` | array | 必填* | 对话历史，每条包含 `role` 和 `content`；可承载 `user / assistant / toolcall / toolresponse` 等消息类型<br>*如果提供了 `system`，`messages` 可以为空 |
| `memory` | object | 可选 | 长期/外部记忆容器，当前支持 `memory.rag` |
| `memory.rag` | array | 可选 | RAG 上下文片段，支持两种格式：<br>1. 字符串数组：`["片段1", "片段2"]`<br>2. 对象数组：`[{id, source, content, type}]` |
| `model` | string/object | 可选 | 模型名称（如 `"gpt-4o-mini"`）或对象 `{name, max_context_tokens}` |
| `budget` | int | 可选 | token 预算，缺省时从 policy/model 推导 |
| `policy` | string | 可选 | 清洗策略名称，缺省走 `default_policy` |
| `session_id` | string | 可选 | 会话 ID，缺省自动生成 `ext-{random}` |
| `request_id` | string | 可选 | 请求 ID，缺省自动生成 `req-{random}` |

## 响应格式

```json
{
  "system": "你是代码助手",
  "messages": [
    {"role": "user", "content": "帮我排查问题"}
  ],
  "memory": {
    "rag": [
      {
        "id": "rag-1",
        "source": "external",
        "sources": ["external"],
        "fragments": [
          {"type": "FRAGMENT_TYPE_LOG", "content": "日志片段"}
        ]
      }
    ]
  },
  "trace_id": "8151e060...",        // trace ID，用于在 dashboard 定位
  "request_id": "req-2e070c1c...",
  "session_id": "ext-46be0add...",
  "input_tokens": 209,              // 清洗前 token 数
  "output_tokens": 209,             // 清洗后 token 数
  "saved_tokens": 0,                // 节省的 token 数
  "compression_ratio": 1.0,         // 压缩比（output/input）
  "budget_met": true,               // 是否满足预算
  "cache_hit": false                // 前缀缓存是否命中
}
```

返回体中的 `system + messages + memory` 就是清洗完成后的结构化上下文。调用方应基于自己的模型 API 设计，自行把这三块拼成最终下游请求，而不是依赖服务端提前拼好的字符串 prompt。

## 清洗完成后的上下文示例

下面这个示例就是当前 `/api/refine` 返回给调用方的核心结果形态，也是 dashboard / trace evaluation 中可看到的 `output_context` 结构：

```json
{
  "system": "你是一个偏后端排障的代码助手。回答必须中文，保留端口、容器名、错误码、关键调用链和最终可执行结论。",
  "messages": [
    {
      "role": "user",
      "content": "我在本机 Windows 上排查 docker compose 部署。当前问题是 otel-collector 向 tempo 导出 traces 时持续 EOF，同时 Redis 中 summary jobs 有积压。请压缩上下文，但不要丢 collector -> tempo 链路、端口映射和最近一次 fallback summary 线索。"
    },
    {
      "role": "assistant",
      "content": "已知重点：4318/14318 端口映射、tempo 接收状态、Redis backlog、fallback summary、中文输出偏好。"
    }
  ],
  "memory": {
    "rag": [
      {
        "id": "docker-log-otel",
        "source": "runtime/docker-log",
        "sources": ["runtime/docker-log", "runtime/compose"],
        "fragments": [
          {
            "type": "FRAGMENT_TYPE_LOG",
            "content": "2026-04-17T13:19:11Z WARN otel-collector export failed: Post http://tempo:4318/v1/traces EOF\n2026-04-17T13:19:12Z WARN summary worker fallback summary scheduled due to upstream timeout\n2026-04-17T13:19:13Z ERROR redis queue backlog exceeded threshold=128 current=211\n2026-04-17T13:19:16Z WARN otel-collector export failed: connectex no connection could be made because the target machine actively refused it"
          },
          {
            "type": "FRAGMENT_TYPE_ERROR_STACK",
            "content": "java.net.SocketTimeoutException: timeout while exporting trace batch\nCaused by: io.EOF\nat tempo.receiver.otlphttp()"
          }
        ]
      },
      {
        "id": "docker-compose-snippet",
        "source": "repo/deploy",
        "sources": ["repo/deploy", "repo/config"],
        "fragments": [
          {
            "type": "FRAGMENT_TYPE_CODE",
            "language": "yaml",
            "content": "services:\n  context-refiner:\n    ports:\n      - \"15051:15051\"\n      - \"18080:18080\"\n  otel-collector:\n    ports:\n      - \"4317:4317\"\n      - \"4318:4318\"\n  tempo:\n    ports:\n      - \"13200:3200\""
          },
          {
            "type": "FRAGMENT_TYPE_JSON",
            "content": "{\"redis\":{\"addr\":\"127.0.0.1:16379\",\"pending_jobs\":14},\"user_preferences\":{\"language\":\"zh-CN\"}}"
          }
        ]
      }
    ]
  }
}
```

这个示例体现的是清洗后的几个关键结果：

- `system` 被单独收口，不再混在 `messages` 里。
- `messages` 只保留 active turn 中真正有语义价值的对话。
- `memory.rag` 承载外部上下文，保留来源、结构化 fragment 和必要证据链。
- 重复噪音日志、无价值抖动字段、纯样板对话不会继续膨胀到输出上下文里。

## 使用示例

### curl

```bash
curl -X POST http://127.0.0.1:18080/api/refine \
  -H "Content-Type: application/json; charset=utf-8" \
  -d '{
    "system": "你是代码助手",
    "messages": [{"role":"user","content":"帮我排查"}],
    "memory": {"rag": ["日志片段"]},
    "budget": 200
  }'
```

### Python

```python
import requests

resp = requests.post(
    "http://127.0.0.1:18080/api/refine",
    json={
        "system": "你是代码助手",
        "messages": [{"role": "user", "content": "帮我排查"}],
        "memory": {"rag": ["日志片段"]},
        "budget": 200,
    },
)
data = resp.json()
system_prompt = data["system"]
messages = data["messages"]
memory_rag = data["memory"]["rag"]
print(f"压缩比: {data['compression_ratio']:.2%}")
print(f"节省 tokens: {data['saved_tokens']}")
```

### JavaScript / Node.js

```javascript
const response = await fetch("http://127.0.0.1:18080/api/refine", {
  method: "POST",
  headers: { "Content-Type": "application/json; charset=utf-8" },
  body: JSON.stringify({
    system: "你是代码助手",
    messages: [{ role: "user", content: "帮我排查" }],
    memory: { rag: ["日志片段"] },
    budget: 200,
  }),
});
const data = await response.json();
console.log(`system: ${data.system}`);
console.log(`messages: ${data.messages.length}`);
console.log(`cache hit: ${data.cache_hit}`);
```

## 错误响应

```json
{
  "error": "messages 与 system 至少要提供一个"
}
```

常见错误码：
- `400 Bad Request`：请求格式错误、缺少必填字段
- `405 Method Not Allowed`：只支持 POST
- `500 Internal Server Error`：服务端处理失败

## 与 gRPC 的对比

| 维度 | gRPC (`RefinerService.Refine`) | HTTP REST (`POST /api/refine`) |
|------|-------------------------------|-------------------------------|
| 协议 | gRPC + protobuf | HTTP + JSON |
| 端口 | `:15051` | `:18080`（与 dashboard 共用） |
| 客户端 | 需编译 proto、装 gRPC SDK | 任何 HTTP 客户端即可 |
| 入参 | 嵌套结构（`Message`, `RagChunk`, `ModelConfig`） | `system + messages + memory`，其中 `memory.rag` 支持字符串数组简写 |
| 出参 | 完整 `RefineResponse`（含 `audits`, `paged_chunks` 和结构化 output context） | 直接返回结构化 `system + messages + memory` 与元信息 |
| 适用场景 | 内部服务、性能敏感、需要完整审计 | 外部集成、快速接入、轻量调用 |

## 注意事项

1. **编码**：请求和响应均为 UTF-8，建议显式指定 `Content-Type: application/json; charset=utf-8`。
2. **trace_id**：返回的 `trace_id` 可以在 dashboard (`http://127.0.0.1:18080`) 的「按 Trace ID 打开评估」中粘贴查看详细清洗步骤。
3. **session_id**：同一 session 内的多次请求会共享前缀缓存，建议外部应用维护稳定的 session_id。
4. **budget**：如果不传 `budget`，服务端会从 `policy` 或 `model.max_context_tokens` 推导，但建议显式指定以避免歧义。
5. **memory.rag 格式**：字符串数组会自动包装成 `FRAGMENT_TYPE_BODY`；如需指定 `FRAGMENT_TYPE_CODE` / `FRAGMENT_TYPE_LOG` 等，请用对象数组格式。

## 完整工作流示例

```bash
# 1. 发起清洗请求
RESPONSE=$(curl -s -X POST http://127.0.0.1:18080/api/refine \
  -H "Content-Type: application/json" \
  -d '{
    "system": "你是代码助手",
    "messages": [{"role":"user","content":"排查问题"}],
    "memory": {"rag": ["日志片段"]},
    "budget": 200
  }')

# 2. 提取结构化上下文
SYSTEM=$(echo "$RESPONSE" | jq -r '.system')
MESSAGES=$(echo "$RESPONSE" | jq -c '.messages')
MEMORY=$(echo "$RESPONSE" | jq -c '.memory')

# 3. 将三段结构化结果交给你的调用方代码，自行按目标模型 API 组装
echo "$SYSTEM"
echo "$MESSAGES"
echo "$MEMORY"

# 4. 如需查看清洗详情，复制 trace_id 到 dashboard
TRACE_ID=$(echo "$RESPONSE" | jq -r '.trace_id')
echo "查看详情: http://127.0.0.1:18080/?trace=$TRACE_ID"
```

## 更新日志

- `2026-04-18`：外部 JSON 接口升级为单一结构 `system + messages + memory`；`memory.rag` 取代顶层 `rag`，旧兼容字段已删除。
- `2026-04-18`：响应主视图升级为结构化 `system + messages + memory`；调用方应自行按目标模型 API 拼接最终请求。
