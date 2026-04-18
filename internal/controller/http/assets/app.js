const overviewCards = document.getElementById("overview-cards");
const generatedAt = document.getElementById("generated-at");
const refreshButton = document.getElementById("refresh-btn");
const traceSearchForm = document.getElementById("trace-search-form");
const traceQueryInput = document.getElementById("trace-query-input");
const traceSearchButton = document.getElementById("trace-search-btn");
const traceIdForm = document.getElementById("trace-id-form");
const traceIdInput = document.getElementById("trace-id-input");
const traceResults = document.getElementById("trace-results");
const searchSummary = document.getElementById("search-summary");
const evaluationStatus = document.getElementById("evaluation-status");
const evaluationMetrics = document.getElementById("evaluation-metrics");
const evaluationKVCache = document.getElementById("evaluation-kvcache");
const evaluationKVCacheDetail = document.getElementById("evaluation-kv-cache");
const kvcacheStats = document.getElementById("kvcache-stats");
const kvcacheDetail = document.getElementById("kvcache-detail");

const PREFIX_LOOKUP_LABEL = {
  hit: "命中",
  miss: "未命中",
  skipped: "跳过",
};
const PREFIX_ADMISSION_LABEL = {
  admitted: "已收纳",
  skipped: "未收纳",
};
const PREFIX_MISS_REASON_LABEL = {
  none: "—",
  empty: "前缀为空",
  short_prefix: "前缀过短",
  low_value_prefix: "低价值前缀",
  created: "首次写入",
  ttl_expired: "TTL 过期",
  hash_changed: "哈希变化",
  model_changed: "模型变化",
  system_changed: "system 变化",
  memory_changed: "memory 变化",
  rag_changed: "rag 变化",
  normalization_changed: "归一化变化",
};
const PREFIX_TTL_TIER_LABEL = {
  hot: "热点",
  warm: "温热",
  cold: "冷",
  default: "默认",
};
const PREFIX_PREDICTION_LABEL = {
  predicted_hit: "预计命中",
  predicted_miss: "预计未命中",
  partial_reusable: "部分可复用",
  unstable_prefix: "前缀不稳定",
  unknown: "未知",
};
const contextCompare = document.getElementById("context-compare");
const stepAudits = document.getElementById("step-audits");
const evaluationExtra = document.getElementById("evaluation-extra");

const defaultTraceQuery = '{ name = "refiner.refine" }';

traceQueryInput.value = defaultTraceQuery;

async function requestJSON(url) {
  const response = await fetch(url);
  const payload = await response.json();
  if (!response.ok) {
    throw new Error(payload.error || "请求失败");
  }
  return payload;
}

async function loadSnapshot() {
  const payload = await requestJSON("/api/snapshot");
  renderOverview(payload.overview || {});
  renderKVCache(payload);
  generatedAt.textContent = `最近刷新：${formatTime(payload.overview?.generated_at)}`;
}

async function searchTraces(query) {
  const traceQL = query.trim() || defaultTraceQuery;
  const params = new URLSearchParams({
    q: traceQL,
    limit: "20",
    spss: "3",
  });
  return requestJSON(`/api/traces/search?${params.toString()}`);
}

async function loadTraceDetail(traceId) {
  return requestJSON(`/api/traces/detail?id=${encodeURIComponent(traceId)}`);
}

async function loadTraceEvaluation(traceId) {
  return requestJSON(`/api/traces/evaluation?id=${encodeURIComponent(traceId)}`);
}

function renderOverview(overview) {
  const cards = [
    {
      label: "面板地址",
      value: overview.web_listen_addr || "-",
      meta: "本机 Web 入口",
      accent: "",
    },
    {
      label: "Tempo 地址",
      value: overview.tempo_query_url || "-",
      meta: "TraceQL 查询与 trace 详情读取",
      accent: "accent-teal",
    },
    {
      label: "Tracing 状态",
      value: overview.tracing_enabled ? "已开启" : "未开启",
      meta: overview.tracing_enabled ? "新的清洗请求会写入 trace" : "请先检查 OTLP 配置",
      accent: overview.tracing_enabled ? "accent-ok" : "accent-warn",
    },
    {
      label: "Summary Worker",
      value: overview.summary_worker_enabled ? "运行中" : "未启用",
      meta: `默认策略：${overview.default_policy || "-"}`,
      accent: overview.summary_worker_enabled ? "accent-ok" : "accent-warn",
    },
    {
      label: "已采集页面",
      value: String(overview.page_artifact_count || 0),
      meta: `摘要 ${overview.summary_artifact_count || 0} · 前缀 ${overview.prefix_entry_count || 0}`,
      accent: "",
    },
    {
      label: "Metrics",
      value: overview.metrics_listen_addr || "-",
      meta: `路径 ${overview.metrics_path || "-"} · Redis ${overview.redis_addr || "-"}`,
      accent: "accent-teal",
    },
  ];

  overviewCards.innerHTML = cards.map((item) => `
    <article class="stat-card ${item.accent}">
      <span>${escapeHtml(item.label)}</span>
      <strong>${escapeHtml(String(item.value))}</strong>
      <div class="metric-meta">${escapeHtml(item.meta)}</div>
    </article>
  `).join("");
}

