package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	refinerv1 "context-refiner/api/refinerv1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type scenario struct {
	name    string
	session string
	budget  int32
	req     *refinerv1.RefineRequest
}

func main() {
	conn, err := grpc.NewClient("127.0.0.1:15051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial grpc failed: %v", err)
	}
	defer conn.Close()

	client := refinerv1.NewRefinerServiceClient(conn)
	scenarios := buildScenarios()
	fmt.Printf("mock_scenarios=%d\n", len(scenarios))

	for idx, item := range scenarios {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		request := item.req
		request.SessionId = item.session
		request.RequestId = fmt.Sprintf("%s-%d", item.session, time.Now().UnixNano())
		request.TokenBudget = item.budget

		resp, err := client.Refine(ctx, request)
		cancel()
		if err != nil {
			log.Fatalf("seed scenario %d (%s) failed: %v", idx+1, item.name, err)
		}

		fmt.Printf("scenario=%s\n", item.name)
		fmt.Printf("trace_id=%s\n", resp.GetMetadata()["trace_id"])
		fmt.Printf("request_id=%s\n", request.GetRequestId())
		fmt.Printf("input_tokens=%d output_tokens=%d saved_tokens=%d budget_met=%v paged_chunks=%d pending_summary_jobs=%d\n",
			resp.GetInputTokens(),
			resp.GetOutputTokens(),
			resp.GetInputTokens()-resp.GetOutputTokens(),
			resp.GetBudgetMet(),
			len(resp.GetPagedChunks()),
			len(resp.GetPendingSummaryJobIds()),
		)
		fmt.Println("---")
	}
}

func buildScenarios() []scenario {
	return []scenario{
		buildDockerIncidentScenario(),
		buildFrontendRegressionScenario(),
		buildSecurityAuditScenario(),
		buildDataPipelineScenario(),
	}
}

func buildDockerIncidentScenario() scenario {
	return scenario{
		name:    "docker-otel-timeout",
		session: "mock-docker-otel-timeout",
		budget:  240,
		req: &refinerv1.RefineRequest{
			Policy: "strict_coding_assistant",
			Model: &refinerv1.ModelConfig{
				Model:            "gpt-4o-mini",
				MaxContextTokens: 8192,
			},
			Messages: []*refinerv1.Message{
				{
					Role:    "system",
					Content: "你是一个偏运维和后端排障的代码助手。请在压缩上下文时保留：失败服务名、端口、容器名、调用路径、错误码、根因猜测、最后一次成功配置，以及用户明确指出不能丢的上下文。",
				},
				{
					Role:    "user",
					Content: "我在本机 Windows 上调试 Docker Compose 部署，context-refiner、otel-collector、tempo、redis 都在混跑。现在我需要一份可交给同事的精简上下文，但不能丢失 collector EOF、端口映射、compose 版本差异、以及最近一次 fallback summary 的线索。",
				},
				{
					Role:    "assistant",
					Content: "收到。保留重点包括：监听地址、容器名、镜像 tag、compose 配置片段、失败时序、是否只影响 tracing 还是连 dashboard 也影响、以及 Redis 中 pending jobs 的变化。",
				},
				{
					Role:    "user",
					Content: "下面是新补充的运行上下文。我要你压缩，但要保留从 collector 到 tempo 的链路问题、Redis 积压、以及用户偏好中文输出这几个重点。",
				},
			},
			RagChunks: []*refinerv1.RagChunk{
				{
					Id:      "docker-log-otel",
					Source:  "runtime/docker-log",
					Sources: []string{"runtime/docker-log", "runtime/compose"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type: refinerv1.FragmentType_FRAGMENT_TYPE_LOG,
							Content: strings.Join([]string{
								"2026-04-17T13:19:10Z INFO dashboard HTTP server listening on http://127.0.0.1:8080",
								"2026-04-17T13:19:11Z WARN otel-collector export failed: Post http://tempo:4318/v1/traces EOF",
								"2026-04-17T13:19:12Z WARN summary worker fallback summary scheduled due to upstream timeout",
								"2026-04-17T13:19:13Z ERROR redis queue backlog exceeded threshold=128 current=211",
								"2026-04-17T13:19:14Z INFO retrying trace export in 2s",
								"2026-04-17T13:19:16Z WARN otel-collector export failed: connectex no connection could be made because the target machine actively refused it",
							}, "\n") + "\n" + buildRepeatedLog("otel-collector", 32),
						},
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_ERROR_STACK,
							Content: "java.net.SocketTimeoutException: timeout while exporting trace batch\n\tat collector.export()\n\tat collector.retryLoop()\nCaused by: io.EOF\n\tat tempo.receiver.otlphttp()",
						},
					},
				},
				{
					Id:      "docker-compose-snippet",
					Source:  "repo/deploy",
					Sources: []string{"repo/deploy", "repo/config"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type:     refinerv1.FragmentType_FRAGMENT_TYPE_CODE,
							Language: "yaml",
							Content: strings.Join([]string{
								"services:",
								"  context-refiner:",
								"    ports:",
								"      - \"50051:50051\"",
								"      - \"8080:8080\"",
								"  otel-collector:",
								"    ports:",
								"      - \"4317:4317\"",
								"      - \"4318:4318\"",
								"  tempo:",
								"    ports:",
								"      - \"3200:3200\"",
								"observability:",
								"  tracing_enabled: true",
								"  tracing_endpoint: http://127.0.0.1:4318/v1/traces",
								"  tempo_query_url: http://localhost:3200",
								"redis:",
								"  addr: 127.0.0.1:6379",
							}, "\n"),
						},
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_TABLE,
							Content: "service | listen | container | state\ncontext-refiner | 50051/8080 | refiner | ok\notel-collector | 4317/4318 | collector | timeout\ntempo | 3200 | tempo | receiving partial data\nredis | 6379 | redis | backlog high",
						},
					},
				},
				{
					Id:      "docker-json-state",
					Source:  "runtime/state-dump",
					Sources: []string{"runtime/state-dump"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_JSON,
							Content: "{\"user_preferences\":{\"language\":\"zh-CN\",\"needs\":\"trace dashboard + cleanup evaluation\"},\"collector\":{\"grpc\":\"0.0.0.0:4317\",\"http\":\"0.0.0.0:4318\",\"last_error\":\"EOF\",\"retry\":\"2s\"},\"tempo\":{\"query\":\"http://localhost:3200\",\"trace_count\":18},\"redis\":{\"pending_jobs\":14,\"stream\":\"context-refiner:summary-jobs\",\"consumer_group\":\"context-refiner-summary\"},\"recent_actions\":[\"force recreate otel-collector\",\"restart refiner\",\"query trace search api\",\"open evaluation dashboard\"]}",
						},
					},
				},
			},
		},
	}
}

