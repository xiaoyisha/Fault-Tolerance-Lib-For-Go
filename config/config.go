package config

import (
	"sync"
	"time"
)

var (
	// DefaultMaxConcurrent is how many commands of the same type can run at the same time in an executor pool
	DefaultMaxConcurrent = 10
)

type Config struct {
	MaxConcurrentRequests int
	SleepWindow           time.Duration
}

var circuitConfig map[string]*Config
var configMutex *sync.RWMutex

func init() {
	circuitConfig = make(map[string]*Config)
}

// CommandConfig is used to tune circuit settings at runtime
type CommandConfig struct {
	MaxConcurrentRequests int
}

// Configure applies settings for a set of circuits
func Configure(cmds map[string]CommandConfig) {
	for k, v := range cmds {
		ConfigureCommand(k, v)
	}
}

// ConfigureCommand applies settings for a circuit
func ConfigureCommand(name string, config CommandConfig) {
	configMutex.Lock()
	defer configMutex.Unlock()

	max := DefaultMaxConcurrent
	if config.MaxConcurrentRequests != 0 {
		max = config.MaxConcurrentRequests
	}

	circuitConfig[name] = &Config{
		MaxConcurrentRequests: max,
	}
}

// GetCircuitConfig get the config of the circuit by name
func GetCircuitConfig(name string) *Config {
	configMutex.RLock()
	s, exists := circuitConfig[name]
	configMutex.RUnlock()

	if !exists {
		ConfigureCommand(name, CommandConfig{}) // use the default config
		s = GetCircuitConfig(name)
	}

	return s
}