function renderKVCache(payload) {
  if (!kvcacheStats || !kvcacheDetail) return;
  const overview = payload.overview || {};
  const recent = Array.isArray(payload.recent_prefixes) ? payload.recent_prefixes : [];
  const hot = Array.isArray(payload.hot_prefixes) ? payload.hot_prefixes : [];
  const queue = payload.summary_queue || {};

  const totalHits = recent.reduce((sum, item) => sum + (Number(item.hit_count) || 0), 0);
  const totalTokens = recent.reduce((sum, item) => sum + (Number(item.stable_prefix_tokens) || 0), 0);
  const avgTokens = recent.length ? Math.round(totalTokens / recent.length) : 0;
  const maxHotScore = hot.reduce((m, item) => Math.max(m, Number(item.hot_score) || 0), 0);

  const stats = [
    { label: "缓存条目", value: String(overview.prefix_entry_count || recent.length || 0), meta: "全量前缀缓存数量", accent: "accent-teal" },
    { label: "热点数量", value: String(hot.length), meta: `最高 hot_score ${maxHotScore}`, accent: hot.length ? "accent-warn" : "" },
    { label: "累计命中", value: totalHits.toLocaleString(), meta: `最近 ${recent.length} 条记录合计`, accent: "accent-ok" },
    { label: "平均稳定前缀", value: `${avgTokens.toLocaleString()} tok`, meta: "stable_prefix_tokens 平均值", accent: "" },
    { label: "Summary 队列", value: String(queue.stream_length || 0), meta: `待处理 ${queue.pending_count || 0} · 消费者 ${queue.consumer_count || 0}`, accent: (queue.pending_count || 0) > 0 ? "accent-warn" : "accent-ok" },
    { label: "默认策略", value: overview.default_policy || "-", meta: `租户 ${overview.default_tenant || "-"}`, accent: "" },
  ];

  kvcacheStats.innerHTML = stats.map((item) => `
    <article class="stat-card ${item.accent}">
      <span>${escapeHtml(item.label)}</span>
      <strong>${escapeHtml(String(item.value))}</strong>
      <div class="metric-meta">${escapeHtml(item.meta)}</div>
    </article>
  `).join("");

  kvcacheDetail.innerHTML = `
    <article class="kvcache-card">
      <div class="kvcache-card-head">
        <h3>热点前缀 · Hot Prefixes</h3>
        <span class="metric-meta">按 hot_score 排序 · ${hot.length} 条</span>
      </div>
      <div class="prefix-table">${renderPrefixRows(hot, { sortBy: "hot_score", emptyHint: "还没有命中热点阈值的前缀。" })}</div>
    </article>
    <article class="kvcache-card">
      <div class="kvcache-card-head">
        <h3>Summary 队列</h3>
        <span class="metric-meta">${escapeHtml(queue.stream || "-")}</span>
      </div>
      <div class="queue-kv">
        ${renderQueueRow("stream_length", queue.stream_length)}
        ${renderQueueRow("pending", queue.pending_count)}
        ${renderQueueRow("consumer_group", queue.consumer_group)}
        ${renderQueueRow("consumers", queue.consumer_count)}
        ${renderQueueRow("last_id", queue.last_generated_id)}
        ${renderQueueRow("oldest_pending", queue.oldest_pending_id)}
        ${renderQueueRow("newest_pending", queue.newest_pending_id)}
      </div>
    </article>
    <article class="kvcache-card" style="grid-column: 1 / -1;">
      <div class="kvcache-card-head">
        <h3>最近入驻前缀 · Recent Prefixes</h3>
        <span class="metric-meta">按 last_seen 倒序 · ${recent.length} 条</span>
      </div>
      <div class="prefix-table">${renderPrefixRows(recent, { sortBy: "last_seen", emptyHint: "暂无前缀缓存记录。" })}</div>
    </article>
  `;
}

function renderPrefixRows(items, { sortBy, emptyHint }) {
  if (!items.length) {
    return `<div class="empty-state-card" style="margin:0;">${escapeHtml(emptyHint)}</div>`;
  }
  const sorted = items.slice();
  if (sortBy === "hot_score") {
    sorted.sort((a, b) => (Number(b.hot_score) || 0) - (Number(a.hot_score) || 0));
  } else {
    sorted.sort((a, b) => new Date(b.last_seen_at || 0) - new Date(a.last_seen_at || 0));
  }
  return sorted.slice(0, 12).map((item) => {
    const tier = String(item.cache_tier || "default").toLowerCase();
    const tierClass = tier === "hot" ? "tier-hot" : "tier-default";
    return `
      <div class="prefix-row ${item.hot ? "is-hot" : ""}">
        <div>
          <div class="prefix-hash">${escapeHtml(compactId(item.prefix_hash || "-", 20))}<span class="tier-pill ${tierClass}">${escapeHtml(tier)}</span></div>
          <div class="prefix-namespace">${escapeHtml(item.model_id || "-")} · ${escapeHtml(compactId(item.namespace || "", 48))}</div>
        </div>
        <div class="prefix-tokens"><strong>${(Number(item.stable_prefix_tokens) || 0).toLocaleString()}</strong><span>tokens</span></div>
        <div class="prefix-hits"><strong>${(Number(item.hit_count) || 0).toLocaleString()}</strong><span>hits</span></div>
        <div class="prefix-score"><strong>${formatScore(item.hot_score)}</strong><span>score</span></div>
      </div>
    `;
  }).join("");
}

function renderQueueRow(label, value) {
  const display = value === undefined || value === null || value === "" ? "-" : String(value);
  return `<div class="kv-row"><span>${escapeHtml(label)}</span><strong>${escapeHtml(display)}</strong></div>`;
}

function formatScore(value) {
  const num = Number(value || 0);
  if (!Number.isFinite(num)) return "0";
  if (num >= 100) return num.toFixed(0);
  return num.toFixed(num < 10 ? 2 : 1);
}

function renderTraceResults(payload) {
  const items = payload.traces || [];
  const metrics = payload.metrics || {};
  const inspectedBytes = metrics.inspectedBytes ? `，扫描 ${metrics.inspectedBytes} bytes` : "";
  searchSummary.textContent = `当前查询：${payload.query?.query || defaultTraceQuery}，命中 ${items.length} 条 trace${inspectedBytes}。`;

  if (!items.length) {
    traceResults.innerHTML = `<div class="empty-state-card">当前查询没有命中 trace。可以先触发一次清洗请求，再重新查询。</div>`;
    return;
  }

  traceResults.innerHTML = items.map((item) => `
    <article class="trace-card">
      <div class="trace-card-head">
        <div class="trace-card-main">
          <h3>${escapeHtml(getTraceTitle(item))}</h3>
          <p class="metric-meta">${escapeHtml(getTraceServiceName(item))} · ${escapeHtml(formatTime(item.start_time))}</p>
        </div>
        <button class="secondary-btn trace-open-btn" type="button" data-trace-id="${escapeHtml(item.trace_id)}">打开评估</button>
      </div>
      <div class="trace-meta-grid">
        ${renderMiniMetric("Trace ID", compactId(item.trace_id, 18))}
        ${renderMiniMetric("总耗时", formatDuration(item.duration_ms))}
        ${renderMiniMetric("Span 数量", String(item.span_count || 0))}
        ${renderMiniMetric("服务数量", String(item.service_count || 0))}
        ${renderMiniMetric("命中 Span", String(item.matched_spans || 0))}
        ${renderMiniMetric("错误 Span", String(item.error_count || 0))}
      </div>
      <div class="trace-card-preview">${escapeHtml(getTracePreview(item))}</div>
    </article>
  `).join("");

  document.querySelectorAll(".trace-open-btn").forEach((button) => {
    button.addEventListener("click", async () => {
      const traceId = button.dataset.traceId;
      traceIdInput.value = traceId;
      await showEvaluation(traceId);
    });
  });
}

