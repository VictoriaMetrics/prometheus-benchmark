package models

import (
	"fmt"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"gopkg.in/yaml.v2"
)

type ConfigOptions func(config *Config)

func NewConfig(options ...ConfigOptions) *Config {
	c := &Config{}
	for _, fn := range options {
		fn(c)
	}
	return c
}

// Config represents essential parts from Prometheus config defined at https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type Config struct {
	Global                   GlobalConfig   `yaml:"global"`
	ScrapeConfigs            []ScrapeConfig `yaml:"scrape_configs,omitempty"`
	targetCount              int
	targetName               string
	percentOfUpdatingTargets int
	stConfigs                []StaticConfig
}

// GlobalConfig represents essential parts for `global` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type GlobalConfig struct {
	ScrapeInterval time.Duration `yaml:"scrape_interval,omitempty"`
}

// ScrapeConfig represents essential parts for `scrape_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
type ScrapeConfig struct {
	JobName        string         `yaml:"job_name"`
	ScrapeInterval time.Duration  `yaml:"scrape_interval,omitempty"`
	ScrapeTimeout  time.Duration  `yaml:"scrape_timeout,omitempty"`
	StaticConfigs  []StaticConfig `yaml:"static_configs,omitempty"`
}

// StaticConfig represents essential parts for `static_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config
type StaticConfig struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels,omitempty"`
}

func WithGlobalConfig(scrapeInterval time.Duration) ConfigOptions {
	return func(config *Config) {
		config.Global = GlobalConfig{ScrapeInterval: scrapeInterval}
	}
}

func WithScrapeConfig(targetCount, percentOfUpdatingTargets int, targetName string) ConfigOptions {
	return func(config *Config) {
		config.targetCount = targetCount
		config.targetName = targetName
		config.percentOfUpdatingTargets = percentOfUpdatingTargets
		config.update()
	}
}

func (cfg *Config) update() {
	var scrapeConfigs []ScrapeConfig
	configs := make(map[string][]StaticConfig)
	configs["vmalert"] = []StaticConfig{
		{
			Targets: []string{"localhost:8429"},
			Labels:  nil,
		},
	}
	if len(cfg.stConfigs) == cfg.targetCount {
		num := cfg.targetCount / cfg.percentOfUpdatingTargets
		if num < 0 {
			num = 1
		}
		for i := 0; i < num; i++ {
			cfg.stConfigs[i] = StaticConfig{
				Targets: []string{cfg.targetName},
				Labels: map[string]string{
					"host_number": cfg.stConfigs[i].Labels["host_number"],
					"instance":    strconv.FormatInt(time.Now().UnixNano(), 10),
				},
			}
		}
	} else {
		for i := 0; i < cfg.targetCount; i++ {
			cfg.stConfigs = append(cfg.stConfigs, StaticConfig{
				Targets: []string{cfg.targetName},
				Labels: map[string]string{
					"host_number": fmt.Sprintf("cfg_%d", i),
					"instance":    strconv.FormatInt(time.Now().UnixNano(), 10),
				},
			})
		}
	}
	configs["node_exporter"] = cfg.stConfigs
	for jobName, scrapeCfgs := range configs {
		scrapeConfigs = append(scrapeConfigs, ScrapeConfig{
			JobName:       jobName,
			StaticConfigs: scrapeCfgs,
		})
	}
	cfg.ScrapeConfigs = scrapeConfigs
}

func (cfg *Config) marshal() []byte {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		logger.Panicf("BUG: cannot marshal Config: %s", err)
	}
	return data
}