func buildFrontendRegressionScenario() scenario {
	return scenario{
		name:    "frontend-regression-analysis",
		session: "mock-frontend-regression",
		budget:  260,
		req: &refinerv1.RefineRequest{
			Policy: "strict_coding_assistant",
			Model: &refinerv1.ModelConfig{
				Model:            "gpt-4o-mini",
				MaxContextTokens: 8192,
			},
			Messages: []*refinerv1.Message{
				{
					Role:    "system",
					Content: "你是一个前端审查助手。请在压缩时保留：用户路径、复现步骤、关键 DOM 结构、报错栈、视觉回归说明、以及与字体/响应式相关的诉求。",
				},
				{
					Role:    "user",
					Content: "我们刚把 trace dashboard 改成中文评估面板，但用户说移动端字体过小、上下文对比区滚动不顺、步骤卡片太密。我要保留能支撑设计迭代的上下文，同时压缩成评估 prompt。",
				},
				{
					Role:    "assistant",
					Content: "重点保留：页面入口、断点行为、字体策略、颜色层次、组件间距、触摸滚动问题、以及具体哪个区块在 390px 宽度下出问题。",
				},
				{
					Role:    "user",
					Content: "下面有浏览器控制台、可访问性检查、DOM 结构和 CSS 片段。请保留最关键的前端证据链，不要把噪音信息全带上。",
				},
			},
			RagChunks: []*refinerv1.RagChunk{
				{
					Id:      "browser-console",
					Source:  "playwright/console",
					Sources: []string{"playwright/console", "playwright/a11y"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type: refinerv1.FragmentType_FRAGMENT_TYPE_LOG,
							Content: strings.Join([]string{
								"[console] trace card click works but context compare panel reflows twice",
								"[console] evaluation payload loaded in 118ms",
								"[console] resize observer loop completed with undelivered notifications",
								"[a11y] table headers have sufficient contrast",
								"[a11y] context-body font size under 12px on mobile would fail readability review",
							}, "\n") + "\n" + buildRepeatedConsoleNoise(28),
						},
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_TOOL_OUTPUT,
							Content: "viewport=390x844\nhero.height=332\ncompareGrid.columns=1\nstepCard.padding=18\ncontextBody.fontSize=11.5px\nstatusBanner.flexDirection=column\nscrollContainer=pre.context-body",
						},
					},
				},
				{
					Id:      "dom-outline",
					Source:  "ui/dom-snapshot",
					Sources: []string{"ui/dom-snapshot"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type:     refinerv1.FragmentType_FRAGMENT_TYPE_CODE,
							Language: "html",
							Content: strings.Join([]string{
								"<main class=\"content-grid\">",
								"  <section class=\"panel panel-wide\">",
								"    <div id=\"evaluation-metrics\" class=\"stats-grid\"></div>",
								"  </section>",
								"  <section class=\"panel panel-wide\">",
								"    <div id=\"context-compare\" class=\"compare-grid\">",
								"      <article class=\"context-card\">...</article>",
								"      <article class=\"context-card\">...</article>",
								"    </div>",
								"  </section>",
								"  <section class=\"panel panel-wide\">",
								"    <div id=\"step-audits\" class=\"step-list\"></div>",
								"  </section>",
								"</main>",
							}, "\n"),
						},
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_TABLE,
							Content: "breakpoint | selector | issue | recommendation\n390px | .context-body | font too small | clamp(13px,1vw,15px)\n390px | .step-semantic-grid | wraps harshly | use 1 column\n768px | .query-grid | ok | keep 2 columns\n1280px | .trace-results | ok | increase minmax width",
						},
					},
				},
				{
					Id:      "css-regression",
					Source:  "ui/styles",
					Sources: []string{"ui/styles"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type:     refinerv1.FragmentType_FRAGMENT_TYPE_CODE,
							Language: "css",
							Content: strings.Join([]string{
								".context-body {",
								"  font-family: \"IBM Plex Mono\", \"Consolas\", monospace;",
								"  font-size: clamp(12px, 0.95vw, 14px);",
								"  line-height: 1.68;",
								"  overflow: auto;",
								"}",
								".compare-grid {",
								"  display: grid;",
								"  grid-template-columns: repeat(2, minmax(0, 1fr));",
								"}",
								"@media (max-width: 1100px) {",
								"  .compare-grid { grid-template-columns: 1fr; }",
								"}",
							}, "\n"),
						},
					},
				},
			},
		},
	}
}