function renderMiniMetric(label, value) {
  return `
    <div class="mini-metric">
      <span>${escapeHtml(label)}</span>
      <strong>${escapeHtml(value)}</strong>
    </div>
  `;
}

async function showEvaluation(traceId, { silent = false } = {}) {
  currentTraceId = traceId;
  if (!silent) {
    evaluationStatus.innerHTML = `<div class="empty-state-card">正在加载评估数据...</div>`;
    evaluationMetrics.innerHTML = "";
    evaluationKVCache.innerHTML = "";
    if (evaluationKVCacheDetail) {
      evaluationKVCacheDetail.innerHTML = `<article class="empty-state-card">正在加载 KV Cache 指标...</article>`;
    }
    contextCompare.innerHTML = `
      <article class="context-card empty-state-card">正在读取清洗前上下文...</article>
      <article class="context-card empty-state-card">正在读取清洗后上下文...</article>
    `;
    stepAudits.innerHTML = "";
    evaluationExtra.innerHTML = "";
  }

  const [detailResult, evaluationResult] = await Promise.allSettled([
    loadTraceDetail(traceId),
    loadTraceEvaluation(traceId),
  ]);

  const detail = detailResult.status === "fulfilled" ? detailResult.value : null;
  const evaluation = evaluationResult.status === "fulfilled" ? evaluationResult.value : null;

  if (!evaluation) {
    const reason = evaluationResult.status === "rejected" ? evaluationResult.reason?.message || "未找到评估快照" : "未找到评估快照";
    evaluationStatus.innerHTML = `
      <div class="empty-state-card">
        这条 trace 已经能查到，但还没有关联的上下文清洗评估快照。
        <div class="metric-meta">原因：${escapeHtml(reason)}。请先触发一次新的 Refine 请求，系统会把清洗前后上下文按 trace 落库，再回来查看。</div>
      </div>
    `;
    if (detail) {
      renderFallbackMetrics(detail);
    }
    evaluationKVCache.innerHTML = "";
    if (evaluationKVCacheDetail) {
      evaluationKVCacheDetail.innerHTML = `<article class="empty-state-card">这条 trace 还没有关联的 KV Cache 评估快照。</article>`;
    }
    return;
  }

  renderEvaluation(traceId, detail, evaluation);
}

function renderFallbackMetrics(detail) {
  const cards = [
    { label: "Trace ID", value: detail.trace_id || "-" },
    { label: "入口服务", value: detail.root_service_name || "-" },
    { label: "根 Span", value: detail.root_span_name || "-" },
    { label: "总耗时", value: formatDuration(detail.duration_ms) },
    { label: "Span 数量", value: String(detail.span_count || 0) },
    { label: "错误 Span", value: String(detail.error_count || 0) },
  ];
  evaluationMetrics.innerHTML = cards.map((item) => `
    <article class="stat-card compact">
      <span>${escapeHtml(item.label)}</span>
      <strong>${escapeHtml(item.value)}</strong>
    </article>
  `).join("");
}

function renderEvaluation(traceId, detail, evaluation) {
  const savedTokens = Number(evaluation.saved_tokens || 0);
  const inputTokens = Number(evaluation.input_tokens || 0);
  const outputTokens = Number(evaluation.output_tokens || 0);
  const reductionPercent = inputTokens > 0 ? ((savedTokens / inputTokens) * 100) : 0;

  evaluationStatus.innerHTML = `
    <div class="status-banner">
      <div>
        <strong>${escapeHtml(detail?.root_span_name || getTraceLabelFromEvaluation(evaluation))}</strong>
        <div class="metric-meta">Trace ${escapeHtml(traceId)} · Request ${escapeHtml(evaluation.request_id || "-")} · Session ${escapeHtml(evaluation.session_id || "-")}</div>
      </div>
      <span class="status-pill ${evaluation.budget_met ? "status-ok" : "status-warn"}">${evaluation.budget_met ? "预算内" : "超预算"}</span>
    </div>
    <div class="reduction-bar">
      <div class="reduction-bar-head">
        <span>Token 压缩比</span>
        <strong>${formatPercentNumber(reductionPercent)} · 节省 ${savedTokens.toLocaleString()} / ${inputTokens.toLocaleString()}</strong>
      </div>
      <div class="reduction-bar-track">
        <div class="reduction-bar-fill" style="width: ${Math.min(100, Math.max(0, reductionPercent)).toFixed(1)}%"></div>
      </div>
    </div>
  `;

  const metricCards = [
    { label: "输入 Tokens", value: inputTokens.toLocaleString(), meta: "清洗前整体上下文大小", accent: "" },
    { label: "输出 Tokens", value: outputTokens.toLocaleString(), meta: "清洗后最终 prompt 大小", accent: "accent-teal" },
    { label: "节省 Tokens", value: savedTokens.toLocaleString(), meta: `压缩比例 ${formatPercentNumber(reductionPercent)}`, accent: "accent-ok" },
    { label: "预算", value: String(evaluation.budget || 0), meta: evaluation.budget_met ? "当前结果落在预算内" : "当前结果仍高于预算", accent: evaluation.budget_met ? "accent-ok" : "accent-warn" },
    { label: "消息数量", value: String(evaluation.message_count || 0), meta: `RAG Chunk ${evaluation.rag_chunk_count || 0}`, accent: "" },
    { label: "分页 Chunk", value: String((evaluation.paged_chunks || []).length), meta: `待摘要任务 ${metadataInt(evaluation.metadata, "pending_summary_jobs")}`, accent: "" },
    { label: "Prefix Cache", value: lookupLabel(PREFIX_LOOKUP_LABEL, evaluation.metadata?.prefix_cache_lookup, evaluation.metadata?.prefix_cache_lookup || "-"), meta: `稳定前缀 ${metadataInt(evaluation.metadata, "stable_prefix_tokens")} tokens`, accent: "accent-teal" },
    { label: "Trace 耗时", value: formatDuration(detail?.duration_ms || 0), meta: `Span ${detail?.span_count || 0} · 服务 ${detail?.service_count || 0}`, accent: "" },
  ];

  evaluationMetrics.innerHTML = metricCards.map((item) => `
    <article class="stat-card ${item.accent}">
      <span>${escapeHtml(item.label)}</span>
      <strong>${escapeHtml(item.value)}</strong>
      <div class="metric-meta">${escapeHtml(item.meta)}</div>
    </article>
  `).join("");

  evaluationKVCache.innerHTML = renderKVCacheStrip(evaluation.metadata || {}, evaluation);
  if (evaluationKVCacheDetail) {
    evaluationKVCacheDetail.innerHTML = renderKVCacheMetrics(evaluation.metadata || {}, evaluation);
  }

  contextCompare.innerHTML = renderContextCompare(
    evaluation,
    evaluation.before_context || "",
    evaluation.after_context || "",
    inputTokens,
    outputTokens,
  );

  stepAudits.innerHTML = renderStepAudits(evaluation.steps || []);
  evaluationExtra.innerHTML = renderExtraPanels(evaluation, detail, reductionPercent);
}

