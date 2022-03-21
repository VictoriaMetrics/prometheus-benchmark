package models

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"gopkg.in/yaml.v2"
)

const (
	defaultJobName                   = "node_exporter"
	defaultTargetAddr                = "vm-benchmark-exporter.default.svc:9102"
	defaultTargetCount               = 1000
	defaultTargetsToUpdatePercentage = 10
)

type ConfigOptions func(config *Config)

func NewConfig(options ...ConfigOptions) *Config {
	c := &Config{}
	for _, fn := range options {
		fn(c)
	}
	c.prepareStaticConfig()
	c.update()
	return c
}

// Config represents essential parts from Prometheus config defined at https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type Config struct {
	Global                    GlobalConfig   `yaml:"global"`
	ScrapeConfigs             []ScrapeConfig `yaml:"scrape_configs,omitempty"`
	targetCount               int
	targetAddr                string
	targetsToUpdatePercentage int
	jobName                   string
	stConfigs                 []*StaticConfig
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
	JobName        string          `yaml:"job_name"`
	ScrapeInterval time.Duration   `yaml:"scrape_interval,omitempty"`
	ScrapeTimeout  time.Duration   `yaml:"scrape_timeout,omitempty"`
	StaticConfigs  []*StaticConfig `yaml:"static_configs,omitempty"`
}

// StaticConfig represents essential parts for `static_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config
type StaticConfig struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels,omitempty"`
}

func WithScrapeInterval(scrapeInterval time.Duration) ConfigOptions {
	return func(config *Config) {
		config.Global = GlobalConfig{ScrapeInterval: scrapeInterval}
	}
}

func WithTargetCount(targetCount int) ConfigOptions {
	return func(config *Config) {
		if targetCount == 0 {
			targetCount = defaultTargetCount
			logger.Infof("targetCount must be greater than 0. Used default value")
		}
		config.targetCount = targetCount
	}
}

func WithTargetsToUpdatePercentage(targetsToUpdatePercentage int) ConfigOptions {
	return func(config *Config) {
		if targetsToUpdatePercentage > 100 || targetsToUpdatePercentage < 0 {
			targetsToUpdatePercentage = defaultTargetsToUpdatePercentage
			logger.Infof("targetsToUpdatePercentage can't be lower than 0 or more than 100. Used default value")
		}
		config.targetsToUpdatePercentage = targetsToUpdatePercentage
	}
}

func WithTargetAddr(targetAddr string) ConfigOptions {
	return func(config *Config) {
		host, port, err := net.SplitHostPort(targetAddr)
		if err != nil {
			targetAddr = defaultTargetAddr
			logger.Infof("targetName not correct. Used default value")
		} else if host == "" || port == "" {
			targetAddr = defaultTargetAddr
			logger.Infof("targetName can't be empty or must include correct host:port configuration. Used default value")
		}
		config.targetAddr = targetAddr
	}
}

func WithJobName(jobName string) ConfigOptions {
	return func(config *Config) {
		if jobName == "" {
			jobName = defaultJobName
		}
		config.jobName = jobName
	}
}

func (cfg *Config) prepareStaticConfig() {
	for i := 0; i < cfg.targetCount; i++ {
		cfg.stConfigs = append(cfg.stConfigs, &StaticConfig{
			Targets: []string{cfg.targetAddr},
			Labels: map[string]string{
				"host_number": fmt.Sprintf("cfg_%d", i),
				"instance":    strconv.FormatInt(time.Now().UnixNano(), 10),
			},
		})
	}
}

func (cfg *Config) update() {
	num := float64(cfg.targetCount) * (float64(cfg.targetsToUpdatePercentage) / 100)
	for i := 0; i < int(num); i++ {
		j := rand.Intn(len(cfg.stConfigs) - 1)
		cfg.stConfigs[j].Labels["instance"] = strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	var scrapeConfigs []ScrapeConfig
	scrapeConfigs = append(scrapeConfigs, ScrapeConfig{
		JobName:       cfg.jobName,
		StaticConfigs: cfg.stConfigs,
	})

	cfg.ScrapeConfigs = scrapeConfigs
}

func (cfg *Config) marshal() []byte {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		logger.Panicf("BUG: cannot marshal Config: %s", err)
	}
	return data
}