func buildSecurityAuditScenario() scenario {
	return scenario{
		name:    "security-audit-secrets",
		session: "mock-security-audit",
		budget:  210,
		req: &refinerv1.RefineRequest{
			Policy: "strict_coding_assistant",
			Model: &refinerv1.ModelConfig{
				Model:            "gpt-4o-mini",
				MaxContextTokens: 8192,
			},
			Messages: []*refinerv1.Message{
				{
					Role:    "system",
					Content: "你是一个安全审计助手。请在压缩上下文时保留：泄露风险、证据位置、影响范围、是否已脱敏、修复建议、和必须回归验证的路径。",
				},
				{
					Role:    "user",
					Content: "我在审计上下文清洗的副作用，担心日志、tool output、JSON 片段里把 token、cookie、内网地址、trace id 混进最终 prompt。请保留那些会影响安全结论的上下文。",
				},
				{
					Role:    "assistant",
					Content: "重点保留：疑似 secret 的字段名、出现场景、是否已归一化或掩码、以及用户要求必须继续支持中文展示和本机调试。",
				},
			},
			RagChunks: []*refinerv1.RagChunk{
				{
					Id:      "security-json",
					Source:  "audit/raw-json",
					Sources: []string{"audit/raw-json"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_JSON,
							Content: "{\"apiKey\":\"sk-live-should-not-leak\",\"cookie\":\"session=abcd.efgh.ijkl\",\"trace_id\":\"9f0c...masked\",\"internal_hosts\":[\"10.8.0.12\",\"10.8.0.18\"],\"payload\":{\"user\":\"ops-admin\",\"action\":\"dump dashboard metrics\",\"token\":\"ghp_mock_should_be_trimmed\"},\"audit\":{\"masking\":\"partial\",\"notes\":\"need stricter secret scrubbing before final prompt\"}}",
						},
					},
				},
				{
					Id:      "security-log",
					Source:  "audit/logs",
					Sources: []string{"audit/logs", "audit/tool-output"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_LOG,
							Content: "2026-04-17T12:31:10Z WARN outgoing request header Authorization: Bearer sk-live-should-not-leak\n2026-04-17T12:31:11Z INFO normalized trace_id into placeholder\n2026-04-17T12:31:12Z ERROR cookie persisted in tool output preview",
						},
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_TOOL_OUTPUT,
							Content: "curl -H 'Authorization: Bearer sk-live-should-not-leak' -H 'Cookie: session=abcd.efgh.ijkl' http://10.8.0.18/internal/debug",
						},
					},
				},
				{
					Id:      "security-guideline",
					Source:  "audit/checklist",
					Sources: []string{"audit/checklist"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_TABLE,
							Content: "risk | evidence | current state | next action\nsecret in JSON | apiKey/token field present | not masked enough | trim to keys only\nsecret in tool output | Authorization header visible | dangerous | redact values\ninternal hosts | 10.8.x.x leaked | partial masking | keep subnet only\ntrace id | normalized | acceptable | keep placeholder",
						},
					},
				},
			},
		},
	}
}