function lookupLabel(map, value, fallback) {
  const key = String(value || "").trim().toLowerCase();
  if (!key) return fallback;
  return map[key] || value;
}

function renderKVCacheStrip(metadata, evaluation) {
  const lookupRaw = String(metadata.prefix_cache_lookup || "skipped").toLowerCase();
  const lookupLabelText = lookupLabel(PREFIX_LOOKUP_LABEL, lookupRaw, "未知");
  const stableTokens = metadataInt(metadata, "stable_prefix_tokens");
  const reusableTokens = metadataInt(metadata, "predicted_reusable_tokens");
  const inputTokens = Number(evaluation?.input_tokens || 0);
  const reusePercent = inputTokens > 0
    ? Math.min(100, (reusableTokens / inputTokens) * 100)
    : 0;
  const hot = String(metadata.prefix_cache_hot || "").toLowerCase() === "true";
  const hotScore = metadataInt(metadata, "prefix_cache_hot_score");
  const hitCount = metadataInt(metadata, "prefix_cache_hit_count");

  const stateClass = lookupRaw === "hit" ? "is-hit" : lookupRaw === "miss" ? "is-miss" : "is-skip";

  const items = [
    { label: "缓存判定", value: lookupLabelText, cls: stateClass },
    { label: "稳定前缀", value: `${stableTokens.toLocaleString()} tok` },
    { label: "可复用 Tokens", value: `${reusableTokens.toLocaleString()} (${formatPercentNumber(reusePercent)})` },
    { label: "命中次数", value: hitCount.toLocaleString() },
    { label: "热度", value: hot ? `热点 · ${hotScore}` : `普通 · ${hotScore}`, cls: hot ? "is-hit" : "" },
  ];

  return items.map((item) => `
    <div class="kv-item ${item.cls || ""}">
      <span>${escapeHtml(item.label)}</span>
      <strong>${escapeHtml(String(item.value))}</strong>
    </div>
  `).join("");
}

