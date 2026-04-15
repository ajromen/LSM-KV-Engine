package config

var settingsInstance *Config

func createSettings(config *Config) {
	settingsInstance = config
}

func GetSettings() *Config {
	if settingsInstance == nil {
		createSettings(NewDefaultConfig())
	}
	return settingsInstance
}

// should only be used for test to change global settings
func TESTSetSettings(config *Config) {
	settingsInstance = config
}
