package models

import (
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	configValue atomic.Value
)

type ConfigManager struct {
	config *Config
}

func InitConfigManager(config *Config) *ConfigManager {
	c := &ConfigManager{
		config: config,
	}
	return c
}

func (c *ConfigManager) Update() error {
	configValue.Store(c.config.marshal())
	return nil
}

func GetConfig() []byte {
	av := configValue.Load()
	config, ok := av.([]byte)
	if !ok {
		logger.Fatalf("BUG: unexpected type stored in config value")
	}
	return config
}
