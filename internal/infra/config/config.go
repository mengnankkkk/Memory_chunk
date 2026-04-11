package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	GRPC struct {
		ListenAddr string `yaml:"listen_addr"`
	} `yaml:"grpc"`
	Observability struct {
		MetricsEnabled    bool    `yaml:"metrics_enabled"`
		MetricsListenAddr string  `yaml:"metrics_listen_addr"`
		MetricsPath       string  `yaml:"metrics_path"`
		TracingEnabled    bool    `yaml:"tracing_enabled"`
		ServiceName       string  `yaml:"service_name"`
		TracingEndpoint   string  `yaml:"tracing_endpoint"`
		TracingInsecure   bool    `yaml:"tracing_insecure"`
		TracingSampleRate float64 `yaml:"tracing_sample_rate"`
	} `yaml:"observability"`
	Redis struct {
		Addr           string        `yaml:"addr"`
		Username       string        `yaml:"username"`
		Password       string        `yaml:"password"`
		DB             int           `yaml:"db"`
		KeyPrefix      string        `yaml:"key_prefix"`
		PageTTL        time.Duration `yaml:"page_ttl"`
		PrefixCacheTTL time.Duration `yaml:"prefix_cache_ttl"`
		SummaryStream  string        `yaml:"summary_stream"`
	} `yaml:"redis"`
	Tokenizer struct {
		Model            string `yaml:"model"`
		FallbackEncoding string `yaml:"fallback_encoding"`
	} `yaml:"tokenizer"`
	Pipeline struct {
		PolicyFile           string `yaml:"policy_file"`
		DefaultPolicy        string `yaml:"default_policy"`
		PagingTokenThreshold int    `yaml:"paging_token_threshold"`
	} `yaml:"pipeline"`
	PrefixCache struct {
		MinStablePrefixTokens int           `yaml:"min_stable_prefix_tokens"`
		MinSegmentCount       int           `yaml:"min_segment_count"`
		DefaultTenant         string        `yaml:"default_tenant"`
		HotThreshold          int64         `yaml:"hot_threshold"`
		HotTTL                time.Duration `yaml:"hot_ttl"`
		Namespace             struct {
			IncludePolicy bool `yaml:"include_policy"`
			IncludeModel  bool `yaml:"include_model"`
			IncludeTenant bool `yaml:"include_tenant"`
		} `yaml:"namespace"`
		Prewarm []struct {
			Name         string `yaml:"name"`
			ModelID      string `yaml:"model_id"`
			Policy       string `yaml:"policy"`
			Tenant       string `yaml:"tenant"`
			SystemPrompt string `yaml:"system_prompt"`
			MemoryPrompt string `yaml:"memory_prompt"`
			RAGPrompt    string `yaml:"rag_prompt"`
		} `yaml:"prewarm"`
	} `yaml:"prefix_cache"`
	SummaryWorker struct {
		Enabled       bool          `yaml:"enabled"`
		ConsumerGroup string        `yaml:"consumer_group"`
		ConsumerName  string        `yaml:"consumer_name"`
		BatchSize     int64         `yaml:"batch_size"`
		BlockTimeout  time.Duration `yaml:"block_timeout"`
	} `yaml:"summary_worker"`
}

