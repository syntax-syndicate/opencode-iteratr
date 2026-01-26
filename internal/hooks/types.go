package hooks

// Config is the top-level configuration for hooks loaded from .iteratr.hooks.yml.
type Config struct {
	Version int         `yaml:"version"`
	Hooks   HooksConfig `yaml:"hooks"`
}

// HooksConfig contains all hook configurations.
type HooksConfig struct {
	PreIteration *HookConfig `yaml:"pre_iteration"`
}

// HookConfig defines a single hook's configuration.
type HookConfig struct {
	Command string `yaml:"command"`
	Timeout int    `yaml:"timeout"` // seconds, default 30
}

// DefaultTimeout is the default timeout for hook execution in seconds.
const DefaultTimeout = 30
