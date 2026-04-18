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

### 完整示例（带 RAG 和可选参数）

```json
{
  "system": "你是一个偏后端的代码助手，回答必须中文，并保留关键栈信息。",
  "messages": [
    {"role": "user", "content": "我的 otel-collector 一直 EOF"},
    {"role": "assistant", "content": "先检查 4318 端口"},
    {"role": "user", "content": "已验证端口通，下面是日志"}
  ],
  "rag": [
    "2026-04-17 WARN otel-collector export EOF",
    "services:\n  otel-collector:\n    ports: [4318]"
  ],
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
| `system` | string | 可选 | 系统 prompt，会自动拼成第一条 `role:system` 消息 |
| `messages` | array | 必填* | 对话历史，每条包含 `role` 和 `content`<br>*如果提供了 `system`，`messages` 可以为空 |
| `rag` | array | 可选 | RAG 上下文片段，支持两种格式：<br>1. 字符串数组：`["片段1", "片段2"]`<br>2. 对象数组：`[{id, source, content, type}]` |
| `model` | string/object | 可选 | 模型名称（如 `"gpt-4o-mini"`）或对象 `{name, max_context_tokens}` |
| `budget` | int | 可选 | token 预算，缺省时从 policy/model 推导 |
| `policy` | string | 可选 | 清洗策略名称，缺省走 `default_policy` |
| `session_id` | string | 可选 | 会话 ID，缺省自动生成 `ext-{random}` |
| `request_id` | string | 可选 | 请求 ID，缺省自动生成 `req-{random}` |

## 响应格式

```json
{
  "prompt": "...",                  // 优化后的最终 prompt，直接喂给大模型
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

## 使用示例

### curl

```bash
curl -X POST http://127.0.0.1:18080/api/refine \
  -H "Content-Type: application/json; charset=utf-8" \
  -d '{
    "system": "你是代码助手",
    "messages": [{"role":"user","content":"帮我排查"}],
    "rag": ["日志片段"],
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
        "rag": ["日志片段"],
        "budget": 200,
    },
)
data = resp.json()
optimized_prompt = data["prompt"]
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
    rag: ["日志片段"],
    budget: 200,
  }),
});
const data = await response.json();
console.log(`优化后 prompt: ${data.prompt}`);
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
| 入参 | 嵌套结构（`RagFragment`, `FragmentType`, `ModelConfig`） | 扁平 JSON，支持字符串数组简写 |
| 出参 | 完整 `RefineResponse`（含 `audits`, `paged_chunks` 等） | 精简字段，只暴露集成方关心的 |
| 适用场景 | 内部服务、性能敏感、需要完整审计 | 外部集成、快速接入、轻量调用 |

## 注意事项

1. **编码**：请求和响应均为 UTF-8，建议显式指定 `Content-Type: application/json; charset=utf-8`。
2. **trace_id**：返回的 `trace_id` 可以在 dashboard (`http://127.0.0.1:18080`) 的「按 Trace ID 打开评估」中粘贴查看详细清洗步骤。
3. **session_id**：同一 session 内的多次请求会共享前缀缓存，建议外部应用维护稳定的 session_id。
4. **budget**：如果不传 `budget`，服务端会从 `policy` 或 `model.max_context_tokens` 推导，但建议显式指定以避免歧义。
5. **rag 格式**：字符串数组会自动包装成 `FRAGMENT_TYPE_BODY`；如需指定 `FRAGMENT_TYPE_CODE` / `FRAGMENT_TYPE_LOG` 等，请用对象数组格式。

## 完整工作流示例

```bash
# 1. 发起清洗请求
RESPONSE=$(curl -s -X POST http://127.0.0.1:18080/api/refine \
  -H "Content-Type: application/json" \
  -d '{
    "system": "你是代码助手",
    "messages": [{"role":"user","content":"排查问题"}],
    "rag": ["日志片段"],
    "budget": 200
  }')

# 2. 提取优化后的 prompt
PROMPT=$(echo "$RESPONSE" | jq -r '.prompt')

# 3. 喂给大模型（示例：OpenAI API）
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"gpt-4o-mini\",
    \"messages\": [{\"role\":\"user\",\"content\":$(echo "$PROMPT" | jq -Rs .)}]
  }"

# 4. 如需查看清洗详情，复制 trace_id 到 dashboard
TRACE_ID=$(echo "$RESPONSE" | jq -r '.trace_id')
echo "查看详情: http://127.0.0.1:18080/?trace=$TRACE_ID"
```

## 更新日志

- `2026-04-18`：首次发布，支持 system + messages + rag 三件套接入。