func LoadAppConfig(path string) (*AppConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read app config failed: %w", err)
	}
	var cfg AppConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse app config failed: %w", err)
	}
	if cfg.Redis.PageTTL <= 0 {
		cfg.Redis.PageTTL = 24 * time.Hour
	}
	if cfg.Redis.PrefixCacheTTL <= 0 {
		cfg.Redis.PrefixCacheTTL = 24 * time.Hour
	}
	if strings.TrimSpace(cfg.Redis.SummaryStream) == "" {
		cfg.Redis.SummaryStream = "context-refiner:summary-jobs"
	}
	if strings.TrimSpace(cfg.Tokenizer.FallbackEncoding) == "" {
		cfg.Tokenizer.FallbackEncoding = "cl100k_base"
	}
	if cfg.Pipeline.PagingTokenThreshold <= 0 {
		cfg.Pipeline.PagingTokenThreshold = 320
	}
	if cfg.PrefixCache.MinStablePrefixTokens <= 0 {
		cfg.PrefixCache.MinStablePrefixTokens = 32
	}
	if cfg.PrefixCache.MinSegmentCount <= 0 {
		cfg.PrefixCache.MinSegmentCount = 1
	}
	if strings.TrimSpace(cfg.PrefixCache.DefaultTenant) == "" {
		cfg.PrefixCache.DefaultTenant = "global"
	}
	if cfg.PrefixCache.HotThreshold <= 0 {
		cfg.PrefixCache.HotThreshold = 5
	}
	if cfg.PrefixCache.HotTTL <= 0 {
		cfg.PrefixCache.HotTTL = 72 * time.Hour
	}
	if !cfg.PrefixCache.Namespace.IncludePolicy && !cfg.PrefixCache.Namespace.IncludeModel && !cfg.PrefixCache.Namespace.IncludeTenant {
		cfg.PrefixCache.Namespace.IncludePolicy = true
		cfg.PrefixCache.Namespace.IncludeModel = true
		cfg.PrefixCache.Namespace.IncludeTenant = true
	}
	if strings.TrimSpace(cfg.Observability.MetricsPath) == "" {
		cfg.Observability.MetricsPath = "/metrics"
	}
	if strings.TrimSpace(cfg.Observability.ServiceName) == "" {
		cfg.Observability.ServiceName = "context-refiner"
	}
	if cfg.Observability.MetricsEnabled && strings.TrimSpace(cfg.Observability.MetricsListenAddr) == "" {
		cfg.Observability.MetricsListenAddr = ":9091"
	}
	if cfg.Observability.TracingSampleRate <= 0 || cfg.Observability.TracingSampleRate > 1 {
		cfg.Observability.TracingSampleRate = 1
	}
	if strings.TrimSpace(cfg.SummaryWorker.ConsumerGroup) == "" {
		cfg.SummaryWorker.ConsumerGroup = "context-refiner-summary"
	}
	if strings.TrimSpace(cfg.SummaryWorker.ConsumerName) == "" {
		cfg.SummaryWorker.ConsumerName = "worker-1"
	}
	if cfg.SummaryWorker.BatchSize <= 0 {
		cfg.SummaryWorker.BatchSize = 8
	}
	if cfg.SummaryWorker.BlockTimeout <= 0 {
		cfg.SummaryWorker.BlockTimeout = 2 * time.Second
	}
	return &cfg, nil
}

func (c *AppConfig) Validate() error {
	switch {
	case strings.TrimSpace(c.GRPC.ListenAddr) == "":
		return fmt.Errorf("grpc.listen_addr is required")
	case strings.TrimSpace(c.Redis.Addr) == "":
		return fmt.Errorf("redis.addr is required")
	case strings.TrimSpace(c.Pipeline.PolicyFile) == "":
		return fmt.Errorf("pipeline.policy_file is required")
	case strings.TrimSpace(c.Pipeline.DefaultPolicy) == "":
		return fmt.Errorf("pipeline.default_policy is required")
	case c.PrefixCache.MinStablePrefixTokens < 0:
		return fmt.Errorf("prefix_cache.min_stable_prefix_tokens must be >= 0")
	case c.PrefixCache.MinSegmentCount < 0:
		return fmt.Errorf("prefix_cache.min_segment_count must be >= 0")
	case c.PrefixCache.HotThreshold < 0:
		return fmt.Errorf("prefix_cache.hot_threshold must be >= 0")
	case c.Observability.MetricsEnabled && strings.TrimSpace(c.Observability.MetricsListenAddr) == "":
		return fmt.Errorf("observability.metrics_listen_addr is required when metrics_enabled=true")
	case c.Observability.TracingEnabled && strings.TrimSpace(c.Observability.TracingEndpoint) == "":
		return fmt.Errorf("observability.tracing_endpoint is required when tracing_enabled=true")
	case c.Observability.TracingSampleRate < 0 || c.Observability.TracingSampleRate > 1:
		return fmt.Errorf("observability.tracing_sample_rate must be between 0 and 1")
	default:
		return nil
	}
}
