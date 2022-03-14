package models

import "sync/atomic"

var configTmpl atomic.Value

type (
	Updater interface {
		Update() error
	}
)

func Update() error {
	return nil
}

func GetConfig() string {
	return ""
}

type ConfigManager struct {
	config string
}

func InitConfigManager() *ConfigManager {
	return *ConfigManager{}
}
