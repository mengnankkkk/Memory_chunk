package config

import (
	"fmt"
	"os"

	"context-refiner/internal/domain/core"

	"gopkg.in/yaml.v3"
)

type PolicyFile struct {
	Policies []Policy `yaml:"policies"`
}

type Policy struct {
	Name                 string   `yaml:"name"`
	BudgetRatio          float64  `yaml:"budget_ratio"`
	Steps                []string `yaml:"steps"`
	AutoCompactThreshold int      `yaml:"auto_compact_threshold"`
	SnipParams           struct {
		KeepHeadLines int `yaml:"keep_head_lines"`
		KeepTailLines int `yaml:"keep_tail_lines"`
	} `yaml:"snip_params"`
}

func LoadPolicies(path string) (map[string]core.RuntimePolicy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy file failed: %w", err)
	}
	var file PolicyFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse policy file failed: %w", err)
	}
	items := make(map[string]core.RuntimePolicy, len(file.Policies))
	for _, policy := range file.Policies {
		items[policy.Name] = core.RuntimePolicy{
			Name:        policy.Name,
			BudgetRatio: policy.BudgetRatio,
			Steps:       append([]string(nil), policy.Steps...),
			Snip: core.SnipConfig{
				KeepHeadLines: policy.SnipParams.KeepHeadLines,
				KeepTailLines: policy.SnipParams.KeepTailLines,
			},
			AutoCompactThreshold: policy.AutoCompactThreshold,
		}
	}
	return items, nil
}
