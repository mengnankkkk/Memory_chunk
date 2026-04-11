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
	Redis struct {
		Addr          string        `yaml:"addr"`
		Username      string        `yaml:"username"`
		Password      string        `yaml:"password"`
		DB            int           `yaml:"db"`
		KeyPrefix     string        `yaml:"key_prefix"`
		PageTTL       time.Duration `yaml:"page_ttl"`
		SummaryStream string        `yaml:"summary_stream"`
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
	if strings.TrimSpace(cfg.Redis.SummaryStream) == "" {
		cfg.Redis.SummaryStream = "context-refiner:summary-jobs"
	}
	if strings.TrimSpace(cfg.Tokenizer.FallbackEncoding) == "" {
		cfg.Tokenizer.FallbackEncoding = "cl100k_base"
	}
	if cfg.Pipeline.PagingTokenThreshold <= 0 {
		cfg.Pipeline.PagingTokenThreshold = 320
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
	default:
		return nil
	}
}