func buildDataPipelineScenario() scenario {
	return scenario{
		name:    "etl-data-quality",
		session: "mock-etl-data-quality",
		budget:  230,
		req: &refinerv1.RefineRequest{
			Policy: "strict_coding_assistant",
			Model: &refinerv1.ModelConfig{
				Model:            "gpt-4o-mini",
				MaxContextTokens: 8192,
			},
			Messages: []*refinerv1.Message{
				{
					Role:    "system",
					Content: "你是一个数据平台排障助手。压缩时保留：数据源、行数、坏数据类型、字段漂移、SQL 片段、关键报错、修复策略、以及回归检查项。",
				},
				{
					Role:    "user",
					Content: "昨晚 ETL 把订单上下文清洗错了，导致下游模型把字段 `price` 当成了字符串拼接。我要保留排障证据，但不能让上下文继续爆长。",
				},
				{
					Role:    "assistant",
					Content: "重点保留：schema 变化、样例脏数据、触发时间段、数据量级、清洗规则版本、以及 fallback 到 summary 的记录。",
				},
				{
					Role:    "user",
					Content: "这里有 SQL、CSV 摘要、错误日志、和一段很长的数据字典。请尽可能压缩，但别丢订单金额类型漂移、退款行混入、地区字段映射错误这些问题。",
				},
			},
			RagChunks: []*refinerv1.RagChunk{
				{
					Id:      "etl-sql",
					Source:  "warehouse/sql",
					Sources: []string{"warehouse/sql"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type:     refinerv1.FragmentType_FRAGMENT_TYPE_CODE,
							Language: "sql",
							Content:  "SELECT order_id, price, refund_amount, region_code, order_status, updated_at FROM ods_order_delta WHERE dt BETWEEN '2026-04-16' AND '2026-04-17';\nWITH typed AS (\n  SELECT order_id, CAST(price AS DECIMAL(18,2)) AS price_num, region_code FROM ods_order_delta\n)\nSELECT * FROM typed WHERE price_num IS NULL;",
						},
					},
				},
				{
					Id:      "etl-data-dict",
					Source:  "warehouse/data-dict",
					Sources: []string{"warehouse/data-dict", "warehouse/csv-sample"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_TABLE,
							Content: buildLargeDictionaryTable(),
						},
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_LOG,
							Content: "2026-04-17T01:11:07Z WARN schema drift detected field=price old=decimal new=string\n2026-04-17T01:11:12Z WARN refund rows mixed into active order feed\n2026-04-17T01:11:19Z ERROR region mapping failed code=HK-MO fallback=CN-SOUTH\n2026-04-17T01:11:30Z INFO summary fallback triggered for oversized incident context",
						},
					},
				},
				{
					Id:      "etl-json-state",
					Source:  "warehouse/job-state",
					Sources: []string{"warehouse/job-state"},
					Fragments: []*refinerv1.RagFragment{
						{
							Type:    refinerv1.FragmentType_FRAGMENT_TYPE_JSON,
							Content: "{\"job\":\"nightly-order-cleaning\",\"rows\":1287332,\"bad_rows\":2814,\"schema_version_before\":\"v17\",\"schema_version_after\":\"v18\",\"issues\":[\"price became string\",\"refund rows mixed\",\"region_code fallback wrong\"],\"owners\":[\"data-platform\",\"order-intelligence\"],\"timeline\":{\"start\":\"2026-04-17T01:00:00Z\",\"alert\":\"2026-04-17T01:11:19Z\",\"rollback\":\"2026-04-17T01:22:10Z\"}}",
						},
					},
				},
			},
		},
	}
}

func buildRepeatedLog(service string, count int) string {
	lines := make([]string, 0, count)
	for i := 0; i < count; i++ {
		lines = append(lines, fmt.Sprintf("2026-04-17T13:%02d:%02dZ WARN %s retry=%d export batch size=%d state=degraded", 20+i/2, 10+i, service, i+1, 24+i))
	}
	return strings.Join(lines, "\n")
}

func buildRepeatedConsoleNoise(count int) string {
	lines := make([]string, 0, count)
	for i := 0; i < count; i++ {
		lines = append(lines, fmt.Sprintf("[console] layout sample %d selector=.step-card height=%d width=%d", i+1, 180+i, 320+i))
	}
	return strings.Join(lines, "\n")
}

func buildLargeDictionaryTable() string {
	rows := []string{
		"field | expected_type | observed_type | note",
		"order_id | string | string | stable",
		"price | decimal(18,2) | string | drift after upstream csv merge",
		"refund_amount | decimal(18,2) | decimal(18,2) | stable",
		"region_code | string | string | fallback mapping wrong for hk/mo",
		"order_status | enum | enum | stable",
	}
	for i := 0; i < 26; i++ {
		rows = append(rows, fmt.Sprintf("ext_attr_%02d | string | string | low-value attribute repeated for dictionary expansion", i+1))
	}
	return strings.Join(rows, "\n")
}
