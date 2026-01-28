// Package config provides centralized configuration management using Viper.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration values for iteratr.
type Config struct {
	Model      string `mapstructure:"model" yaml:"model"`
	AutoCommit bool   `mapstructure:"auto_commit" yaml:"auto_commit"`
	DataDir    string `mapstructure:"data_dir" yaml:"data_dir"`
	LogLevel   string `mapstructure:"log_level" yaml:"log_level"`
	LogFile    string `mapstructure:"log_file" yaml:"log_file"`
	Iterations int    `mapstructure:"iterations" yaml:"iterations"`
	Headless   bool   `mapstructure:"headless" yaml:"headless"`
	Template   string `mapstructure:"template" yaml:"template"`
}

// Load loads configuration with full precedence:
// CLI flags > ENV vars > project config > XDG global config > defaults
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName("iteratr")

	// Set defaults (model has no default - it's required)
	v.SetDefault("auto_commit", true)
	v.SetDefault("data_dir", ".iteratr")
	v.SetDefault("log_level", "info")
	v.SetDefault("log_file", "")
	v.SetDefault("iterations", 0)
	v.SetDefault("headless", false)
	v.SetDefault("template", "")

	// Setup ENV binding with ITERATR_ prefix
	v.SetEnvPrefix("ITERATR")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicit ENV bindings for better bool/int parsing
	// Note: BindEnv errors are very rare (only invalid key names), but we check them anyway
	if err := v.BindEnv("model", "ITERATR_MODEL"); err != nil {
		return nil, fmt.Errorf("binding model env: %w", err)
	}
	if err := v.BindEnv("auto_commit", "ITERATR_AUTO_COMMIT"); err != nil {
		return nil, fmt.Errorf("binding auto_commit env: %w", err)
	}
	if err := v.BindEnv("data_dir", "ITERATR_DATA_DIR"); err != nil {
		return nil, fmt.Errorf("binding data_dir env: %w", err)
	}
	if err := v.BindEnv("log_level", "ITERATR_LOG_LEVEL"); err != nil {
		return nil, fmt.Errorf("binding log_level env: %w", err)
	}
	if err := v.BindEnv("log_file", "ITERATR_LOG_FILE"); err != nil {
		return nil, fmt.Errorf("binding log_file env: %w", err)
	}
	if err := v.BindEnv("iterations", "ITERATR_ITERATIONS"); err != nil {
		return nil, fmt.Errorf("binding iterations env: %w", err)
	}
	if err := v.BindEnv("headless", "ITERATR_HEADLESS"); err != nil {
		return nil, fmt.Errorf("binding headless env: %w", err)
	}
	if err := v.BindEnv("template", "ITERATR_TEMPLATE"); err != nil {
		return nil, fmt.Errorf("binding template env: %w", err)
	}

	// Load global config first (if exists)
	globalPath := GlobalPath()
	if fileExists(globalPath) {
		v.SetConfigFile(globalPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading global config: %w", err)
		}
	}

	// Merge project config on top (if exists)
	projectPath := ProjectPath()
	if fileExists(projectPath) {
		// Need to set config file explicitly for merge
		v.SetConfigFile(projectPath)
		if err := v.MergeInConfig(); err != nil {
			return nil, fmt.Errorf("merging project config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// Exists returns true if any config file exists (global or project).
func Exists() bool {
	return fileExists(GlobalPath()) || fileExists(ProjectPath())
}

// GlobalPath returns the XDG global config path.
// Returns ~/.config/iteratr/iteratr.yml or $XDG_CONFIG_HOME/iteratr/iteratr.yml.
func GlobalPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "iteratr", "iteratr.yml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "iteratr", "iteratr.yml")
}

// ProjectPath returns the project-local config path.
// Returns ./iteratr.yml in the current working directory.
func ProjectPath() string {
	return "iteratr.yml"
}

// WriteGlobal writes the config to the XDG global location.
func WriteGlobal(cfg *Config) error {
	path := GlobalPath()

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// WriteProject writes the config to the project-local location.
func WriteProject(cfg *Config) error {
	path := ProjectPath()

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