function renderKVCacheMetrics(metadata, evaluation) {
  const stablePrefixTokens = metadataInt(metadata, "stable_prefix_tokens");
  const predictedReusableTokens = metadataInt(metadata, "predicted_reusable_tokens");
  const cacheHitCount = metadataInt(metadata, "prefix_cache_hit_count");
  const systemPrefixTokens = metadataInt(metadata, "system_prefix_tokens");
  const memoryPrefixTokens = metadataInt(metadata, "memory_prefix_tokens");
  const ragPrefixTokens = metadataInt(metadata, "rag_prefix_tokens");
  const hotScore = metadataInt(metadata, "prefix_cache_hot_score");
  const pageCount = Array.isArray(evaluation?.paged_chunks) ? evaluation.paged_chunks.length : 0;
  const inputTokens = Number(evaluation?.input_tokens || 0);

  const lookupRaw = String(metadata.prefix_cache_lookup || "").toLowerCase();
  const admissionRaw = String(metadata.prefix_cache_admission || "").toLowerCase();
  const missReasonRaw = String(metadata.prefix_cache_miss_reason || "").toLowerCase();
  const ttlTierRaw = String(metadata.prefix_cache_ttl_tier || "").toLowerCase();
  const predictionRaw = String(metadata.cache_prediction_result || "").toLowerCase();

  const lookupLabelText = lookupLabel(PREFIX_LOOKUP_LABEL, lookupRaw, "未知");
  const admissionLabel = lookupLabel(PREFIX_ADMISSION_LABEL, admissionRaw, admissionRaw || "—");
  const missReasonLabel = (missReasonRaw && missReasonRaw !== "none")
    ? lookupLabel(PREFIX_MISS_REASON_LABEL, missReasonRaw, missReasonRaw)
    : "";
  const ttlTierLabel = lookupLabel(PREFIX_TTL_TIER_LABEL, ttlTierRaw, ttlTierRaw || "—");
  const predictionLabel = lookupLabel(PREFIX_PREDICTION_LABEL, predictionRaw, predictionRaw || "—");

  const isHot = String(metadata.prefix_cache_hot || "").toLowerCase() === "true";
  const lookupAccent = lookupRaw === "hit" ? "accent-ok" : lookupRaw === "miss" ? "accent-warn" : "";
  // 复用率：以输入 token 为分母，反映"本次输入中有多少 tokens 可以从缓存复用"
  const reusePercent = inputTokens > 0
    ? Math.min(100, (predictedReusableTokens / inputTokens) * 100)
    : 0;
  const stableShare = inputTokens > 0
    ? Math.min(100, (stablePrefixTokens / inputTokens) * 100)
    : 0;

  const lookupMetaParts = [`收纳：${admissionLabel}`];
  if (missReasonLabel) {
    lookupMetaParts.push(`原因：${missReasonLabel}`);
  }
  if (predictionLabel && predictionLabel !== "—") {
    lookupMetaParts.push(`预测：${predictionLabel}`);
  }

  const ttlSeconds = Number(metadata.prefix_cache_applied_ttl_seconds || 0);
  const ttlDisplay = ttlSeconds > 0 ? `TTL ${ttlSeconds.toLocaleString()} 秒` : "TTL 未设置";

  const cards = [
    {
      label: "缓存判定",
      value: lookupLabelText,
      meta: lookupMetaParts.join(" · "),
      accent: lookupAccent,
    },
    {
      label: "稳定前缀 Tokens",
      value: stablePrefixTokens.toLocaleString(),
      meta: `占输入 ${formatPercentNumber(stableShare)}`,
      accent: "accent-teal",
    },
    {
      label: "预计可复用 Tokens",
      value: predictedReusableTokens.toLocaleString(),
      meta: `输入复用率 ${formatPercentNumber(reusePercent)}`,
      accent: predictedReusableTokens > 0 ? "accent-ok" : "",
    },
    {
      label: "缓存热度",
      value: isHot ? "热点" : "普通",
      meta: `命中 ${cacheHitCount.toLocaleString()} 次 · hot_score ${hotScore.toLocaleString()}`,
      accent: isHot ? "accent-ok" : "",
    },
    {
      label: "TTL 层级",
      value: ttlTierLabel,
      meta: ttlDisplay,
      accent: "",
    },
    {
      label: "前缀构成 Tokens",
      value: `${systemPrefixTokens.toLocaleString()} / ${memoryPrefixTokens.toLocaleString()} / ${ragPrefixTokens.toLocaleString()}`,
      meta: "system / memory / rag",
      accent: "",
    },
    {
      label: "命名空间",
      value: shortenIdentifier(metadata.prefix_cache_namespace || "—"),
      meta: `tenant ${metadata.prefix_cache_tenant || "—"} · model ${metadata.prefix_cache_model || evaluation?.model_name || "—"}`,
      accent: "",
    },
    {
      label: "分页 / 待摘要",
      value: `${pageCount} / ${metadataInt(metadata, "pending_summary_jobs")}`,
      meta: "paged chunks / pending summary jobs",
      accent: "",
    },
  ];

  const extraRows = [
    ["系统前缀 Hash", shortenIdentifier(metadata.system_prefix_hash)],
    ["记忆前缀 Hash", shortenIdentifier(metadata.memory_prefix_hash)],
    ["RAG 前缀 Hash", shortenIdentifier(metadata.rag_prefix_hash)],
    ["组合前缀 Hash", shortenIdentifier(metadata.combined_prefix_hash)],
    ["缓存 Key", shortenIdentifier(metadata.prefix_cache_key)],
    ["预测结果", predictionLabel],
    ["Segment 数", metadata.prefix_cache_segment_count],
    ["Segment 主因", metadata.prefix_cache_segment_reason],
    ["System 变化", formatBoolText(metadata.prefix_cache_system_changed)],
    ["Memory 变化", formatBoolText(metadata.prefix_cache_memory_changed)],
    ["RAG 变化", formatBoolText(metadata.prefix_cache_rag_changed)],
  ].filter(([, value]) => value !== undefined && value !== null && String(value).trim() !== "" && String(value).trim() !== "—");

  return `
    <div class="stats-grid">
      ${cards.map((item) => `
        <article class="stat-card ${item.accent}">
          <span>${escapeHtml(item.label)}</span>
          <strong>${escapeHtml(String(item.value))}</strong>
          <div class="metric-meta">${escapeHtml(item.meta)}</div>
        </article>
      `).join("")}
    </div>
    <details class="kv-cache-details">
      <summary>查看 KV Cache 关键明细</summary>
      <div class="metadata-kv">
        ${extraRows.map(([key, value]) => `
          <div class="kv-row">
            <span>${escapeHtml(key)}</span>
            <strong>${escapeHtml(String(value))}</strong>
          </div>
        `).join("")}
      </div>
    </details>
  `;
}

function shortenIdentifier(value) {
  const str = String(value || "").trim();
  if (!str) return "—";
  if (str.length <= 24) return str;
  return `${str.slice(0, 12)}…${str.slice(-8)}`;
}

function renderContextCompare(evaluation, before, after, beforeTokens, afterTokens) {
  const inputContext = evaluation.input_context || {};
  const outputContext = evaluation.output_context || {};

  return `
    <div class="context-compare-stack">
      <div class="structured-grid">
        ${renderStructuredContextCard("输入结构", inputContext, beforeTokens)}
        ${renderStructuredContextCard("输出结构", outputContext, afterTokens)}
      </div>
      <div class="raw-diff-grid">
        ${renderDiffPanes(before, after, beforeTokens, afterTokens)}
      </div>
    </div>
  `;
}

function renderStructuredContextCard(title, context, tokens) {
  const system = String(context?.system || "").trim();
  const messages = Array.isArray(context?.messages) ? context.messages : [];
  const ragChunks = Array.isArray(context?.memory?.rag) ? context.memory.rag : [];
  const summary = [
    `System ${system ? 1 : 0}`,
    `Messages ${messages.length}`,
    `Memory.RAG ${ragChunks.length}`,
    `Tokens ${Number(tokens || 0).toLocaleString()}`,
  ].join(" · ");

  return `
    <article class="context-card structured-card">
      <div class="context-head">
        <div>
          <h3>${escapeHtml(title)}</h3>
          <p class="metric-meta">${escapeHtml(summary)}</p>
        </div>
      </div>
      <div class="context-body structured-body">
        ${renderStructuredBlock("system", system ? `<pre class="structured-text">${escapeHtml(system)}</pre>` : emptyStructured("暂无 system 内容"))}
        ${renderStructuredBlock("messages", renderStructuredMessages(messages))}
        ${renderStructuredBlock("memory", renderStructuredMemory(ragChunks))}
      </div>
    </article>
  `;
}

function renderStructuredBlock(label, body) {
  return `
    <section class="structured-block">
      <div class="structured-label">${escapeHtml(label)}</div>
      ${body}
    </section>
  `;
}

function renderStructuredMessages(messages) {
  if (!messages.length) {
    return emptyStructured("暂无 messages");
  }
  return `
    <div class="structured-list">
      ${messages.map((message, index) => `
        <article class="message-entry">
          <div class="message-entry-head">
            <span class="role-pill">${escapeHtml(formatMessageRole(message.role))}</span>
            <span class="metric-meta">#${index + 1}</span>
          </div>
          <pre class="structured-text">${escapeHtml(String(message.content || "").trim() || " ")}</pre>
        </article>
      `).join("")}
    </div>
  `;
}

function renderStructuredMemory(ragChunks) {
  if (!ragChunks.length) {
    return emptyStructured("暂无 memory.rag");
  }
  return `
    <div class="structured-list">
      ${ragChunks.map((chunk, index) => {
        const sources = Array.isArray(chunk?.sources) && chunk.sources.length
          ? chunk.sources.join(", ")
          : (chunk?.source || "unknown");
        const fragments = Array.isArray(chunk?.fragments) ? chunk.fragments : [];
        const pageRefs = Array.isArray(chunk?.page_refs) ? chunk.page_refs : [];
        return `
          <article class="memory-entry">
            <div class="memory-entry-head">
              <strong>${escapeHtml(chunk?.id || `rag-${index + 1}`)}</strong>
              <span class="metric-meta">${escapeHtml(sources)}</span>
            </div>
            <div class="memory-entry-meta">
              <span>${escapeHtml(`Fragments ${fragments.length}`)}</span>
              <span>${escapeHtml(`Pages ${pageRefs.length}`)}</span>
            </div>
            <div class="fragment-list">
              ${fragments.map((fragment) => `
                <div class="fragment-chip">
                  <span class="fragment-chip-type">${escapeHtml(String(fragment?.type || "body"))}</span>
                  <span class="fragment-chip-content">${escapeHtml(truncateStructuredText(fragment?.content || "", 120))}</span>
                </div>
              `).join("") || `<div class="metric-meta">无 fragment</div>`}
            </div>
          </article>
        `;
      }).join("")}
    </div>
  `;
}

function emptyStructured(label) {
  return `<div class="structured-empty">${escapeHtml(label)}</div>`;
}

function formatMessageRole(role) {
  const normalized = String(role || "").trim().toLowerCase();
  if (!normalized) return "USER";
  return normalized.toUpperCase();
}

function truncateStructuredText(value, maxLength) {
  const text = String(value || "").trim();
  if (text.length <= maxLength) {
    return text;
  }
  return `${text.slice(0, maxLength)}...`;
}

function renderDiffPanes(before, after, beforeTokens, afterTokens) {
  const beforeLines = splitLines(before);
  const afterLines = splitLines(after);
  const ops = diffLines(beforeLines, afterLines);

  const beforeRows = [];
  const afterRows = [];
  let bNum = 0;
  let aNum = 0;
  let adds = 0;
  let dels = 0;

  for (const op of ops) {
    if (op.type === "eq") {
      bNum++; aNum++;
      beforeRows.push(diffRow("eq", bNum, op.text));
      afterRows.push(diffRow("eq", aNum, op.text));
    } else if (op.type === "del") {
      bNum++; dels++;
      beforeRows.push(diffRow("del", bNum, op.text));
    } else if (op.type === "add") {
      aNum++; adds++;
      afterRows.push(diffRow("add", aNum, op.text));
    }
  }

  const beforeStats = `Tokens ${beforeTokens.toLocaleString()} · 行数 ${beforeLines.length} · 删除 ${dels}`;
  const afterStats = `Tokens ${afterTokens.toLocaleString()} · 行数 ${afterLines.length} · 新增 ${adds}`;

  return `
    <article class="context-card">
      <div class="context-head">
        <div>
          <h3>清洗前上下文</h3>
          <p class="metric-meta">${escapeHtml(beforeStats)}</p>
        </div>
        <span class="diff-legend"><span class="diff-legend-item diff-legend-del">removed</span></span>
      </div>
      <div class="context-body as-diff">${beforeRows.join("") || emptyDiff()}</div>
    </article>
    <article class="context-card">
      <div class="context-head">
        <div>
          <h3>清洗后上下文</h3>
          <p class="metric-meta">${escapeHtml(afterStats)}</p>
        </div>
        <span class="diff-legend"><span class="diff-legend-item diff-legend-add">added</span></span>
      </div>
      <div class="context-body as-diff">${afterRows.join("") || emptyDiff()}</div>
    </article>
  `;
}

function diffRow(kind, num, text) {
  const marker = kind === "add" ? "+" : kind === "del" ? "−" : " ";
  return `<div class="diff-line diff-${kind}"><span class="diff-gutter">${num}</span><span class="diff-content">${marker} ${escapeHtml(text)}</span></div>`;
}

function emptyDiff() {
  return `<div class="diff-line diff-eq"><span class="diff-gutter"></span><span class="diff-content" style="color:var(--muted);padding:12px 0 12px 4px;">暂无内容</span></div>`;
}

function splitLines(text) {
  if (!text) return [];
  return String(text).split(/\r?\n/);
}

// LCS-based line diff. Returns ops [{type:"eq"|"add"|"del", text}].
// Guard against extreme sizes with a simple fallback.
function diffLines(a, b) {
  const MAX = 1500;
  if (a.length > MAX || b.length > MAX) {
    return fallbackDiff(a, b);
  }
  const m = a.length, n = b.length;
  const dp = new Array(m + 1);
  for (let i = 0; i <= m; i++) dp[i] = new Int32Array(n + 1);
  for (let i = m - 1; i >= 0; i--) {
    for (let j = n - 1; j >= 0; j--) {
      if (a[i] === b[j]) dp[i][j] = dp[i + 1][j + 1] + 1;
      else dp[i][j] = Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const ops = [];
  let i = 0, j = 0;
  while (i < m && j < n) {
    if (a[i] === b[j]) { ops.push({ type: "eq", text: a[i] }); i++; j++; }
    else if (dp[i + 1][j] >= dp[i][j + 1]) { ops.push({ type: "del", text: a[i] }); i++; }
    else { ops.push({ type: "add", text: b[j] }); j++; }
  }
  while (i < m) { ops.push({ type: "del", text: a[i++] }); }
  while (j < n) { ops.push({ type: "add", text: b[j++] }); }
  return ops;
}

function fallbackDiff(a, b) {
  const setB = new Set(b);
  const setA = new Set(a);
  const ops = [];
  for (const line of a) {
    ops.push({ type: setB.has(line) ? "eq" : "del", text: line });
  }
  for (const line of b) {
    if (!setA.has(line)) ops.push({ type: "add", text: line });
  }
  return ops;
}

function renderStepAudits(steps) {
  if (!steps.length) {
    return `<div class="empty-state-card">当前评估没有记录到步骤审计。</div>`;
  }

  return steps.map((step) => {
    const delta = Number(step.delta_tokens || 0);
    const details = Object.entries(step.details || {});
    const summaryRows = [
      ["减少 Tokens", formatCompactDelta(-delta)],
      ["耗时", formatDuration(step.duration_ms)],
      ["Lossy", step.capabilities?.lossy ? "是" : "否"],
      ["保留引用", step.capabilities?.preserve_citation ? "是" : "否"],
    ];
    const semanticRows = [
      ["移除", summarizeSemanticList(step.semantic?.removed)],
      ["保留", summarizeSemanticList(step.semantic?.retained)],
      ["原因", summarizeSemanticList(step.semantic?.reasons)],
    ].filter(([, value]) => value);

    return `
      <article class="step-card">
        <details class="step-details">
          <summary class="step-summary">
            <div class="step-summary-main">
              <strong>${escapeHtml(step.name || "未命名步骤")}</strong>
              <span class="metric-meta">Before ${escapeHtml(String(step.before_tokens || 0))} · After ${escapeHtml(String(step.after_tokens || 0))} · 阈值 ${escapeHtml(String(step.capabilities?.min_trigger_tokens || 0))}</span>
            </div>
            <span class="delta-pill ${delta < 0 ? "delta-good" : delta > 0 ? "delta-bad" : "delta-neutral"}">${escapeHtml(formatDelta(delta))}</span>
          </summary>
          <div class="step-detail-body">
            <div class="trace-meta-grid">
              ${summaryRows.map(([label, value]) => renderMiniMetric(label, value)).join("")}
            </div>
            <div class="metadata-kv">
              ${semanticRows.map(([key, value]) => `
                <div class="kv-row">
                  <span>${escapeHtml(key)}</span>
                  <strong>${escapeHtml(value)}</strong>
                </div>
              `).join("")}
              ${details.map(([key, value]) => `
                <div class="kv-row">
                  <span>${escapeHtml(key)}</span>
                  <strong>${escapeHtml(String(value))}</strong>
                </div>
              `).join("")}
              <div class="kv-row">
                <span>保留源码 / 代码块 / 错误栈</span>
                <strong>${escapeHtml(renderSemanticFlags(step.semantic))}</strong>
              </div>
              <div class="kv-row">
                <span>丢失引用数</span>
                <strong>${escapeHtml(String(step.semantic?.dropped_citations || 0))}</strong>
              </div>
            </div>
          </div>
        </details>
      </article>
    `;
  }).join("");
}

function renderSemanticBlock(label, values) {
  const items = Array.isArray(values) ? values.filter(Boolean) : [];
  if (!items.length) {
    return `
      <div class="semantic-block">
        <span>${escapeHtml(label)}</span>
        <div class="chip-row muted">无</div>
      </div>
    `;
  }
  return `
    <div class="semantic-block">
      <span>${escapeHtml(label)}</span>
      <div class="chip-row">
        ${items.map((item) => `<span class="chip">${escapeHtml(item)}</span>`).join("")}
      </div>
    </div>
  `;
}

function renderExtraPanels(evaluation, detail, reductionPercent) {
  const pagedChunks = evaluation.paged_chunks || [];
  return `
    <article class="extra-card">
      <h3>分页结果</h3>
      ${pagedChunks.length ? pagedChunks.map((item) => `
        <div class="paged-card">
          <div class="metric-meta">Chunk ${escapeHtml(item.chunk_id || "-")} · 页面 ${escapeHtml(String((item.page_keys || []).length))}</div>
          <div class="chip-row">
            ${(item.page_keys || []).map((key) => `<span class="chip">${escapeHtml(key)}</span>`).join("")}
          </div>
        </div>
      `).join("") : `<div class="metric-meta">这次清洗没有触发分页。</div>`}
    </article>
    <article class="extra-card">
      <h3>辅助指标</h3>
      <div class="metadata-kv">
        <div class="kv-row">
          <span>压缩比例</span>
          <strong>${escapeHtml(formatPercentNumber(reductionPercent))}</strong>
        </div>
        <div class="kv-row">
          <span>Budget Met</span>
          <strong>${escapeHtml(evaluation.budget_met ? "是" : "否")}</strong>
        </div>
        <div class="kv-row">
          <span>Root Span</span>
          <strong>${escapeHtml(detail?.root_span_name || "-")}</strong>
        </div>
        <div class="kv-row">
          <span>服务</span>
          <strong>${escapeHtml(detail?.root_service_name || "-")}</strong>
        </div>
        <div class="kv-row">
          <span>模型</span>
          <strong>${escapeHtml(evaluation.model_name || "-")}</strong>
        </div>
        <div class="kv-row">
          <span>策略</span>
          <strong>${escapeHtml(evaluation.policy || "-")}</strong>
        </div>
        <div class="kv-row">
          <span>创建时间</span>
          <strong>${escapeHtml(formatTime(evaluation.created_at))}</strong>
        </div>
      </div>
    </article>
  `;
}

function getTraceTitle(item) {
  if (item.root_trace_name) {
    return item.root_trace_name;
  }
  for (const spanSet of item.span_sets || []) {
    for (const span of spanSet.spans || []) {
      if (span.name) {
        return span.name;
      }
    }
  }
  return "未命名 trace";
}

function getTraceServiceName(item) {
  if (item.root_service_name) {
    return item.root_service_name;
  }
  for (const spanSet of item.span_sets || []) {
    for (const span of spanSet.spans || []) {
      const serviceAttr = (span.attributes || []).find((attribute) => attribute.key === "service.name");
      if (serviceAttr?.value) {
        return serviceAttr.value;
      }
    }
  }
  return "未识别服务";
}

function getTracePreview(item) {
  const names = [];
  for (const spanSet of item.span_sets || []) {
    for (const span of spanSet.spans || []) {
      if (span.name && !names.includes(span.name)) {
        names.push(span.name);
      }
      if (names.length >= 3) {
        return `匹配到的关键 Span：${names.join(" · ")}`;
      }
    }
  }
  return "搜索结果没有返回更多可读 Span 名称，可直接打开评估查看。";
}

function getTraceLabelFromEvaluation(evaluation) {
  if (!evaluation?.steps?.length) {
    return "上下文清洗结果";
  }
  return `${evaluation.steps.length} 个清洗步骤`;
}

function compactId(value, keep = 16) {
  const text = String(value || "").trim();
  if (!text) {
    return "-";
  }
  if (text.length <= keep) {
    return text;
  }
  const head = Math.max(6, Math.floor(keep / 2));
  const tail = Math.max(4, keep - head);
  return `${text.slice(0, head)}...${text.slice(-tail)}`;
}

function countLines(content) {
  if (!content) {
    return 0;
  }
  return String(content).split(/\r?\n/).length;
}

function metadataInt(metadata, key) {
  const value = Number(metadata?.[key] || 0);
  return Number.isFinite(value) ? value : 0;
}

function summarizeSemanticList(values) {
  const items = Array.isArray(values) ? values.filter(Boolean) : [];
  if (!items.length) {
    return "";
  }
  return items.slice(0, 3).join(" · ");
}

function renderSemanticFlags(semantic) {
  const flags = [
    semantic?.source_preserved ? "是" : "否",
    semantic?.code_fence_preserved ? "是" : "否",
    semantic?.error_stack_preserved ? "是" : "否",
  ];
  return flags.join(" / ");
}

function formatBoolText(value) {
  if (value === true || value === "true") {
    return "是";
  }
  if (value === false || value === "false") {
    return "否";
  }
  return String(value || "-");
}

function formatCompactDelta(value) {
  const number = Number(value || 0);
  if (number > 0) {
    return `+${number}`;
  }
  if (number < 0) {
    return `${number}`;
  }
  return "0";
}

function formatDelta(value) {
  const number = Number(value || 0);
  if (number > 0) {
    return `+${number}`;
  }
  return `${number}`;
}

function formatPercentNumber(value) {
  const number = Number(value || 0);
  if (!Number.isFinite(number)) {
    return "0%";
  }
  return `${number.toFixed(number >= 10 ? 0 : 1)}%`;
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll("\"", "&quot;")
    .replaceAll("'", "&#39;");
}

function formatTime(value) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function formatDuration(value) {
  const duration = Number(value || 0);
  if (!Number.isFinite(duration) || duration <= 0) {
    return "0 ms";
  }
  if (duration < 1000) {
    return `${duration} ms`;
  }
  if (duration < 60000) {
    return `${(duration / 1000).toFixed(2)} s`;
  }
  return `${(duration / 60000).toFixed(2)} min`;
}

const autoRefreshToggle = document.getElementById("auto-refresh-toggle");
const autoRefreshInterval = document.getElementById("auto-refresh-interval");

let currentTraceId = null;
let currentSearchQuery = defaultTraceQuery;
let refreshTimer = null;
let refreshInFlight = false;

async function runRefresh({ silent = false } = {}) {
  if (refreshInFlight) return;
  refreshInFlight = true;
  if (!silent) document.body.classList.add("refreshing");
  try {
    const tasks = [loadSnapshot()];
    tasks.push(searchTraces(currentSearchQuery).then(renderTraceResults).catch((e) => {
      traceResults.innerHTML = `<div class="empty-state-card">Trace 查询失败：${escapeHtml(e.message)}</div>`;
    }));
    if (currentTraceId) {
      tasks.push(showEvaluation(currentTraceId, { silent }).catch(() => {}));
    }
    await Promise.all(tasks);
  } catch (error) {
    generatedAt.textContent = `刷新失败：${error.message}`;
  } finally {
    refreshInFlight = false;
    if (!silent) document.body.classList.remove("refreshing");
  }
}

function stopAutoRefresh() {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
}

function startAutoRefresh() {
  stopAutoRefresh();
  if (!autoRefreshToggle.checked) return;
  const ms = Number(autoRefreshInterval.value) || 10000;
  refreshTimer = setInterval(() => {
    if (document.hidden) return;
    runRefresh({ silent: true });
  }, ms);
}

refreshButton.addEventListener("click", () => runRefresh());

autoRefreshToggle.addEventListener("change", startAutoRefresh);
autoRefreshInterval.addEventListener("change", startAutoRefresh);

document.addEventListener("visibilitychange", () => {
  if (!document.hidden && autoRefreshToggle.checked) runRefresh({ silent: true });
});

traceSearchForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  traceSearchButton.disabled = true;
  currentSearchQuery = traceQueryInput.value.trim() || defaultTraceQuery;
  traceResults.innerHTML = `<div class="empty-state-card">正在执行 TraceQL 查询...</div>`;
  try {
    const payload = await searchTraces(currentSearchQuery);
    renderTraceResults(payload);
    if (payload.traces?.[0]?.trace_id) {
      currentTraceId = payload.traces[0].trace_id;
      traceIdInput.value = currentTraceId;
      await showEvaluation(currentTraceId);
    }
  } catch (error) {
    traceResults.innerHTML = `<div class="empty-state-card">Trace 查询失败：${escapeHtml(error.message)}</div>`;
  } finally {
    traceSearchButton.disabled = false;
  }
});

traceIdForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const traceId = traceIdInput.value.trim();
  if (!traceId) {
    evaluationStatus.innerHTML = `<div class="empty-state-card">请输入完整 trace ID。</div>`;
    return;
  }
  currentTraceId = traceId;
  await showEvaluation(traceId);
});

// Track trace selection is done inside showEvaluation (sets currentTraceId).

Promise.all([
  loadSnapshot(),
  searchTraces(defaultTraceQuery).then(async (payload) => {
    renderTraceResults(payload);
    if (payload.traces?.[0]?.trace_id) {
      currentTraceId = payload.traces[0].trace_id;
      traceIdInput.value = currentTraceId;
      await showEvaluation(currentTraceId);
    }
  }),
]).then(() => {
  startAutoRefresh();
}).catch((error) => {
  generatedAt.textContent = `初始化失败：${error.message}`;
  startAutoRefresh();
});
